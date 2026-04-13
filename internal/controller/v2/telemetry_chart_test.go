package v2

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTelemetryChartFullModeRendersCoreStack(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.mode=full",
		"--set", "victoria-metrics-operator.enabled=true",
		"--set", "grafana-operator.enabled=true",
	)

	mustContain(t, output, "kind: VMSingle")
	mustContain(t, output, "kind: VMAgent")
	mustContain(t, output, "kind: VLSingle")
	mustContain(t, output, "kind: VTSingle")
	mustContain(t, output, "name: victoria-otlp-gateway-config")
	mustContain(t, output, "name: victoria-otlp-gateway")
	mustContain(t, output, "kind: Grafana")
	mustContain(t, output, "kind: GrafanaDatasource")
	mustContain(t, output, "url: \"http://vmsingle-victoria-instance:8428\"")
	mustContain(t, output, "url: \"http://vlsingle-victoria-logs:9428\"")
	mustContain(t, output, "url: \"http://vtsingle-victoria-traces:10428/select/jaeger\"")
	mustContain(t, output, "inputName: DS_VICTORIATRACES")
	mustContain(t, output, "datasourceName: VictoriaTraces")
	mustContain(t, output, "\"title\": \"Open Traces in Explore\"")
	mustContain(t, output, "\"title\": \"Trace Coverage\"")
	mustContain(t, output, "\"title\": \"Metadata Operation Call Rate\"")
	mustContain(t, output, "\"title\": \"Redis Pool Usage by Service\"")
	mustContain(t, output, "gorilla-executor")
	mustContain(t, output, "\"title\": \"MySQL Exporter Up\"")
	mustNotContain(t, output, "name: vmui")
	mustNotContain(t, output, "name: perses")
	mustContain(t, output, "retentionPeriod: \"1d\"")
}

func TestTelemetryChartForwardModeSkipsGrafanaButAddsForwarding(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.mode=forward",
		"--set", "telemetry.forwarding.otlp.endpoint=https://otel.example.com",
		"--set", "victoria-metrics-operator.enabled=true",
	)

	mustContain(t, output, "kind: VMSingle")
	mustContain(t, output, "kind: VMAgent")
	mustContain(t, output, "kind: VLSingle")
	mustContain(t, output, "kind: VTSingle")
	mustContain(t, output, "name: victoria-otlp-gateway-config")
	mustContain(t, output, "name: victoria-otlp-gateway")
	mustContain(t, output, "endpoint: \"https://otel.example.com\"")
	mustNotContain(t, output, "kind: Grafana")
}

func TestTelemetryChartOffModeSkipsManagedStack(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.mode=off",
	)

	mustNotContain(t, output, "kind: VMSingle")
	mustNotContain(t, output, "kind: VMAgent")
	mustNotContain(t, output, "kind: VLSingle")
	mustNotContain(t, output, "kind: VTSingle")
	mustNotContain(t, output, "name: victoria-otlp-gateway-config")
	mustNotContain(t, output, "name: victoria-otlp-gateway")
	mustNotContain(t, output, "kind: Grafana")
}

func TestStandaloneTelemetryChartFullModeRendersCoreStack(t *testing.T) {
	output := runHelmTemplateForChart(t, filepath.Join("..", "..", "..", "deploy", "telemetry"),
		"--set", "mode=full",
		"--set", "namespace=wandb",
	)

	mustContain(t, output, "kind: VMSingle")
	mustContain(t, output, "kind: VMAgent")
	mustContain(t, output, "kind: Grafana")
	mustContain(t, output, "url: \"http://vmsingle-victoria-instance:8428\"")
}

func TestOperatorChartFullModeRendersTelemetryConfigMapWithoutTelemetryEnvs(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb.install=false",
		"--set", "telemetry.mode=full",
		"--set", "telemetry.otel.secretName=wandb-otel-connection",
		"--set", "victoria-metrics-operator.enabled=true",
		"--set", "grafana-operator.enabled=true",
	)

	mustContain(t, output, "name: wandb-operator-telemetry-config")
	mustContain(t, output, "TELEMETRY_MODE: \"full\"")
	mustNotContain(t, output, "name: TELEMETRY_ENABLED")
	mustNotContain(t, output, "name: TELEMETRY_MANAGED_NAMESPACE")
}

