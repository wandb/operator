package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	if resolved.StatsdAddress != "udp://victoria-otlp-gateway.wandb.svc:8125" {
		t.Fatalf("unexpected statsd address: %s", resolved.StatsdAddress)
	}
	if resolved.DatadogTraceAgentURL != "http://victoria-otlp-gateway.wandb.svc:8126" {
		t.Fatalf("unexpected Datadog trace agent URL: %s", resolved.DatadogTraceAgentURL)
	}
	if resolved.DatadogTraceAgentHost != "victoria-otlp-gateway.wandb.svc" {
		t.Fatalf("unexpected Datadog trace agent host: %s", resolved.DatadogTraceAgentHost)
	}
	if resolved.DatadogTraceAgentPort != "8126" {
		t.Fatalf("unexpected Datadog trace agent port: %s", resolved.DatadogTraceAgentPort)
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

func TestLoadTelemetryRuntimeConfigFromConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-operator-telemetry-config",
			Namespace: "wandb-operator",
		},
		Data: map[string]string{
			"TELEMETRY_ENABLED":                  "true",
			"TELEMETRY_MODE":                     "full",
			"TELEMETRY_MANAGED_NAMESPACE":        "wandb",
			"TELEMETRY_OTEL_SECRET_NAME":         "custom-otel-secret",
			"TELEMETRY_OTEL_PROTOCOL":            "grpc",
			"TELEMETRY_OTEL_SERVICE_NAME":        "custom-service",
			"TELEMETRY_OTEL_RESOURCE_ATTRIBUTES": "deployment.environment=dev",
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	cfg, err := LoadTelemetryRuntimeConfigFromConfigMap(
		context.Background(),
		client,
		types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace},
		DefaultTelemetryRuntimeConfig(),
	)
	if err != nil {
		t.Fatalf("LoadTelemetryRuntimeConfigFromConfigMap returned error: %v", err)
	}

	if !cfg.Enabled {
		t.Fatalf("expected telemetry to be enabled")
	}
	if cfg.Mode != telemetryModeFull {
		t.Fatalf("unexpected telemetry mode: %q", cfg.Mode)
	}
	if cfg.Namespace != "wandb" {
		t.Fatalf("unexpected managed namespace: %q", cfg.Namespace)
	}
	if cfg.OTel.SecretName != "custom-otel-secret" {
		t.Fatalf("unexpected secret name: %q", cfg.OTel.SecretName)
	}
	if cfg.OTel.Protocol != "grpc" {
		t.Fatalf("unexpected protocol: %q", cfg.OTel.Protocol)
	}
	if cfg.OTel.ServiceName != "custom-service" {
		t.Fatalf("unexpected service name: %q", cfg.OTel.ServiceName)
	}
	if cfg.OTel.ResourceAttributes != "deployment.environment=dev" {
		t.Fatalf("unexpected resource attributes: %q", cfg.OTel.ResourceAttributes)
	}
}

func TestLoadTelemetryRuntimeConfigFromConfigMapInvalidBool(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-operator-telemetry-config",
			Namespace: "wandb-operator",
		},
		Data: map[string]string{
			"TELEMETRY_ENABLED": "definitely",
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	_, err := LoadTelemetryRuntimeConfigFromConfigMap(
		context.Background(),
		client,
		types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace},
		DefaultTelemetryRuntimeConfig(),
	)
	if err == nil {
		t.Fatalf("expected invalid telemetry enabled value to return an error")
	}
}

func TestLoadTelemetryRuntimeConfigFromConfigMapMissingSecretNameWhenEnabled(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-operator-telemetry-config",
			Namespace: "wandb-operator",
		},
		Data: map[string]string{
			"TELEMETRY_MODE":              "forward",
			"TELEMETRY_ENABLED":           "true",
			"TELEMETRY_MANAGED_NAMESPACE": "wandb",
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	_, err := LoadTelemetryRuntimeConfigFromConfigMap(
		context.Background(),
		client,
		types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace},
		DefaultTelemetryRuntimeConfig(),
	)
	if err == nil {
		t.Fatalf("expected missing telemetry secret name to return an error")
	}
}

