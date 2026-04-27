package v2

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTelemetryRuntimeConfigResolveEndpointsEnabled(t *testing.T) {
	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Enabled = true
	cfg.Namespace = "wandb"
	cfg.Normalize()

	resolved := cfg.ResolveEndpoints()
	if resolved.MetricsEndpoint != "http://victoria-otlp-gateway.wandb.svc:4318/v1/metrics" {
		t.Fatalf("unexpected metrics endpoint: %s", resolved.MetricsEndpoint)
	}
	if resolved.LogsEndpoint != "http://victoria-otlp-gateway.wandb.svc:4318/v1/logs" {
		t.Fatalf("unexpected logs endpoint: %s", resolved.LogsEndpoint)
	}
	if resolved.TracesEndpoint != "http://victoria-otlp-gateway.wandb.svc:4318/v1/traces" {
		t.Fatalf("unexpected traces endpoint: %s", resolved.TracesEndpoint)
	}
}

func TestTelemetryRuntimeConfigResolveEndpointsDisabled(t *testing.T) {
	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Enabled = false
	cfg.Normalize()

	resolved := cfg.ResolveEndpoints()
	if resolved.MetricsEndpoint != "" || resolved.LogsEndpoint != "" || resolved.TracesEndpoint != "" {
		t.Fatalf("expected empty telemetry endpoints when telemetry is disabled: %+v", resolved)
	}
}

func TestReconcileTelemetryConnectionSecretCreateManaged(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding appsv2 to scheme: %v", err)
	}

	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb).Build()

	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Enabled = true
	cfg.Normalize()

	if err := reconcileTelemetryConnectionSecret(context.Background(), client, wandb, cfg); err != nil {
		t.Fatalf("reconcileTelemetryConnectionSecret returned error: %v", err)
	}

	secret := &corev1.Secret{}
	lookup := types.NamespacedName{Name: cfg.OTel.SecretName, Namespace: wandb.Namespace}
	if err := client.Get(context.Background(), lookup, secret); err != nil {
		t.Fatalf("failed retrieving telemetry secret: %v", err)
	}

	if got := string(secret.Data["OTEL_EXPORTER_OTLP_PROTOCOL"]); got != "http/protobuf" {
		t.Fatalf("unexpected protocol in secret: %q", got)
	}
	if got := string(secret.Data["OTEL_METRICS_EXPORTER"]); got != "otlp" {
		t.Fatalf("unexpected metrics exporter in secret: %q", got)
	}
	if got := string(secret.Data["OTEL_LOGS_EXPORTER"]); got != "otlp" {
		t.Fatalf("unexpected logs exporter in secret: %q", got)
	}
	if got := string(secret.Data["OTEL_TRACES_EXPORTER"]); got != "otlp" {
		t.Fatalf("unexpected traces exporter in secret: %q", got)
	}
	if got := string(secret.Data["GORILLA_TRACER"]); got != "otlp+http://victoria-otlp-gateway:4318" {
		t.Fatalf("unexpected gorilla tracer connection in secret: %q", got)
	}
	if got := string(secret.Data["OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"]); got != "http://victoria-otlp-gateway:4318/v1/metrics" {
		t.Fatalf("unexpected metrics endpoint in secret: %q", got)
	}
	if got := string(secret.Data["OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"]); got != "http://victoria-otlp-gateway:4318/v1/logs" {
		t.Fatalf("unexpected logs endpoint in secret: %q", got)
	}
	if got := string(secret.Data["OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"]); got != "http://victoria-otlp-gateway:4318/v1/traces" {
		t.Fatalf("unexpected traces endpoint in secret: %q", got)
	}
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].Name != wandb.Name {
		t.Fatalf("expected secret to be owned by wandb resource")
	}
}

func TestReconcileTelemetryConnectionSecretUpdateManaged(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding appsv2 to scheme: %v", err)
	}

	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-otel-connection",
			Namespace: "default",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": []byte("http://old.example/metrics"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, secret).Build()

	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Enabled = true
	cfg.Namespace = "wandb"
	cfg.Normalize()

	if err := reconcileTelemetryConnectionSecret(context.Background(), client, wandb, cfg); err != nil {
		t.Fatalf("reconcileTelemetryConnectionSecret returned error: %v", err)
	}

	updated := &corev1.Secret{}
	lookup := types.NamespacedName{Name: cfg.OTel.SecretName, Namespace: wandb.Namespace}
	if err := client.Get(context.Background(), lookup, updated); err != nil {
		t.Fatalf("failed retrieving updated telemetry secret: %v", err)
	}

	if got := string(updated.Data["OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"]); got != "http://victoria-otlp-gateway.wandb.svc:4318/v1/metrics" {
		t.Fatalf("unexpected metrics endpoint: %q", got)
	}
	if got := string(updated.Data["OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"]); got != "http://victoria-otlp-gateway.wandb.svc:4318/v1/logs" {
		t.Fatalf("unexpected logs endpoint: %q", got)
	}
	if got := string(updated.Data["OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"]); got != "http://victoria-otlp-gateway.wandb.svc:4318/v1/traces" {
		t.Fatalf("unexpected traces endpoint: %q", got)
	}
	if got := string(updated.Data["GORILLA_TRACER"]); got != "otlp+http://victoria-otlp-gateway.wandb.svc:4318" {
		t.Fatalf("unexpected gorilla tracer connection in updated secret: %q", got)
	}
}

