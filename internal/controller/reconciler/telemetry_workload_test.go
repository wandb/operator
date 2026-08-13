package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveEnvvarsTelemetrySourceUsesStatusSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Status: apiv2.WeightsAndBiasesStatus{
			TelemetryStatus: apiv2.TelemetryInfraStatus{
				Connection: apiv2.TelemetryConnectionStatus{
					ConnectionSecret: "status-otel-secret",
				},
			},
		},
	}
	envs := []serverManifest.EnvVar{
		{
			Name: "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "metricsEndpoint"},
			},
		},
		{
			Name: "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Name: "custom-otel", Field: "logsEndpoint"},
			},
		},
		{
			Name: "OTEL_TRACES_EXPORTER",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "tracesExporter"},
			},
		},
		{
			Name: "GORILLA_TRACER",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "gorillaTracer"},
			},
		},
		{
			Name: "GORILLA_STATSD_ADDRESS",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "statsdAddress"},
			},
		},
		{
			Name: "DD_TRACE_AGENT_URL",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "datadogTraceAgentURL"},
			},
		},
		{
			Name: "DD_AGENT_HOST",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "datadogTraceAgentHost"},
			},
		},
		{
			Name: "DD_TRACE_AGENT_PORT",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "datadogTraceAgentPort"},
			},
		},
		{
			Name: "OTEL_PROTOCOL_AND_SERVICE",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "protocol"},
				{Type: "telemetry", Field: "serviceName"},
			},
		},
	}

	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, envs)
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}

	metricsEnv := mustFindEnvVar(t, resolved, "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
	if metricsEnv.ValueFrom == nil || metricsEnv.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected metrics endpoint to resolve from secret key ref")
	}
	if metricsEnv.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected metrics secret name: %s", metricsEnv.ValueFrom.SecretKeyRef.Name)
	}
	if metricsEnv.ValueFrom.SecretKeyRef.Key != "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT" {
		t.Fatalf("unexpected metrics key: %s", metricsEnv.ValueFrom.SecretKeyRef.Key)
	}

	logsEnv := mustFindEnvVar(t, resolved, "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT")
	if logsEnv.ValueFrom == nil || logsEnv.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected logs endpoint to resolve from secret key ref")
	}
	if logsEnv.ValueFrom.SecretKeyRef.Name != "custom-otel" {
		t.Fatalf("unexpected logs secret name: %s", logsEnv.ValueFrom.SecretKeyRef.Name)
	}
	if logsEnv.ValueFrom.SecretKeyRef.Key != "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT" {
		t.Fatalf("unexpected logs key: %s", logsEnv.ValueFrom.SecretKeyRef.Key)
	}

	tracesExporter := mustFindEnvVar(t, resolved, "OTEL_TRACES_EXPORTER")
	if tracesExporter.ValueFrom == nil || tracesExporter.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected traces exporter to resolve from secret key ref")
	}
	if tracesExporter.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected traces exporter secret name: %s", tracesExporter.ValueFrom.SecretKeyRef.Name)
	}
	if tracesExporter.ValueFrom.SecretKeyRef.Key != "OTEL_TRACES_EXPORTER" {
		t.Fatalf("unexpected traces exporter key: %s", tracesExporter.ValueFrom.SecretKeyRef.Key)
	}

	gorillaTracer := mustFindEnvVar(t, resolved, "GORILLA_TRACER")
	if gorillaTracer.ValueFrom == nil || gorillaTracer.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected gorilla tracer to resolve from secret key ref")
	}
	if gorillaTracer.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected gorilla tracer secret name: %s", gorillaTracer.ValueFrom.SecretKeyRef.Name)
	}
	if gorillaTracer.ValueFrom.SecretKeyRef.Key != "GORILLA_TRACER" {
		t.Fatalf("unexpected gorilla tracer key: %s", gorillaTracer.ValueFrom.SecretKeyRef.Key)
	}

	statsdAddress := mustFindEnvVar(t, resolved, "GORILLA_STATSD_ADDRESS")
	if statsdAddress.ValueFrom == nil || statsdAddress.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected gorilla statsd address to resolve from secret key ref")
	}
	if statsdAddress.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected gorilla statsd secret name: %s", statsdAddress.ValueFrom.SecretKeyRef.Name)
	}
	if statsdAddress.ValueFrom.SecretKeyRef.Key != "GORILLA_STATSD_ADDRESS" {
		t.Fatalf("unexpected gorilla statsd key: %s", statsdAddress.ValueFrom.SecretKeyRef.Key)
	}

	ddTraceAgentURL := mustFindEnvVar(t, resolved, "DD_TRACE_AGENT_URL")
	if ddTraceAgentURL.ValueFrom == nil || ddTraceAgentURL.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected Datadog trace agent URL to resolve from secret key ref")
	}
	if ddTraceAgentURL.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected Datadog trace agent URL secret name: %s", ddTraceAgentURL.ValueFrom.SecretKeyRef.Name)
	}
	if ddTraceAgentURL.ValueFrom.SecretKeyRef.Key != "DD_TRACE_AGENT_URL" {
		t.Fatalf("unexpected Datadog trace agent URL key: %s", ddTraceAgentURL.ValueFrom.SecretKeyRef.Key)
	}

	ddAgentHost := mustFindEnvVar(t, resolved, "DD_AGENT_HOST")
	if ddAgentHost.ValueFrom == nil || ddAgentHost.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected Datadog agent host to resolve from secret key ref")
	}
	if ddAgentHost.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected Datadog agent host secret name: %s", ddAgentHost.ValueFrom.SecretKeyRef.Name)
	}
	if ddAgentHost.ValueFrom.SecretKeyRef.Key != "DD_AGENT_HOST" {
		t.Fatalf("unexpected Datadog agent host key: %s", ddAgentHost.ValueFrom.SecretKeyRef.Key)
	}

	ddTraceAgentPort := mustFindEnvVar(t, resolved, "DD_TRACE_AGENT_PORT")
	if ddTraceAgentPort.ValueFrom == nil || ddTraceAgentPort.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected Datadog trace agent port to resolve from secret key ref")
	}
	if ddTraceAgentPort.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected Datadog trace agent port secret name: %s", ddTraceAgentPort.ValueFrom.SecretKeyRef.Name)
	}
	if ddTraceAgentPort.ValueFrom.SecretKeyRef.Key != "DD_TRACE_AGENT_PORT" {
		t.Fatalf("unexpected Datadog trace agent port key: %s", ddTraceAgentPort.ValueFrom.SecretKeyRef.Key)
	}

	protocolComponent := mustFindEnvVar(t, resolved, "OTEL_PROTOCOL_AND_SERVICE_0")
	if protocolComponent.ValueFrom == nil || protocolComponent.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected protocol component to resolve from secret")
	}
	if protocolComponent.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected protocol secret name: %s", protocolComponent.ValueFrom.SecretKeyRef.Name)
	}
	if protocolComponent.ValueFrom.SecretKeyRef.Key != "OTEL_EXPORTER_OTLP_PROTOCOL" {
		t.Fatalf("unexpected protocol key: %s", protocolComponent.ValueFrom.SecretKeyRef.Key)
	}

	serviceComponent := mustFindEnvVar(t, resolved, "OTEL_PROTOCOL_AND_SERVICE_1")
	if serviceComponent.ValueFrom == nil || serviceComponent.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected service component to resolve from secret")
	}
	if serviceComponent.ValueFrom.SecretKeyRef.Name != "status-otel-secret" {
		t.Fatalf("unexpected service secret name: %s", serviceComponent.ValueFrom.SecretKeyRef.Name)
	}
	if serviceComponent.ValueFrom.SecretKeyRef.Key != "OTEL_SERVICE_NAME" {
		t.Fatalf("unexpected service key: %s", serviceComponent.ValueFrom.SecretKeyRef.Key)
	}

	joined := mustFindEnvVar(t, resolved, "OTEL_PROTOCOL_AND_SERVICE")
	if joined.Value != "$(OTEL_PROTOCOL_AND_SERVICE_0),$(OTEL_PROTOCOL_AND_SERVICE_1)" {
		t.Fatalf("unexpected joined telemetry env value: %s", joined.Value)
	}
}

