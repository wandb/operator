package telemetry

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestManagedWorkloadEnvVars(t *testing.T) {
	tests := []struct {
		name string
		app  string
		want []string
	}{
		{
			name: "OTEL and StatsD workload",
			app:  "api",
			want: []string{
				envOTLPProtocol,
				envOTELTracesExporter,
				envOTELMetricsExporter,
				envOTELLogsExporter,
				envOTLPMetricsEndpoint,
				envOTLPLogsEndpoint,
				envOTLPTracesEndpoint,
				envOTELServiceName,
				envOTELResourceAttrs,
				envGorillaTracer,
				envGorillaStatsd,
			},
		},
		{
			name: "Datadog-only workload",
			app:  "anaconda2",
			want: []string{envDDTraceAgentURL, envDDAgentHost, envDDTraceAgentPort, envDDService},
		},
		{
			name: "OTEL and Datadog workload",
			app:  "weave-trace",
			want: []string{
				envOTLPProtocol,
				envOTELTracesExporter,
				envOTELMetricsExporter,
				envOTELLogsExporter,
				envOTLPMetricsEndpoint,
				envOTLPLogsEndpoint,
				envOTLPTracesEndpoint,
				envOTELServiceName,
				envOTELResourceAttrs,
				envGorillaTracer,
				envDDTraceAgentURL,
				envDDAgentHost,
				envDDTraceAgentPort,
				envDDService,
			},
		},
		{name: "unmanaged workload", app: "frontend"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := ManagedWorkloadEnvVars(tt.app, TelemetryRuntimeConfig{Enabled: true})
			if len(sources) != len(tt.want) {
				t.Fatalf("got %d sources, want %d: %#v", len(sources), len(tt.want), sources)
			}
			for i, want := range tt.want {
				if sources[i].Name != want {
					t.Errorf("source %d name = %q, want %q", i, sources[i].Name, want)
				}
				if sources[i].Name == envDDService {
					if sources[i].Value != tt.app {
						t.Errorf("%s = %q, want %q", envDDService, sources[i].Value, tt.app)
					}
					continue
				}
				if len(sources[i].Sources) != 1 || sources[i].Sources[0].Type != "telemetry" {
					t.Errorf("source %d is not a telemetry source: %#v", i, sources[i])
				}
			}
		})
	}
}

func TestManagedWorkloadEnvVarsDisabled(t *testing.T) {
	if got := ManagedWorkloadEnvVars("api", TelemetryRuntimeConfig{Enabled: false}); len(got) != 0 {
		t.Fatalf("disabled telemetry returned sources: %#v", got)
	}
}

func TestApplyWorkloadTelemetryDefaultsAddsDatadogService(t *testing.T) {
	envVars := ApplyWorkloadTelemetryDefaults([]corev1.EnvVar{
		{Name: envDDTraceAgentURL, Value: "http://agent:8126"},
	}, "weave-trace")

	for _, envVar := range envVars {
		if envVar.Name == envDDService {
			if envVar.Value != "weave-trace" {
				t.Fatalf("%s = %q, want weave-trace", envDDService, envVar.Value)
			}
			return
		}
	}
	t.Fatalf("%s was not added", envDDService)
}