func TestTelemetryOnlyOperatorChartRendersConfigAndStackWithoutOperatorRBAC(t *testing.T) {
	output, err := runHelmTemplateWithError(t, filepath.Join("..", "..", "..", "deploy", "operator"),
		"--set", "wandb-operator.enabled=false",
		"--set", "wandb.install=false",
		"--set", "mysql-operator.enabled=false",
		"--set", "redis-operator.enabled=false",
		"--set", "strimzi-kafka-operator.enabled=false",
		"--set", "minio-operator.enabled=false",
		"--set", "altinity-clickhouse-operator.enabled=false",
		"--set", "victoria-metrics-operator.enabled=false",
		"--set", "grafana-operator.enabled=false",
		"--set", "telemetry.mode=full",
		"--set", "telemetry.otel.secretName=wandb-otel-connection",
		"--set", "telemetry.namespace=default",
		"--set", "telemetry.operatorNamespace=operator-system",
	)
	if err != nil {
		t.Fatalf("helm template failed: %v\noutput:\n%s", err, output)
	}

	mustContain(t, output, "name: wandb-operator-telemetry-config")
	mustContain(t, output, "namespace: operator-system")
	mustContain(t, output, "TELEMETRY_MODE: \"full\"")
	mustContain(t, output, "kind: VMSingle")
	mustContain(t, output, "kind: Grafana")
	mustNotContain(t, output, "name: telemetry-test-application")
	mustNotContain(t, output, "name: telemetry-test-vm")
	mustNotContain(t, output, "name: telemetry-test-grafana")
}

func TestOperatorChartFullModeRequiresTelemetrySecretName(t *testing.T) {
	output, err := runHelmTemplateWithError(t, filepath.Join("..", "..", "..", "deploy", "operator"),
		"--set", "wandb.install=false",
		"--set", "telemetry.mode=full",
		"--set", "victoria-metrics-operator.enabled=true",
		"--set", "grafana-operator.enabled=true",
	)
	if err == nil {
		t.Fatalf("expected helm template to fail when telemetry secret name is missing")
	}
	mustContain(t, output, "telemetry.mode=forward/full requires telemetry.otel.secretName")
}

func TestOperatorChartOffModeSkipsTelemetryConfigMap(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb.install=false",
		"--set", "telemetry.mode=off",
	)

	mustNotContain(t, output, "name: wandb-operator-telemetry-config")
	mustNotContain(t, output, "name: TELEMETRY_ENABLED")
}

func runHelmTemplate(t *testing.T, extraArgs ...string) string {
	t.Helper()
	output, err := runHelmTemplateWithError(t, filepath.Join("..", "..", "..", "deploy", "operator"), extraArgs...)
	if err != nil {
		t.Fatalf("helm template failed: %v\noutput:\n%s", err, output)
	}
	return output
}

func runHelmTemplateForChart(t *testing.T, chartPath string, extraArgs ...string) string {
	t.Helper()
	output, err := runHelmTemplateWithError(t, chartPath, extraArgs...)
	if err != nil {
		t.Fatalf("helm template failed: %v\noutput:\n%s", err, output)
	}
	return output
}

func runHelmTemplateWithError(t *testing.T, chartPath string, extraArgs ...string) (string, error) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skipf("helm binary not found: %v", err)
		return "", nil
	}

	args := []string{"template", "telemetry-test", chartPath, "-n", "wandb-operator"}
	args = append(args, extraArgs...)

	cmd := exec.Command("helm", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func mustContain(t *testing.T, output, value string) {
	t.Helper()
	if !strings.Contains(output, value) {
		t.Fatalf("expected output to contain %q", value)
	}
}

func mustNotContain(t *testing.T, output, value string) {
	t.Helper()
	if strings.Contains(output, value) {
		t.Fatalf("expected output not to contain %q", value)
	}
}