func TestResolveEnvvarsTelemetrySourceWithoutStatusSecretSkipsEnv(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"}}
	envs := []serverManifest.EnvVar{
		{
			Name: "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
			Sources: []serverManifest.EnvSource{
				{Type: "telemetry", Field: "metricsEndpoint"},
			},
		},
	}

	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, envs)
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}
	if len(resolved) != 0 {
		t.Fatalf("expected telemetry source without status secret to be skipped, got %#v", resolved)
	}
}

func TestResolveEnvvarsServiceSourceFromManifest(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb-dev-v2", Namespace: "default"}}
	manifest := serverManifest.Manifest{
		Applications: map[string]serverManifest.Application{
			"anaconda2": {
				Name: "anaconda2",
				Service: &serverManifest.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "anaconda2", Port: 8080},
					},
				},
			},
		},
	}
	envs := []serverManifest.EnvVar{
		{
			Name: "GORILLA_SWEEP_PROVIDER",
			Sources: []serverManifest.EnvSource{
				{Type: "service", Name: "anaconda2", Proto: "http"},
			},
		},
	}

	resolved, err := resolveEnvvars(context.Background(), client, wandb, manifest, nil, envs)
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}

	sweepProvider := mustFindEnvVar(t, resolved, "GORILLA_SWEEP_PROVIDER")
	if sweepProvider.Value != "http://anaconda2.default.svc.cluster.local:8080" {
		t.Fatalf("unexpected sweep provider value: %s", sweepProvider.Value)
	}
}

