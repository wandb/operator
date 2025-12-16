/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var defaultRequeueMinutes = 1
var defaultRequeueDuration = time.Duration(defaultRequeueMinutes) * time.Minute

// Reconcile for V2 of WandB as the assumed object
func Reconcile(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases) (ctrl.Result, error) {
	var minioConnection *translator.InfraConnection
	var err error

	/////////////////////////
	// Write State
	if err = redisWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = mysqlWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = kafkaWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if minioConnection, err = minioWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = clickHouseWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	/////////////////////////
	// Status Update
	if err = redisReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = mysqlReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = kafkaReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = minioReadState(ctx, client, wandb, minioConnection); err != nil {
		return ctrl.Result{}, err
	}
	if err = clickHouseReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	/////////////////////////

	if err = inferState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	return reconcileWandbManifest(ctx, client, wandb)
}

func reconcileWandbManifest(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases) (ctrl.Result, error) {
	// Reconcile Wandb Manifest

	redisReady := wandb.Status.RedisStatus.Ready
	mysqlReady := wandb.Status.MySQLStatus.Ready
	kafkaReady := wandb.Status.KafkaStatus.Ready
	minioReady := wandb.Status.MinioStatus.Ready
	clickHouseReady := wandb.Status.ClickHouseStatus.Ready

	if !redisReady || !mysqlReady || !kafkaReady || !minioReady || !clickHouseReady {
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	manifest := serverManifest.Manifest{}
	manifestData, err := os.ReadFile("0.76.1.yaml")
	if err != nil {
		return ctrl.Result{}, err
	}
	if err = yaml.Unmarshal(manifestData, &manifest); err != nil {
		return ctrl.Result{}, err
	}

	// Create Strimzi KafkaTopic resources for enabled topics
	if wandb.Spec.Kafka.Enabled {
		for _, topic := range manifest.Kafka {
			if !manifestFeaturesEnabled(topic.Features, manifest.Features) {
				continue
			}

			// Determine namespace and cluster name for Strimzi resources
			kafkaNS := wandb.Spec.Kafka.Namespace
			if kafkaNS == "" {
				kafkaNS = wandb.Namespace
			}
			clusterName := wandb.Spec.Kafka.Name
			if clusterName == "" {
				// Fallback to instance name if not explicitly configured
				clusterName = wandb.Name
			}

			// Use the logical topic name as the resource name
			resName := topic.Name
			if resName == "" {
				// If not set, fallback to topic entry name
				resName = topic.Topic
			}
			if resName == "" {
				// Nothing actionable without a name
				continue
			}
			labels := map[string]string{
				"strimzi.io/cluster":           clusterName,
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/part-of":    "wandb",
				"app.kubernetes.io/instance":   wandb.Name,
			}

			// Prepare spec
			partitions := int32(1)
			if topic.PartitionCount > 0 {
				partitions = int32(topic.PartitionCount)
			}
			replicas := int32(1)
			if wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor > 0 {
				replicas = int32(wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor)
			}

			// Build typed KafkaTopic object
			kt := &strimziv1.KafkaTopic{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resName,
					Namespace: kafkaNS,
					Labels:    labels,
				},
				Spec: strimziv1.KafkaTopicSpec{
					TopicName:  topic.Topic,
					Partitions: partitions,
					Replicas:   replicas,
				},
			}

			// Create or Update
			existing := &strimziv1.KafkaTopic{}
			getErr := client.Get(ctx, types.NamespacedName{Name: kt.Name, Namespace: kafkaNS}, existing)
			if getErr != nil {
				if apiErrors.IsNotFound(getErr) {
					// Set ownerRef only if same namespace
					if kafkaNS == wandb.Namespace {
						_ = controllerutil.SetOwnerReference(wandb, kt, client.Scheme())
					}
					if err := client.Create(ctx, kt); err != nil {
						return ctrl.Result{}, err
					}
				} else {
					return ctrl.Result{}, getErr
				}
			} else {
				// Update managed spec fields and preserve other fields
				existing.Spec.TopicName = topic.Topic
				existing.Spec.Partitions = partitions
				existing.Spec.Replicas = replicas
				// Preserve/ensure labels
				exLabels := existing.GetLabels()
				if exLabels == nil {
					exLabels = map[string]string{}
				}
				for k, v := range labels {
					exLabels[k] = v
				}
				existing.SetLabels(exLabels)
				if err := client.Update(ctx, existing); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	// TODO lets run migrations here, need to add some logic for ensuring they run only once.
	//for name, migrationTask := range manifest.Migrations {
	//	// check for a currently running job
	//	jobName := fmt.Sprintf("%s-%s", wandb.Name, name)
	//	job := &batchv1.Job{}
	//	err := client.Get(
	//		ctx,
	//		types.NamespacedName{
	//			Namespace: wandb.Namespace,
	//			Name:      jobName,
	//		},
	//		job,
	//	)
	//	if err != nil && !apiErrors.IsNotFound(err) {
	//		return ctrl.Result{}, err
	//	}
	//	if apiErrors.IsNotFound(err) {
	//		job.ObjectMeta.Name = jobName
	//		job.ObjectMeta.Namespace = wandb.Namespace
	//		containerSpec := corev1.Container{
	//			Name:  name,
	//			Image: migrationTask.Image.GetImage(),
	//			Args:  migrationTask.Args,
	//		}
	//		job.Spec.Template.Spec.Containers = []corev1.Container{containerSpec}
	//		err = client.Create(ctx, job)
	//	} else {
	//		if job.Status.Succeeded == 0 {
	//			client.Delete(ctx, job)
	//		}
	//	}
	//}

	for _, app := range manifest.Applications {
		// If the application is gated behind features, only install it when
		// at least one of those features is enabled in the manifest.
		if len(app.Features) > 0 && !manifestFeaturesEnabled(app.Features, manifest.Features) {
			continue
		}
		application := &apiv2.Application{}
		applicationName := fmt.Sprintf("%s-%s", wandb.Name, app.Name)
		err := client.Get(ctx, types.NamespacedName{Name: applicationName, Namespace: wandb.Namespace}, application)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				application.ObjectMeta.Name = applicationName
				application.ObjectMeta.Namespace = wandb.Namespace
			} else {
				return ctrl.Result{}, err
			}
		}

		var combinedEnvs []serverManifest.EnvVar
		for _, commonVars := range app.CommonEnvs {
			if envvars, ok := manifest.CommonEnvvars[commonVars]; ok {
				for _, env := range envvars {
					combinedEnvs = append(combinedEnvs, env)
				}
			}
		}

		for _, env := range app.Env {
			combinedEnvs = append(combinedEnvs, env)
		}

		var envVars []corev1.EnvVar
		for _, env := range combinedEnvs {
			// If a literal value is provided, it's a simple case.
			if env.Value != "" {
				envVars = append(envVars, corev1.EnvVar{Name: env.Name, Value: env.Value})
				continue
			}

			// Multi-source composition: build a comma-separated value from all resolvable sources.
			// Secret-backed sources are exposed via intermediate env vars and referenced with $(VAR) expansion.
			// If there is exactly one secret-backed source and no literals, keep direct SecretKeyRef for back-compat.

			// Temporary slices to build the final env value and intermediates
			components := []string{}
			intermediateVars := []corev1.EnvVar{}

			// Helper to add a secret-backed component via an intermediate env var
			addSecretComponent := func(selector corev1.SecretKeySelector, idx int) {
				// Deterministic name based on target env and source index
				ivName := fmt.Sprintf("%s_%d", env.Name, idx)
				// K8s env var names must be alphanumeric + _ and not start with a number
				// The env.Name in manifest follows standard patterns; idx ensures uniqueness.
				intermediateVars = append(intermediateVars, corev1.EnvVar{
					Name: ivName,
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: selector.LocalObjectReference,
						Key:                  selector.Key,
						Optional:             selector.Optional,
					}},
				})
				components = append(components, fmt.Sprintf("$(%s)", ivName))
			}

			// Track if we only have a single secret-backed component
			singleSecretSelector := corev1.SecretKeySelector{}
			secretOnlyCount := 0

			for idx, src := range env.Sources {
				switch src.Type {
				case "mysql":
					// mysql connection URL as a secret ref
					selector := wandb.Status.MySQLStatus.Connection.URL
					// Record for potential direct assignment case
					singleSecretSelector = selector
					secretOnlyCount++
					addSecretComponent(selector, idx)
				case "redis":
					selector := wandb.Status.RedisStatus.Connection.URL
					singleSecretSelector = selector
					secretOnlyCount++
					addSecretComponent(selector, idx)
				case "bucket":
					selector := wandb.Status.MinioStatus.Connection.URL
					singleSecretSelector = selector
					secretOnlyCount++
					addSecretComponent(selector, idx)
				case "clickhouse":
					// clickhouse fields are provided as separate keys in the same secret
					selector := corev1.SecretKeySelector{
						LocalObjectReference: wandb.Status.ClickHouseStatus.Connection.URL.LocalObjectReference,
					}
					switch src.Field {
					case "host":
						selector.Key = "Host"
					case "port":
						selector.Key = "Port"
					case "user":
						selector.Key = "User"
					case "password":
						selector.Key = "Password"
					case "database":
						selector.Key = "Database"
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
					selector := corev1.SecretKeySelector{
						LocalObjectReference: wandb.Status.KafkaStatus.Connection.URL.LocalObjectReference,
					}
					switch src.Field {
					case "host":
						selector.Key = "Host"
					case "port":
						selector.Key = "Port"
					case "url":
						selector.Key = "url"
					default:
						// Unrecognized field; skip
						continue
					}
					singleSecretSelector = selector
					secretOnlyCount++
					addSecretComponent(selector, idx)
				case "service":
					// Resolve to a literal URL (proto://serviceName:port/path)
					serviceList := &corev1.ServiceList{}
					targetApplicationName := fmt.Sprintf("%s-%s", wandb.Name, src.Name)
					err := client.List(
						ctx,
						serviceList,
						ctrlClient.InNamespace(wandb.Namespace),
						ctrlClient.MatchingLabels{"app.kubernetes.io/name": targetApplicationName},
					)
					if err != nil {
						return ctrl.Result{}, err
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
				case "custom-resource":
					// Read a value from the current WandB custom resource via dotted field path
					if src.Field == "" {
						// No field specified; nothing to resolve
						continue
					}
					if val, ok := resolveCRFieldString(wandb, src.Field); ok {
						// Treat as a literal component (not secret-backed)
						components = append(components, val)
					}
				default:
					// Unknown source type; skip
					continue
				}
			}

			// If we built no components, skip emitting this env var
			if len(components) == 0 {
				continue
			}

			// Optimization/back-compat: if there's exactly one component and it is secret-backed, emit ValueFrom directly
			if len(components) == 1 && secretOnlyCount == 1 && components[0] != "" && intermediateVars != nil {
				// Emit the single env var directly from the secret without intermediate
				envVars = append(envVars, corev1.EnvVar{
					Name:      env.Name,
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &singleSecretSelector},
				})
				continue
			}

			// Otherwise, add all intermediate vars first to ensure $(VAR) expansion works
			envVars = append(envVars, intermediateVars...)
			// Then add the final composed env var
			envVars = append(envVars, corev1.EnvVar{
				Name:  env.Name,
				Value: strings.Join(components, ","),
			})
		}

		var Ports []corev1.ContainerPort
		for _, port := range app.Ports {
			containerPort := corev1.ContainerPort{
				Name:          port.Name,
				ContainerPort: port.ContainerPort,
				Protocol:      port.Protocol,
			}
			Ports = append(Ports, containerPort)
		}

		container := corev1.Container{
			Name:    app.Name,
			Image:   app.Image.GetImage(),
			Env:     envVars,
			Args:    app.Args,
			Command: app.Command,
			Ports:   Ports,
		}

		// Handle file injection via ConfigMaps according to manifest Application.Files
		volumes := []corev1.Volume{}
		volumeMounts := []corev1.VolumeMount{}
		if len(app.Files) > 0 {
			// Collect inline files into a single operator-managed ConfigMap
			inlineData := map[string]string{}
			inlineCMName := fmt.Sprintf("%s-%s-files", wandb.Name, app.Name)
			// Track external ConfigMap refs and create one Volume per unique ref
			cmRefVolumeNames := map[string]string{}

			for _, f := range app.Files {
				key := f.Name
				fileName := f.FileName
				if fileName == "" {
					fileName = key
				}

				var volName string
				if f.Inline != "" {
					// Accumulate into inline CM data
					inlineData[key] = f.Inline
					volName = "files-inline"
				} else if f.ConfigMapRef != "" {
					// external ConfigMap reference
					if existing, ok := cmRefVolumeNames[f.ConfigMapRef]; ok {
						volName = existing
					} else {
						volName = fmt.Sprintf("cm-%s", f.ConfigMapRef)
						cmRefVolumeNames[f.ConfigMapRef] = volName
					}
				} else {
					// neither inline nor ref provided; skip
					continue
				}

				// Mount each file as a single file using subPath into the specified directory
				mountPath := f.MountPath
				if mountPath == "" {
					// require a mountPath; skip if not provided
					continue
				}
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      volName,
					MountPath: fmt.Sprintf("%s/%s", mountPath, fileName),
					SubPath:   key,
					ReadOnly:  true,
				})
			}

			// Create/update inline ConfigMap if we have any inline data
			if len(inlineData) > 0 {
				cm := &corev1.ConfigMap{}
				cm.Namespace = wandb.Namespace
				cm.Name = inlineCMName
				// Try to get existing
				getErr := client.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, cm)
				if getErr != nil {
					if apiErrors.IsNotFound(getErr) {
						cm.Data = inlineData
						if err := controllerutil.SetOwnerReference(wandb, cm, client.Scheme()); err != nil {
							return ctrl.Result{}, err
						}
						if err := client.Create(ctx, cm); err != nil {
							return ctrl.Result{}, err
						}
					} else {
						return ctrl.Result{}, getErr
					}
				} else {
					// Update data if changed
					if cm.Data == nil {
						cm.Data = map[string]string{}
					}
					cm.Data = inlineData
					if err := client.Update(ctx, cm); err != nil {
						return ctrl.Result{}, err
					}
				}

				// Add a volume for the inline CM
				volumes = append(volumes, corev1.Volume{
					Name: "files-inline",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: inlineCMName}},
					},
				})
			}

			// Add volumes for each external ConfigMap ref
			for ref, volName := range cmRefVolumeNames {
				volumes = append(volumes, corev1.Volume{
					Name: volName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: ref}},
					},
				})
			}

			// Attach mounts to the container if any
			if len(volumeMounts) > 0 {
				container.VolumeMounts = append(container.VolumeMounts, volumeMounts...)
			}
		}

		initContainers := []corev1.Container{}

		if app.InitContainers != nil {
			for _, initContainerSpec := range app.InitContainers {
				initContainer := corev1.Container{
					Name:    initContainerSpec.Name,
					Image:   initContainerSpec.Image.GetImage(),
					Env:     envVars,
					Args:    initContainerSpec.Args,
					Command: initContainerSpec.Command,
				}
				initContainers = append(initContainers, initContainer)
			}
		}

		application.Spec.Kind = "Deployment"
		application.Spec.PodTemplate.Spec.Containers = []corev1.Container{container}
		// Replace volumes entirely on each reconcile to avoid accumulating duplicates
		// across updates (e.g., duplicate "files-inline" volume names).
		application.Spec.PodTemplate.Spec.Volumes = volumes
		application.Spec.PodTemplate.Spec.InitContainers = initContainers

		// Reconcile Service ports: fully replace the ServiceTemplate ports with
		// the ports declared in the manifest for this app. This ensures that any
		// change to port numbers, names, or protocols is propagated on each
		// reconcile. If no service ports are declared, clear the ServiceTemplate.
		if app.Service != nil && len(app.Service.Ports) > 0 {
			ports := make([]corev1.ServicePort, 0, len(app.Service.Ports))
			for _, p := range app.Service.Ports {
				ports = append(ports, corev1.ServicePort{
					Name:     p.Name,
					Port:     p.Port,
					Protocol: p.Protocol,
				})
			}
			if application.Spec.ServiceTemplate == nil {
				application.Spec.ServiceTemplate = &corev1.ServiceSpec{}
			}
			// Replace ports entirely to avoid stale or duplicate entries
			application.Spec.ServiceTemplate.Ports = ports
		} else {
			// No service declared in manifest; ensure we clear any previous template
			application.Spec.ServiceTemplate = nil
		}

		err = controllerutil.SetOwnerReference(wandb, application, client.Scheme())
		if err != nil {
			return ctrl.Result{}, err
		}

		if application.ObjectMeta.CreationTimestamp.IsZero() {
			if err = client.Create(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err = client.Update(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
		}

	}
	return ctrl.Result{}, nil
}