func TestResolveGorillaTracerConnection(t *testing.T) {
	tests := []struct {
		name           string
		protocol       string
		tracesEndpoint string
		want           string
	}{
		{
			name:           "http protobuf defaults to otlp+http",
			protocol:       "http/protobuf",
			tracesEndpoint: "http://vtsingle-victoria-traces:10428/insert/opentelemetry/v1/traces",
			want:           "otlp+http://vtsingle-victoria-traces:10428",
		},
		{
			name:           "https endpoint maps to otlp+https",
			protocol:       "http/protobuf",
			tracesEndpoint: "https://traces.example.com/v1/traces",
			want:           "otlp+https://traces.example.com",
		},
		{
			name:           "grpc protocol maps to otlp+grpc",
			protocol:       "grpc",
			tracesEndpoint: "http://otel-collector.default.svc:4317",
			want:           "otlp+grpc://otel-collector.default.svc:4317",
		},
		{
			name:           "invalid endpoint falls back to noop",
			protocol:       "http/protobuf",
			tracesEndpoint: "not-a-url",
			want:           "noop://",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveGorillaTracerConnection(tc.protocol, tc.tracesEndpoint)
			if got != tc.want {
				t.Fatalf("unexpected gorilla tracer connection: got %q want %q", got, tc.want)
			}
		})
	}
}