func TestResolveEnvvarsServiceSourcePortNameFromManifest(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb-dev-v2", Namespace: "default"}}
	manifest := serverManifest.Manifest{
		Applications: map[string]serverManifest.Application{
			"parquet": {
				Name: "parquet",
				Service: &serverManifest.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "api", Port: 8080},
						{Name: "parquet", Port: 9000},
					},
				},
			},
		},
	}
	envs := []serverManifest.EnvVar{
		{
			Name: "GORILLA_HISTORY_STORE",
			Sources: []serverManifest.EnvSource{
				{Type: "service", Name: "parquet", Port: "parquet", Proto: "http", Path: "/_goRPC_"},
			},
		},
	}

	resolved, err := resolveEnvvars(context.Background(), client, wandb, manifest, nil, envs)
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}

	historyStore := mustFindEnvVar(t, resolved, "GORILLA_HISTORY_STORE")
	if historyStore.Value != "http://parquet.default.svc.cluster.local:9000/_goRPC_" {
		t.Fatalf("unexpected history store value: %s", historyStore.Value)
	}
}

func TestApplyWorkloadTelemetryDefaultsOverridesSharedServiceName(t *testing.T) {
	envVars := []corev1.EnvVar{
		{
			Name: "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-otel-connection"},
					Key:                  "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
				},
			},
		},
		{
			Name: "OTEL_SERVICE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-otel-connection"},
					Key:                  "OTEL_SERVICE_NAME",
				},
			},
		},
	}

	resolved := applyWorkloadTelemetryDefaults(envVars, "parquet")

	serviceName := mustFindEnvVar(t, resolved, "OTEL_SERVICE_NAME")
	if serviceName.Value != "parquet" {
		t.Fatalf("expected workload-specific OTEL service name, got %q", serviceName.Value)
	}
	if serviceName.ValueFrom != nil {
		t.Fatalf("expected workload-specific OTEL service name to be a literal override")
	}
}

func TestApplyWorkloadTelemetryDefaultsPreservesExplicitServiceName(t *testing.T) {
	envVars := []corev1.EnvVar{
		{Name: "OTEL_METRICS_EXPORTER", Value: "otlp"},
		{Name: "OTEL_SERVICE_NAME", Value: "custom-service-name"},
	}

	resolved := applyWorkloadTelemetryDefaults(envVars, "parquet")

	serviceName := mustFindEnvVar(t, resolved, "OTEL_SERVICE_NAME")
	if serviceName.Value != "custom-service-name" {
		t.Fatalf("expected explicit OTEL service name to be preserved, got %q", serviceName.Value)
	}
}

