package reconciler

import (
	"context"
	"fmt"
	"strings"

	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	"k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const appWorkloadCapabilityAll v1.Capability = "ALL"

func resolvePodSecurityContext() *v1.PodSecurityContext {
	return &v1.PodSecurityContext{
		SeccompProfile: resolveRuntimeDefaultSeccompProfile(),
	}
}

func resolveContainerSecurityContext() *v1.SecurityContext {
	return &v1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &v1.Capabilities{
			Drop: []v1.Capability{appWorkloadCapabilityAll},
		},
		SeccompProfile: resolveRuntimeDefaultSeccompProfile(),
	}
}

func resolveRuntimeDefaultSeccompProfile() *v1.SeccompProfile {
	return &v1.SeccompProfile{
		Type: v1.SeccompProfileTypeRuntimeDefault,
	}
}

func resolveInitContainers(app serverManifest.Application, wandb *v2.WeightsAndBiases, envVars []v1.EnvVar, volumeMounts []v1.VolumeMount) []v1.Container {
	initContainers := []v1.Container{}

	if app.InitContainers != nil {
		for _, initContainerSpec := range app.InitContainers {
			if initContainerSpec.Name == "migrate" {
				continue
			}
			initContainer := v1.Container{
				Name:            initContainerSpec.Name,
				Image:           initContainerSpec.Image.GetImage(wandb.Spec.Global.ImageRegistry),
				Env:             envVars,
				Args:            initContainerSpec.Args,
				Command:         initContainerSpec.Command,
				VolumeMounts:    volumeMounts,
				SecurityContext: resolveContainerSecurityContext(),
			}
			initContainers = append(initContainers, initContainer)
		}
	}
	return initContainers
}

func resolveContainers(app serverManifest.Application, wandb *v2.WeightsAndBiases, envVars []v1.EnvVar, volumeMounts []v1.VolumeMount) []v1.Container {
	// Build containers: support multi-container apps via app.Containers; fall back to a single
	// default container when no explicit containers are provided.
	containers := []v1.Container{}
	if len(app.Containers) > 0 {
		for _, container := range app.Containers {
			// Convert ports
			var containerPorts []v1.ContainerPort
			for _, p := range container.Ports {
				containerPorts = append(containerPorts, v1.ContainerPort{
					Name:          p.Name,
					ContainerPort: p.ContainerPort,
					Protocol:      p.Protocol,
				})
			}

			// Choose image/args/command with sensible fallbacks to app-level values
			img := app.Image.GetImage(wandb.Spec.Global.ImageRegistry)
			if container.Image.Repository != "" {
				img = container.Image.GetImage(wandb.Spec.Global.ImageRegistry)
			}
			args := app.Args
			if len(container.Args) > 0 {
				args = container.Args
			}
			cmd := app.Command
			if len(container.Command) > 0 {
				cmd = container.Command
			}

			c := v1.Container{
				Name:            container.Name,
				Image:           img,
				Env:             envVars,
				Args:            args,
				Command:         cmd,
				Ports:           containerPorts,
				VolumeMounts:    volumeMounts,
				SecurityContext: resolveContainerSecurityContext(),
			}

			if resources := ResolveResources(app, wandb, container.Resources); resources != nil {
				c.Resources = *resources
			}

			if container.StartupProbe != nil {
				c.StartupProbe = container.StartupProbe.DeepCopy()
			}
			if container.LivenessProbe != nil {
				c.LivenessProbe = container.LivenessProbe.DeepCopy()
			}
			if container.ReadinessProbe != nil {
				c.ReadinessProbe = container.ReadinessProbe.DeepCopy()
			}

			containers = append(containers, c)
		}
	} else {
		// Backward-compatible single-container behavior
		c := v1.Container{
			Name:            app.Name,
			Image:           app.Image.GetImage(wandb.Spec.Global.ImageRegistry),
			Env:             envVars,
			Args:            app.Args,
			Command:         app.Command,
			VolumeMounts:    volumeMounts,
			SecurityContext: resolveContainerSecurityContext(),
		}

		if resources := ResolveResources(app, wandb, nil); resources != nil {
			c.Resources = *resources
		}
		containers = append(containers, c)
	}
	return containers
}