//func TestResolveEnvvarsTelemetrySource(t *testing.T) {
//	scheme := runtime.NewScheme()
//	if err := corev1.AddToScheme(scheme); err != nil {
//		t.Fatalf("failed adding corev1 to scheme: %v", err)
//	}
//	client := fake.NewClientBuilder().WithScheme(scheme).Build()
//
//	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"}}
//	manifest := serverManifest.Manifest{}
//	envs := []serverManifest.EnvVar{
//		{
//			Name: "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
//			Sources: []serverManifest.EnvSource{
//				{Type: "telemetry", Field: "metricsEndpoint"},
//			},
//		},
//		{
//			Name: "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
//			Sources: []serverManifest.EnvSource{
//				{Type: "telemetry", Name: "custom-otel", Field: "logsEndpoint"},
//			},
//		},
//		{
//			Name: "OTEL_TRACES_EXPORTER",
//			Sources: []serverManifest.EnvSource{
//				{Type: "telemetry", Field: "tracesExporter"},
//			},
//		},
//		{
//			Name: "GORILLA_TRACER",
//			Sources: []serverManifest.EnvSource{
//				{Type: "telemetry", Field: "gorillaTracer"},
//			},
//		},
//		{
//			Name: "OTEL_PROTOCOL_AND_SERVICE",
//			Sources: []serverManifest.EnvSource{
//				{Type: "telemetry", Field: "protocol"},
//				{Type: "telemetry", Field: "serviceName"},
//			},
//		},
//	}
//
//	resolved, err := resolveEnvvars(context.Background(), client, wandb, manifest, nil, envs)
//	if err != nil {
//		t.Fatalf("resolveEnvvars returned error: %v", err)
//	}
//
//	metricsEnv := mustFindEnvVar(t, resolved, "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")
//	if metricsEnv.ValueFrom == nil || metricsEnv.ValueFrom.SecretKeyRef == nil {
//		t.Fatalf("expected metrics endpoint to resolve from secret key ref")
//	}
//	if metricsEnv.ValueFrom.SecretKeyRef.Name != "wandb-otel-connection" {
//		t.Fatalf("unexpected metrics secret name: %s", metricsEnv.ValueFrom.SecretKeyRef.Name)
//	}
//	if metricsEnv.ValueFrom.SecretKeyRef.Key != "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT" {
//		t.Fatalf("unexpected metrics key: %s", metricsEnv.ValueFrom.SecretKeyRef.Key)
//	}
//
//	logsEnv := mustFindEnvVar(t, resolved, "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT")
//	if logsEnv.ValueFrom == nil || logsEnv.ValueFrom.SecretKeyRef == nil {
//		t.Fatalf("expected logs endpoint to resolve from secret key ref")
//	}
//	if logsEnv.ValueFrom.SecretKeyRef.Name != "custom-otel" {
//		t.Fatalf("unexpected logs secret name: %s", logsEnv.ValueFrom.SecretKeyRef.Name)
//	}
//	if logsEnv.ValueFrom.SecretKeyRef.Key != "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT" {
//		t.Fatalf("unexpected logs key: %s", logsEnv.ValueFrom.SecretKeyRef.Key)
//	}
//
//	tracesExporter := mustFindEnvVar(t, resolved, "OTEL_TRACES_EXPORTER")
//	if tracesExporter.ValueFrom == nil || tracesExporter.ValueFrom.SecretKeyRef == nil {
//		t.Fatalf("expected traces exporter to resolve from secret key ref")
//	}
//	if tracesExporter.ValueFrom.SecretKeyRef.Name != "wandb-otel-connection" {
//		t.Fatalf("unexpected traces exporter secret name: %s", tracesExporter.ValueFrom.SecretKeyRef.Name)
//	}
//	if tracesExporter.ValueFrom.SecretKeyRef.Key != "OTEL_TRACES_EXPORTER" {
//		t.Fatalf("unexpected traces exporter key: %s", tracesExporter.ValueFrom.SecretKeyRef.Key)
//	}
//
//	gorillaTracer := mustFindEnvVar(t, resolved, "GORILLA_TRACER")
//	if gorillaTracer.ValueFrom == nil || gorillaTracer.ValueFrom.SecretKeyRef == nil {
//		t.Fatalf("expected gorilla tracer to resolve from secret key ref")
//	}
//	if gorillaTracer.ValueFrom.SecretKeyRef.Name != "wandb-otel-connection" {
//		t.Fatalf("unexpected gorilla tracer secret name: %s", gorillaTracer.ValueFrom.SecretKeyRef.Name)
//	}
//	if gorillaTracer.ValueFrom.SecretKeyRef.Key != "GORILLA_TRACER" {
//		t.Fatalf("unexpected gorilla tracer key: %s", gorillaTracer.ValueFrom.SecretKeyRef.Key)
//	}
//
//	protocolComponent := mustFindEnvVar(t, resolved, "OTEL_PROTOCOL_AND_SERVICE_0")
//	if protocolComponent.ValueFrom == nil || protocolComponent.ValueFrom.SecretKeyRef == nil {
//		t.Fatalf("expected protocol component to resolve from secret")
//	}
//	if protocolComponent.ValueFrom.SecretKeyRef.Key != "OTEL_EXPORTER_OTLP_PROTOCOL" {
//		t.Fatalf("unexpected protocol key: %s", protocolComponent.ValueFrom.SecretKeyRef.Key)
//	}
//
//	serviceComponent := mustFindEnvVar(t, resolved, "OTEL_PROTOCOL_AND_SERVICE_1")
//	if serviceComponent.ValueFrom == nil || serviceComponent.ValueFrom.SecretKeyRef == nil {
//		t.Fatalf("expected service component to resolve from secret")
//	}
//	if serviceComponent.ValueFrom.SecretKeyRef.Key != "OTEL_SERVICE_NAME" {
//		t.Fatalf("unexpected service key: %s", serviceComponent.ValueFrom.SecretKeyRef.Key)
//	}
//
//	joined := mustFindEnvVar(t, resolved, "OTEL_PROTOCOL_AND_SERVICE")
//	if joined.Value != "$(OTEL_PROTOCOL_AND_SERVICE_0),$(OTEL_PROTOCOL_AND_SERVICE_1)" {
//		t.Fatalf("unexpected joined telemetry env value: %s", joined.Value)
//	}
//}

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
	if sweepProvider.Value != "http://wandb-dev-v2-anaconda2:8080" {
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
	if historyStore.Value != "http://wandb-dev-v2-parquet:9000/_goRPC_" {
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

	resolved := applyWorkloadTelemetryDefaults(envVars, "wandb-dev-v2-parquet")

	serviceName := mustFindEnvVar(t, resolved, "OTEL_SERVICE_NAME")
	if serviceName.Value != "wandb-dev-v2-parquet" {
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

	resolved := applyWorkloadTelemetryDefaults(envVars, "wandb-dev-v2-parquet")

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
		false,
	)
	if err != nil {
		t.Fatalf("injectManagedWorkloadTelemetryEnvvars returned error: %v", err)
	}
	if len(envVars) != 0 {
		t.Fatalf("expected no managed telemetry envs when telemetry is disabled, got %#v", envVars)
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

//func coreEnvVarSliceContains(envs []corev1.EnvVar, target string) bool {
//	for _, env := range envs {
//		if env.Name == target {
//			return true
//		}
//	}
//	return false
//}