//func TestInjectManagedWorkloadTelemetryEnvvarsCoverage(t *testing.T) {
//	scheme := runtime.NewScheme()
//	if err := corev1.AddToScheme(scheme); err != nil {
//		t.Fatalf("failed adding corev1 to scheme: %v", err)
//	}
//	client := fake.NewClientBuilder().WithScheme(scheme).Build()
//	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb-dev-v2", Namespace: "default"}}
//	manifest := serverManifest.Manifest{}
//
//	expectedApps := []string{
//		"api",
//		"executor",
//		"filemeta",
//		"filestream",
//		"flat-run-fields-updater",
//		"glue",
//		"metric-observer",
//		"parquet",
//		"weave",
//		"weave-trace",
//		"weave-trace-worker",
//		"weave-trace-evaluate-model-worker",
//		"nginx-proxy",
//	}
//
//	for _, appName := range expectedApps {
//		t.Run(appName, func(t *testing.T) {
//			envVars, err := injectManagedWorkloadTelemetryEnvvars(
//				context.Background(),
//				client,
//				wandb,
//				manifest,
//				serverManifest.Application{Name: appName},
//				nil,
//				true,
//			)
//			if err != nil {
//				t.Fatalf("injectManagedWorkloadTelemetryEnvvars returned error: %v", err)
//			}
//
//			for _, envName := range []string{
//				"OTEL_EXPORTER_OTLP_PROTOCOL",
//				"OTEL_TRACES_EXPORTER",
//				"OTEL_METRICS_EXPORTER",
//				"OTEL_LOGS_EXPORTER",
//				"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
//				"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
//				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
//				"OTEL_SERVICE_NAME",
//				"OTEL_RESOURCE_ATTRIBUTES",
//				"GORILLA_TRACER",
//			} {
//				if !coreEnvVarSliceContains(envVars, envName) {
//					t.Fatalf("expected managed telemetry env %q for application %q", envName, appName)
//				}
//			}
//		})
//	}
//
//	t.Run("ineligible-workload", func(t *testing.T) {
//		envVars, err := injectManagedWorkloadTelemetryEnvvars(
//			context.Background(),
//			client,
//			wandb,
//			manifest,
//			serverManifest.Application{Name: "anaconda2"},
//			nil,
//			true,
//		)
//		if err != nil {
//			t.Fatalf("injectManagedWorkloadTelemetryEnvvars returned error: %v", err)
//		}
//		if len(envVars) != 0 {
//			t.Fatalf("expected no managed telemetry envs for ineligible workload, got %#v", envVars)
//		}
//	})
//}

func TestInjectManagedWorkloadTelemetryEnvvarsDisabledSkipsInjection(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb-dev-v2", Namespace: "default"}}

	envVars, err := injectManagedWorkloadTelemetryEnvvars(
		context.Background(),
		client,
		wandb,
		serverManifest.Manifest{},
		serverManifest.Application{Name: "api"},
		nil,
		telemetry.TelemetryRuntimeConfig{Enabled: false},
	)
	if err != nil {
		t.Fatalf("injectManagedWorkloadTelemetryEnvvars returned error: %v", err)
	}
	if len(envVars) != 0 {
		t.Fatalf("expected no managed telemetry envs when telemetry is disabled, got %#v", envVars)
	}
}

func TestInjectManagedWorkloadTelemetryEnvvarsIneligibleWorkloadSkipsInjection(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	wandb := telemetryStatusWandb("wandb-dev-v2", "default", "wandb-otel-connection")

	envVars, err := injectManagedWorkloadTelemetryEnvvars(
		context.Background(),
		client,
		wandb,
		serverManifest.Manifest{},
		serverManifest.Application{Name: "frontend"},
		nil,
		telemetry.TelemetryRuntimeConfig{Enabled: true},
	)
	if err != nil {
		t.Fatalf("injectManagedWorkloadTelemetryEnvvars returned error: %v", err)
	}
	if len(envVars) != 0 {
		t.Fatalf("expected no managed telemetry envs for ineligible workload, got %#v", envVars)
	}
}

func mustFindEnvVar(t *testing.T, envs []corev1.EnvVar, name string) corev1.EnvVar {
	t.Helper()
	for _, env := range envs {
		if env.Name == name {
			return env
		}
	}
	t.Fatalf("env var %q not found", name)
	return corev1.EnvVar{}
}

