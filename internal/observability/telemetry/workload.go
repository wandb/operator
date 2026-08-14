package telemetry

import (
	"slices"

	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
)

var otelWorkloads = map[string]struct{}{
	"api":                               {},
	"executor":                          {},
	"filemeta":                          {},
	"filestream":                        {},
	"flat-run-fields-updater":           {},
	"glue":                              {},
	"metric-observer":                   {},
	"parquet":                           {},
	"weave-trace":                       {},
	"weave-trace-evaluate-model-worker": {},
	"weave-trace-worker":                {},
}

var statsdWorkloads = map[string]struct{}{
	"api":                     {},
	"executor":                {},
	"filemeta":                {},
	"filestream":              {},
	"flat-run-fields-updater": {},
	"glue":                    {},
	"metric-observer":         {},
	"parquet":                 {},
}

var datadogWorkloads = map[string]struct{}{
	"anaconda2":                         {},
	"weave-trace":                       {},
	"weave-trace-worker":                {},
	"weave-trace-evaluate-model-worker": {},
}

var otelFields = []string{
	"protocol",
	"tracesExporter",
	"metricsExporter",
	"logsExporter",
	"metricsEndpoint",
	"logsEndpoint",
	"tracesEndpoint",
	"serviceName",
	"resourceAttributes",
	"gorillaTracer",
}

var statsdFields = []string{"statsdAddress"}

var datadogFields = []string{
	"datadogTraceAgentURL",
	"datadogTraceAgentHost",
	"datadogTraceAgentPort",
}

func ManagedWorkloadEnvVars(applicationName string, cfg TelemetryRuntimeConfig) []serverManifest.EnvVar {
	if !cfg.Enabled {
		return nil
	}

	var fields []string
	if _, ok := otelWorkloads[applicationName]; ok {
		fields = append(fields, otelFields...)
	}
	if _, ok := statsdWorkloads[applicationName]; ok {
		fields = append(fields, statsdFields...)
	}
	isDatadogWorkload := false
	if _, ok := datadogWorkloads[applicationName]; ok {
		fields = append(fields, datadogFields...)
		isDatadogWorkload = true
	}

	sources := envVarsForFields(fields)
	if isDatadogWorkload {
		sources = append(sources, serverManifest.EnvVar{Name: envDDService, Value: applicationName})
	}
	return sources
}

func envVarsForFields(fields []string) []serverManifest.EnvVar {
	sources := make([]serverManifest.EnvVar, 0, len(fields))
	for _, field := range fields {
		key, ok := SecretKeyForField(field)
		if !ok {
			continue
		}
		sources = append(sources, serverManifest.EnvVar{
			Name: key,
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: field},
			},
		})
	}
	return sources
}

func hasAnyEnv(envVars []corev1.EnvVar, names ...string) bool {
	for _, envVar := range envVars {
		if slices.Contains(names, envVar.Name) {
			return true
		}
	}
	return false
}

func ApplyWorkloadTelemetryDefaults(envVars []corev1.EnvVar, applicationName string) []corev1.EnvVar {
	if applicationName == "" {
		return envVars
	}

	if hasAnyEnv(envVars,
		envOTLPMetricsEndpoint,
		envOTLPLogsEndpoint,
		envOTLPTracesEndpoint,
		envOTELMetricsExporter,
		envOTELLogsExporter,
		envOTELTracesExporter,
		envOTELServiceName,
		envGorillaTracer,
	) {
		envVars = setDefaultIdentityEnv(envVars, envOTELServiceName, applicationName)
	}

	if hasAnyEnv(envVars, envDDTraceAgentURL, envDDAgentHost, envDDTraceAgentPort) {
		envVars = setDefaultIdentityEnv(envVars, envDDService, applicationName)
	}

	return envVars
}

func setDefaultIdentityEnv(envVars []corev1.EnvVar, name, value string) []corev1.EnvVar {
	for i, envVar := range envVars {
		if envVar.Name != name {
			continue
		}
		if envVar.Value != "" {
			return envVars
		}
		envVars[i] = corev1.EnvVar{Name: name, Value: value}
		return envVars
	}
	return append(envVars, corev1.EnvVar{Name: name, Value: value})
}