func TestLoadTelemetryRuntimeConfigFromConfigMapMissingReturnsDisabledDefaults(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	defaults := DefaultTelemetryRuntimeConfig()
	cfg, err := LoadTelemetryRuntimeConfigFromConfigMap(
		context.Background(),
		client,
		types.NamespacedName{Name: "wandb-operator-telemetry-config", Namespace: "operator-system"},
		defaults,
	)
	if err != nil {
		t.Fatalf("LoadTelemetryRuntimeConfigFromConfigMap returned error: %v", err)
	}

	if cfg != defaults {
		t.Fatalf("expected missing configmap to return disabled defaults, got %#v", cfg)
	}
}

func TestSummarizeTelemetryInfraStatusForwardReady(t *testing.T) {
	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Mode = telemetryModeForward
	cfg.Enabled = true
	cfg.Namespace = "wandb"
	cfg.OTel.SecretName = "telemetry-secret"
	cfg.Normalize()

	client := fake.NewClientBuilder().WithRuntimeObjects(
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1beta1", Kind: "VMSingle"}, "victoria-instance", "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1beta1", Kind: "VMAgent"}, "victoria-agent", "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1", Kind: "VLSingle"}, "victoria-logs", "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1", Kind: "VTSingle"}, "victoria-traces", "wandb"),
		readyTelemetryResource(appsv1.SchemeGroupVersion.WithKind("Deployment"), telemetryOTLPGatewayName, "wandb"),
	).Build()

	status := summarizeTelemetryInfraStatus(context.Background(), client, cfg)
	if !status.Ready {
		t.Fatalf("expected telemetry infra status to be ready: %#v", status)
	}
	if status.State != telemetryStateReady {
		t.Fatalf("unexpected telemetry state: %q", status.State)
	}
	if status.Mode != telemetryModeForward {
		t.Fatalf("unexpected telemetry mode: %q", status.Mode)
	}
	if status.Connection.ConnectionSecret != "telemetry-secret" {
		t.Fatalf("unexpected connection secret: %q", status.Connection.ConnectionSecret)
	}
	if status.Connection.MetricsEndpoint != "http://victoria-otlp-gateway.wandb.svc:4318/v1/metrics" {
		t.Fatalf("unexpected metrics endpoint: %q", status.Connection.MetricsEndpoint)
	}
	if status.Connection.GorillaTracer != "otlp+http://victoria-otlp-gateway.wandb.svc:4318" {
		t.Fatalf("unexpected gorilla tracer: %q", status.Connection.GorillaTracer)
	}
	if status.Connection.StatsdAddress != "udp://victoria-otlp-gateway.wandb.svc:8125" {
		t.Fatalf("unexpected statsd address: %q", status.Connection.StatsdAddress)
	}
	if status.Connection.DatadogTraceAgentURL != "http://victoria-otlp-gateway.wandb.svc:8126" {
		t.Fatalf("unexpected Datadog trace agent URL: %q", status.Connection.DatadogTraceAgentURL)
	}
	if status.Connection.DatadogTraceAgentHost != "victoria-otlp-gateway.wandb.svc" {
		t.Fatalf("unexpected Datadog trace agent host: %q", status.Connection.DatadogTraceAgentHost)
	}
	if status.Connection.DatadogTraceAgentPort != "8126" {
		t.Fatalf("unexpected Datadog trace agent port: %q", status.Connection.DatadogTraceAgentPort)
	}
}