// manifestFeaturesEnabled returns true if any of the topic's feature flags are enabled
// in the manifest's top-level Features section.
func manifestFeaturesEnabled(topicFeatures []string, mf *serverManifest.Features) bool {
	if len(topicFeatures) == 0 || mf == nil {
		return false
	}
	for _, f := range topicFeatures {
		switch f {
		case "runsV2":
			if mf.RunsV2 {
				return true
			}
		case "filestreamQueue":
			if mf.FilestreamQueue {
				return true
			}
		case "metricObserver":
			if mf.MetricObserver {
				return true
			}
		case "weaveTrace":
			if mf.WeaveTrace {
				return true
			}
		case "proxy":
			if mf.Proxy {
				return true
			}
		case "weaveTraceWorkers":
			if mf.WeaveTraceWorkers {
				return true
			}
		}
	}
	return false
}

// resolveCRFieldString resolves a dotted field path (e.g., "spec.wandb.license")
// from the provided custom resource object, returning the string value if present.
// Non-string terminal values are treated as not found.
func resolveCRFieldString(obj any, path string) (string, bool) {
	if obj == nil || path == "" {
		return "", false
	}
	// Marshal to JSON then unmarshal into a generic map for easy traversal.
	b, err := json.Marshal(obj)
	if err != nil {
		return "", false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "", false
	}
	cur := any(m)
	for _, seg := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		next, ok := mm[seg]
		if !ok {
			return "", false
		}
		cur = next
	}
	if s, ok := cur.(string); ok {
		return s, true
	}
	return "", false
}

func inferState(
	ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	// Infra is "ok" if either it is not enabled or if it is (enabled and) ready
	redisOk := !wandb.Spec.Redis.Enabled || wandb.Status.RedisStatus.Ready
	minioOk := !wandb.Spec.Minio.Enabled || wandb.Status.MinioStatus.Ready
	mysqlOk := !wandb.Spec.MySQL.Enabled || wandb.Status.MySQLStatus.Ready
	clickHouseOk := !wandb.Spec.ClickHouse.Enabled || wandb.Status.ClickHouseStatus.Ready
	kafkaOk := !wandb.Spec.Kafka.Enabled || wandb.Status.KafkaStatus.Ready

	if redisOk && minioOk && mysqlOk && clickHouseOk && kafkaOk {
		wandb.Status.State = "Ready"
	} else {
		wandb.Status.State = "NotReady"
	}

	if err := client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status")
		return err
	}
	return nil
}