func resolveEnvvars(ctx context.Context, client ctrlClient.Client, wandb *v2.WeightsAndBiases, manifest serverManifest.Manifest, commonEnvs []string, envs []serverManifest.EnvVar) ([]v1.EnvVar, error) {
	logger := logx.GetSlog(ctx)
	var combinedEnvs []serverManifest.EnvVar
	for _, commonVars := range commonEnvs {
		if envvars, ok := manifest.CommonEnvvars[commonVars]; ok {
			combinedEnvs = append(combinedEnvs, envvars...)
		}
	}

	for _, env := range envs {
		for i, combinedEnv := range combinedEnvs {
			if combinedEnv.Name == env.Name {
				combinedEnvs = append(combinedEnvs[:i], combinedEnvs[i+1:]...)
				break
			}
		}
	}

	combinedEnvs = append(combinedEnvs, envs...)

	var envVars []v1.EnvVar
	for _, env := range combinedEnvs {
		// If a literal value is provided, it's a simple case.
		if env.Value != "" {
			envVars = append(envVars, v1.EnvVar{Name: env.Name, Value: env.Value})
			continue
		}
		if env.ValueFrom != nil {
			envVars = append(envVars, v1.EnvVar{Name: env.Name, ValueFrom: env.ValueFrom})
			continue
		}

		// Multi-source composition: build a comma-separated value from all resolvable sources.
		// Secret-backed sources are exposed via intermediate env vars and referenced with $(VAR) expansion.
		// If there is exactly one secret-backed source and no literals, keep direct SecretKeyRef for back-compat.

		// Temporary slices to build the final env value and intermediates
		var components []string
		var intermediateVars []v1.EnvVar

		// Helper to add a secret-backed component via an intermediate env var
		addSecretComponent := func(selector v1.SecretKeySelector, idx int) {
			// Deterministic name based on target env and source index
			ivName := fmt.Sprintf("%s_%d", env.Name, idx)
			// K8s env var names must be alphanumeric + _ and not start with a number
			// The env.Name in manifest follows standard patterns; idx ensures uniqueness.
			intermediateVars = append(intermediateVars, v1.EnvVar{
				Name: ivName,
				ValueFrom: &v1.EnvVarSource{SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: selector.LocalObjectReference,
					Key:                  selector.Key,
					Optional:             selector.Optional,
				}},
			})
			components = append(components, fmt.Sprintf("$(%s)", ivName))
		}

		// Track if we only have a single secret-backed component
		singleSecretSelector := v1.SecretKeySelector{}
		secretOnlyCount := 0

		for idx, src := range env.Sources {
			switch src.Type {
			case "generatedSecret":
				if sel, ok := wandb.Status.GeneratedSecrets[src.Name]; ok {
					singleSecretSelector = sel
					secretOnlyCount++
					addSecretComponent(sel, idx)
				}
			case "mysql":
				// MySQL connection URL as a secret ref. src.Name selects the
				// instance, falling back to the default instance when empty or
				// when the named instance has no status yet.
				status, ok := v2.ResolveInstance(wandb.Status.MySQLStatus, src.Name)
				if !ok {
					continue
				}
				selector := status.Connection.URL
				// Record for potential direct assignment case
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "redis":
				status, ok := v2.ResolveInstance(wandb.Status.RedisStatus, src.Name)
				if !ok {
					continue
				}
				selector := status.Connection.URL
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "bucket":
				status, ok := v2.ResolveInstance(wandb.Status.ObjectStoreStatus, src.Name)
				if !ok {
					continue
				}
				selector := v1.SecretKeySelector{
					LocalObjectReference: status.Connection.URL.LocalObjectReference,
				}
				switch src.Field {
				case "host":
					// Host/Port/Region are provider-dependent: absent for GCS/Azure
					// and for plain AWS S3 (no custom endpoint / no region key).
					// Marks optional so pods still start when the key is missing.
					selector.Key = "Host"
					selector.Optional = ptr.To(true)
				case "port":
					selector.Key = "Port"
					selector.Optional = ptr.To(true)
				case "region":
					selector.Key = "Region"
					selector.Optional = ptr.To(true)
				default:
					selector.Key = "url"
				}
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "clickhouse":
				// clickhouse fields are provided as separate keys in the same secret
				status, ok := v2.ResolveInstance(wandb.Status.ClickHouseStatus, src.Name)
				if !ok {
					continue
				}
				selector := v1.SecretKeySelector{
					LocalObjectReference: status.Connection.URL.LocalObjectReference,
				}
				switch src.Field {
				case "host":
					selector.Key = "Host"
				case "http-port":
					selector.Key = "HTTPPort"
				case "tcp-port":
					selector.Key = "TCPPort"
				case "user":
					selector.Key = "User"
				case "password":
					selector.Key = "Password"
				case "database":
					selector.Key = "Database"
				case "url":
					selector.Key = "url"
				default:
					// Unrecognized field; skip
					continue
				}
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "kafka":
				// kafka can be referenced as a full URL (no field) or by specific fields (host/port)
				if src.Field == "" {
					selector := wandb.Status.KafkaStatus.Connection.URL
					singleSecretSelector = selector
					secretOnlyCount++
					addSecretComponent(selector, idx)
					break
				}
				selector := v1.SecretKeySelector{
					LocalObjectReference: wandb.Status.KafkaStatus.Connection.URL.LocalObjectReference,
				}
				switch src.Field {
				case "host":
					selector.Key = "Host"
				case "port":
					selector.Key = "Port"
				default:
					selector.Key = "url"
				}
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "telemetry":
				secretName := strings.TrimSpace(src.Name)
				if secretName == "" {
					secretName = strings.TrimSpace(wandb.Status.TelemetryStatus.Connection.ConnectionSecret)
				}
				if secretName == "" {
					continue
				}

				selector := v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: secretName,
					},
				}
				switch src.Field {
				case "", "metrics", "metricsEndpoint":
					selector.Key = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
				case "logs", "logsEndpoint":
					selector.Key = "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"
				case "traces", "tracesEndpoint":
					selector.Key = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
				case "metricsExporter":
					selector.Key = "OTEL_METRICS_EXPORTER"
				case "logsExporter":
					selector.Key = "OTEL_LOGS_EXPORTER"
				case "tracesExporter":
					selector.Key = "OTEL_TRACES_EXPORTER"
				case "protocol":
					selector.Key = "OTEL_EXPORTER_OTLP_PROTOCOL"
				case "serviceName":
					selector.Key = "OTEL_SERVICE_NAME"
				case "resourceAttributes":
					selector.Key = "OTEL_RESOURCE_ATTRIBUTES"
				case "gorillaTracer", "tracer":
					selector.Key = "GORILLA_TRACER"
				case "statsdHost":
					selector.Key = "GORILLA_STATSD_HOST"
				case "datadogTraceAgentURL", "ddTraceAgentURL":
					selector.Key = "DD_TRACE_AGENT_URL"
				case "datadogTraceAgentHost", "ddAgentHost":
					selector.Key = "DD_AGENT_HOST"
				case "datadogTraceAgentPort", "ddTraceAgentPort":
					selector.Key = "DD_TRACE_AGENT_PORT"
				default:
					if strings.HasPrefix(src.Field, "OTEL_") {
						selector.Key = src.Field
					} else {
						continue
					}
				}

				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "service":
				// TODO(dpanzella): Determine if this is the right approach, I think writing the service to the
				// applications status and reading from there is probably more correct
				// Prefer deterministic manifest-derived service resolution to avoid startup races
				// where the Service object has not been created yet.
				if resolved, ok := manifest.ResolveServiceURL(src); ok {
					components = append(components, resolved)
					continue
				}

				// Fallback: resolve from live Service object (back-compat).
				serviceList := &v1.ServiceList{}
				err := client.List(
					ctx,
					serviceList,
					ctrlClient.InNamespace(wandb.Namespace),
					ctrlClient.MatchingLabels{"app.kubernetes.io/name": src.Name},
				)
				if err != nil {
					return nil, err
				}
				if len(serviceList.Items) == 0 || len(serviceList.Items[0].Spec.Ports) == 0 {
					// Can't resolve; skip this component
					continue
				}
				proto := ""
				if src.Proto != "" {
					proto = fmt.Sprintf("%s://", src.Proto)
				}
				// Choose a port: prefer named match if provided; else pick the first port
				selectedPort := serviceList.Items[0].Spec.Ports[0].Port
				if src.Port != "" {
					for _, servicePort := range serviceList.Items[0].Spec.Ports {
						if servicePort.Name == src.Port {
							selectedPort = servicePort.Port
							break
						}
					}
				}
				components = append(components, fmt.Sprintf("%s%s:%d%s", proto, serviceList.Items[0].Name, selectedPort, src.Path))
			case "jwt-issuer-map":
				if wandb.Spec.Wandb.InternalServiceAuth.Enabled != nil &&
					*wandb.Spec.Wandb.InternalServiceAuth.Enabled {
					// TODO Get real OIDC Issuer
					issuer := "https://kubernetes.default.svc.cluster.local"
					if wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer != "" {
						issuer = wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer
					}
					components = append(
						components,
						fmt.Sprintf(
							"{\"system:serviceaccount:%s:%s\": \"%s\" }",
							wandb.Namespace,
							wandb.Spec.Wandb.ServiceAccount.ServiceAccountName,
							issuer,
						),
					)
				}
			case "custom-resource":
				// Read a value from the current WandB custom resource via dotted field path
				if src.Field == "" {
					// No field specified; nothing to resolve
					continue
				}
				// secret component
				if sel, ok := resolveCRFieldSecretSelector(wandb, src.Field); ok {
					singleSecretSelector = sel
					secretOnlyCount++
					addSecretComponent(sel, idx)
				} else if val, ok := resolveCRFieldString(wandb, src.Field); ok {
					// Treat as a literal component (not secret-backed)
					logger.Debug("field found in CR", "cr", wandb.Name, "field", src.Field, "value", val)
					components = append(components, val)
				} else {
					logger.Debug("field not found in CR", "cr", wandb.Name, "field", src.Field)
				}
			default:
				// Unknown source type; skip
				continue
			}
		}

		// If we built no components, skip emitting this env var
		if len(components) == 0 {
			if env.DefaultValue != "" {
				envVars = append(envVars, v1.EnvVar{Name: env.Name, Value: env.DefaultValue})
			}
			continue
		}

		// Optimization/back-compat: if there's exactly one component and it is secret-backed, emit ValueFrom directly
		if len(components) == 1 && secretOnlyCount == 1 && components[0] != "" && intermediateVars != nil {
			// Emit the single env var directly from the secret without intermediate
			envVars = append(envVars, v1.EnvVar{
				Name:      env.Name,
				ValueFrom: &v1.EnvVarSource{SecretKeyRef: &singleSecretSelector},
			})
			continue
		}

		// Otherwise, add all intermediate vars first to ensure $(VAR) expansion works
		envVars = append(envVars, intermediateVars...)
		// Then add the final composed env var
		envVars = append(envVars, v1.EnvVar{
			Name:  env.Name,
			Value: strings.Join(components, ","),
		})
	}
	return envVars, nil
}