func TestSummarizeTelemetryInfraStatusFullReady(t *testing.T) {
	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Mode = telemetryModeFull
	cfg.Enabled = true
	cfg.Namespace = "wandb"
	cfg.OTel.SecretName = "telemetry-secret"
	cfg.Normalize()

	client := fake.NewClientBuilder().WithRuntimeObjects(
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1beta1", Kind: "VMSingle"}, "victoria-instance", "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1beta1", Kind: "VMAgent"}, "victoria-agent", "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1", Kind: "VLSingle"}, "victoria-logs", "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1", Kind: "VTSingle"}, "victoria-traces", "wandb"),
		readyTelemetryResource(appsv1.SchemeGroupVersion.WithKind("Deployment"), telemetryOTLPGatewayName, "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "grafana.integreatly.org", Version: "v1beta1", Kind: "Grafana"}, "grafana", "wandb"),
	).Build()

	status := summarizeTelemetryInfraStatus(context.Background(), client, cfg)
	if !status.Ready {
		t.Fatalf("expected full telemetry infra status to be ready: %#v", status)
	}
	if status.Mode != telemetryModeFull {
		t.Fatalf("unexpected telemetry mode: %q", status.Mode)
	}
}

func TestSummarizeTelemetryInfraStatusVictoriaUpdateStatusOperational(t *testing.T) {
	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Mode = telemetryModeForward
	cfg.Enabled = true
	cfg.Namespace = "wandb"
	cfg.OTel.SecretName = "telemetry-secret"
	cfg.Normalize()

	client := fake.NewClientBuilder().WithRuntimeObjects(
		operationalTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1beta1", Kind: "VMSingle"}, "victoria-instance", "wandb"),
		operationalTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1beta1", Kind: "VMAgent"}, "victoria-agent", "wandb"),
		operationalTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1", Kind: "VLSingle"}, "victoria-logs", "wandb"),
		operationalTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1", Kind: "VTSingle"}, "victoria-traces", "wandb"),
		readyTelemetryResource(appsv1.SchemeGroupVersion.WithKind("Deployment"), telemetryOTLPGatewayName, "wandb"),
	).Build()

	status := summarizeTelemetryInfraStatus(context.Background(), client, cfg)
	if !status.Ready {
		t.Fatalf("expected telemetry infra status to be ready for operational Victoria resources: %#v", status)
	}
	if status.State != telemetryStateReady {
		t.Fatalf("unexpected telemetry state: %q", status.State)
	}
}

func TestSummarizeTelemetryInfraStatusDisabled(t *testing.T) {
	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Enabled = false
	cfg.Normalize()

	status := summarizeTelemetryInfraStatus(context.Background(), fake.NewClientBuilder().Build(), cfg)
	if status.Ready {
		t.Fatalf("expected telemetry infra status to be disabled: %#v", status)
	}
	if status.State != telemetryStateDisabled {
		t.Fatalf("unexpected telemetry state: %q", status.State)
	}
	if status.Connection != (apiv2.TelemetryConnectionStatus{}) {
		t.Fatalf("expected disabled telemetry connection status to be empty: %#v", status.Connection)
	}
}

func TestSummarizeTelemetryInfraStatusMissingStackIsPending(t *testing.T) {
	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Mode = telemetryModeForward
	cfg.Enabled = true
	cfg.Namespace = "wandb"
	cfg.OTel.SecretName = "telemetry-secret"
	cfg.Normalize()

	status := summarizeTelemetryInfraStatus(context.Background(), fake.NewClientBuilder().Build(), cfg)
	if status.Ready {
		t.Fatalf("expected telemetry infra status to be pending when stack is missing: %#v", status)
	}
	if status.State != telemetryStatePending {
		t.Fatalf("unexpected telemetry state: %q", status.State)
	}
}

func TestSummarizeTelemetryInfraStatusDegradedIsNotReady(t *testing.T) {
	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Mode = telemetryModeForward
	cfg.Enabled = true
	cfg.Namespace = "wandb"
	cfg.OTel.SecretName = "telemetry-secret"
	cfg.Normalize()

	client := fake.NewClientBuilder().WithRuntimeObjects(
		newTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1beta1", Kind: "VMSingle"}, "victoria-instance", "wandb", []map[string]any{{"type": "Degraded", "status": "True"}}),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1beta1", Kind: "VMAgent"}, "victoria-agent", "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1", Kind: "VLSingle"}, "victoria-logs", "wandb"),
		readyTelemetryResource(schema.GroupVersionKind{Group: "operator.victoriametrics.com", Version: "v1", Kind: "VTSingle"}, "victoria-traces", "wandb"),
		readyTelemetryResource(appsv1.SchemeGroupVersion.WithKind("Deployment"), telemetryOTLPGatewayName, "wandb"),
	).Build()

	status := summarizeTelemetryInfraStatus(context.Background(), client, cfg)
	if status.Ready {
		t.Fatalf("expected degraded telemetry infra status to be not ready: %#v", status)
	}
	if status.State != telemetryStateNotReady {
		t.Fatalf("unexpected telemetry state: %q", status.State)
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
	cfg.OTel.SecretName = "wandb-otel-connection"
	cfg.Normalize()
	wandb.Status.TelemetryStatus = summarizeTelemetryInfraStatus(context.Background(), client, cfg)

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
	if got := string(secret.Data["GORILLA_STATSD_ADDRESS"]); got != "udp://victoria-otlp-gateway:8125" {
		t.Fatalf("unexpected gorilla statsd address in secret: %q", got)
	}
	if got := string(secret.Data["DD_TRACE_AGENT_URL"]); got != "http://victoria-otlp-gateway:8126" {
		t.Fatalf("unexpected Datadog trace agent URL in secret: %q", got)
	}
	if got := string(secret.Data["DD_AGENT_HOST"]); got != "victoria-otlp-gateway" {
		t.Fatalf("unexpected Datadog agent host in secret: %q", got)
	}
	if got := string(secret.Data["DD_TRACE_AGENT_PORT"]); got != "8126" {
		t.Fatalf("unexpected Datadog trace agent port in secret: %q", got)
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

func TestReconcileTelemetryConnectionSecretDisabledSkipsCreate(t *testing.T) {
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
	cfg.Enabled = false
	cfg.OTel.SecretName = "wandb-otel-connection"
	cfg.Normalize()

	if err := reconcileTelemetryConnectionSecret(context.Background(), client, wandb, cfg); err != nil {
		t.Fatalf("reconcileTelemetryConnectionSecret returned error: %v", err)
	}

	secret := &corev1.Secret{}
	lookup := types.NamespacedName{Name: cfg.OTel.SecretName, Namespace: wandb.Namespace}
	if err := client.Get(context.Background(), lookup, secret); !apierrors.IsNotFound(err) {
		t.Fatalf("expected telemetry secret not to be created, got error %v", err)
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
	cfg.OTel.SecretName = "wandb-otel-connection"
	cfg.Normalize()
	wandb.Status.TelemetryStatus = summarizeTelemetryInfraStatus(context.Background(), client, cfg)

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
	if got := string(updated.Data["GORILLA_STATSD_ADDRESS"]); got != "udp://victoria-otlp-gateway.wandb.svc:8125" {
		t.Fatalf("unexpected gorilla statsd address in updated secret: %q", got)
	}
	if got := string(updated.Data["DD_TRACE_AGENT_URL"]); got != "http://victoria-otlp-gateway.wandb.svc:8126" {
		t.Fatalf("unexpected Datadog trace agent URL in updated secret: %q", got)
	}
	if got := string(updated.Data["DD_AGENT_HOST"]); got != "victoria-otlp-gateway.wandb.svc" {
		t.Fatalf("unexpected Datadog agent host in updated secret: %q", got)
	}
	if got := string(updated.Data["DD_TRACE_AGENT_PORT"]); got != "8126" {
		t.Fatalf("unexpected Datadog trace agent port in updated secret: %q", got)
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
		TelemetryRuntimeConfig{Enabled: false},
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
		TelemetryRuntimeConfig{Enabled: true},
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

	telemetryConfig := TelemetryRuntimeConfig{
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
				TelemetryRuntimeConfig{Enabled: true},
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

func readyTelemetryResource(gvk schema.GroupVersionKind, name, namespace string) *unstructured.Unstructured {
	return newTelemetryResource(gvk, name, namespace, []map[string]any{{"type": "Available", "status": "True"}})
}

func operationalTelemetryResource(gvk schema.GroupVersionKind, name, namespace string) *unstructured.Unstructured {
	obj := newTelemetryResource(gvk, name, namespace, nil)
	obj.Object["status"] = map[string]any{
		"updateStatus": "operational",
	}
	return obj
}

func newTelemetryResource(
	gvk schema.GroupVersionKind,
	name string,
	namespace string,
	conditions []map[string]any,
) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": gvk.GroupVersion().String(),
			"kind":       gvk.Kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
	obj.SetGroupVersionKind(gvk)
	if len(conditions) > 0 {
		rawConditions := make([]any, 0, len(conditions))
		for _, condition := range conditions {
			rawConditions = append(rawConditions, condition)
		}
		obj.Object["status"] = map[string]any{
			"conditions": rawConditions,
		}
	}
	return obj
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