func TestInjectManagedWorkloadTelemetryEnvvarsUsesStatusSecretName(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	wandb := telemetryStatusWandb("wandb-dev-v2", "default", "status-otel-secret")

	telemetryConfig := telemetry.TelemetryRuntimeConfig{
		Enabled: true,
	}

	envVars, err := injectManagedWorkloadTelemetryEnvvars(
		context.Background(),
		client,
		wandb,
		serverManifest.Manifest{},
		serverManifest.Application{Name: "api"},
		nil,
		telemetryConfig,
	)
	if err != nil {
		t.Fatalf("injectManagedWorkloadTelemetryEnvvars returned error: %v", err)
	}

	expectedNames := []string{
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_TRACES_EXPORTER",
		"OTEL_METRICS_EXPORTER",
		"OTEL_LOGS_EXPORTER",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_SERVICE_NAME",
		"OTEL_RESOURCE_ATTRIBUTES",
		"GORILLA_TRACER",
		"GORILLA_STATSD_ADDRESS",
	}

	for _, name := range expectedNames {
		env := mustFindEnvVar(t, envVars, name)

		if env.ValueFrom == nil {
			t.Errorf("env var %q has no ValueFrom; expected SecretKeyRef to status-otel-secret", name)
			continue
		}
		if env.ValueFrom.SecretKeyRef == nil {
			t.Errorf("env var %q ValueFrom has no SecretKeyRef; expected reference to status-otel-secret", name)
			continue
		}
		if got := env.ValueFrom.SecretKeyRef.Name; got != "status-otel-secret" {
			t.Errorf("env var %q references secret %q, want status-otel-secret", name, got)
		}
	}
}

func TestInjectManagedWorkloadTelemetryEnvvarsAddsDatadogAgentForDdtraceApps(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	wandb := telemetryStatusWandb("wandb-dev-v2", "default", "status-otel-secret")

	for _, appName := range []string{"anaconda2", "weave-trace"} {
		t.Run(appName, func(t *testing.T) {
			envVars, err := injectManagedWorkloadTelemetryEnvvars(
				context.Background(),
				client,
				wandb,
				serverManifest.Manifest{},
				serverManifest.Application{Name: appName},
				nil,
				telemetry.TelemetryRuntimeConfig{Enabled: true},
			)
			if err != nil {
				t.Fatalf("injectManagedWorkloadTelemetryEnvvars returned error: %v", err)
			}

			for _, name := range []string{"DD_TRACE_AGENT_URL", "DD_AGENT_HOST", "DD_TRACE_AGENT_PORT"} {
				env := mustFindEnvVar(t, envVars, name)
				if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
					t.Fatalf("env var %q has no SecretKeyRef", name)
				}
				if got := env.ValueFrom.SecretKeyRef.Name; got != "status-otel-secret" {
					t.Fatalf("env var %q references secret %q, want status-otel-secret", name, got)
				}
			}

			ddService := mustFindEnvVar(t, envVars, "DD_SERVICE")
			if ddService.Value != appName {
				t.Fatalf("unexpected DD_SERVICE value: %q", ddService.Value)
			}

			for _, name := range []string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "GORILLA_TRACER", "GORILLA_STATSD_ADDRESS"} {
				for _, env := range envVars {
					if env.Name == name {
						t.Fatalf("did not expect %q to be injected for ddtrace-only app", name)
					}
				}
			}
		})
	}
}

func telemetryStatusWandb(name, namespace, secretName string) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Status: apiv2.WeightsAndBiasesStatus{
			TelemetryStatus: apiv2.TelemetryInfraStatus{
				Connection: apiv2.TelemetryConnectionStatus{
					ConnectionSecret: secretName,
				},
			},
		},
	}
}

func TestManagedWorkloadTelemetryApplicationsContainsWeaveTraceApps(t *testing.T) {
	for _, name := range []string{
		"weave-trace",
		"weave-trace-worker",
		"weave-trace-evaluate-model-worker",
	} {
		if _, ok := managedWorkloadTelemetryApplications[name]; !ok {
			t.Errorf("%q should be in the OTel allowlist", name)
		}
	}
}