func resolveVolumeMounts(ctx context.Context, manifest serverManifest.Manifest, commonvms []string, vms []serverManifest.VolumeMount) ([]v1.Volume, []v1.VolumeMount, error) {
	log := logx.GetSlog(ctx)

	var combinedVolumeMounts []serverManifest.VolumeMount
	for _, commonVolumeMounts := range commonvms {
		if volumeMounts, ok := manifest.CommonVolumeMounts[commonVolumeMounts]; ok {
			combinedVolumeMounts = append(combinedVolumeMounts, volumeMounts...)
		}
	}

	combinedVolumeMounts = append(combinedVolumeMounts, vms...)

	var volumes []v1.Volume
	var volumeMounts []v1.VolumeMount
	for _, manifestVM := range combinedVolumeMounts {
		volume := v1.Volume{
			Name: manifestVM.Name,
		}
		switch manifestVM.Source.Type {
		case "secret":
			volume.Secret = &v1.SecretVolumeSource{
				SecretName: manifestVM.Source.Name,
			}
		case "configMap":
			volume.ConfigMap = &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: manifestVM.Source.Name,
				},
			}
		case "emptyDir":
			volume.EmptyDir = &v1.EmptyDirVolumeSource{}
		default:
			log.Error("unsupported volume source type", "type", manifestVM.Source.Type)
			return nil, nil, fmt.Errorf("unsupported volume source type: %s", manifestVM.Source.Type)
		}
		volumeMount := v1.VolumeMount{
			MountPath: manifestVM.MountPath,
			Name:      manifestVM.Name,
		}
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
	}
	return volumes, volumeMounts, nil
}
