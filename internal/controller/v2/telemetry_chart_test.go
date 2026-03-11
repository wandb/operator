package v2

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTelemetryChartManagedModeRendersCoreStack(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.enabled=true",
		"--set", "telemetry.mode=managed",
	)

	mustContain(t, output, "kind: VMSingle")
	mustContain(t, output, "kind: VMAgent")
	mustContain(t, output, "kind: VLSingle")
	mustContain(t, output, "kind: VTSingle")
	mustContain(t, output, "name: victoria-otlp-gateway-config")
	mustContain(t, output, "name: victoria-otlp-gateway")
}

func TestTelemetryChartExternalModeSkipsManagedStack(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.enabled=true",
		"--set", "telemetry.mode=external",
		"--set", "telemetry.external.metricsEndpoint=https://metrics.example.com/v1/metrics",
		"--set", "telemetry.external.logsEndpoint=https://logs.example.com/v1/logs",
		"--set", "telemetry.external.tracesEndpoint=https://traces.example.com/v1/traces",
	)

	mustNotContain(t, output, "kind: VMSingle")
	mustNotContain(t, output, "kind: VMAgent")
	mustNotContain(t, output, "kind: VLSingle")
	mustNotContain(t, output, "kind: VTSingle")
	mustNotContain(t, output, "name: victoria-otlp-gateway-config")
	mustNotContain(t, output, "name: victoria-otlp-gateway")
}

func TestTelemetryChartUIToggles(t *testing.T) {
	defaultOutput := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.enabled=true",
		"--set", "telemetry.mode=managed",
	)
	mustNotContain(t, defaultOutput, "kind: Grafana")
	mustNotContain(t, defaultOutput, "name: vmui")

	toggledOutput := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.enabled=true",
		"--set", "telemetry.mode=managed",
		"--set", "telemetry.ui.grafana.enabled=true",
		"--set", "telemetry.ui.vmui.enabled=true",
	)
	mustContain(t, toggledOutput, "kind: Grafana")
	mustContain(t, toggledOutput, "kind: GrafanaDatasource")
	mustContain(t, toggledOutput, "name: vmui")
}

func TestTelemetryChartExternalModeSchemaGuard(t *testing.T) {
	_, err := runHelmTemplateWithError(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.enabled=true",
		"--set", "telemetry.mode=external",
	)
	if err == nil {
		t.Fatalf("expected helm template to fail when external telemetry endpoints are missing")
	}
}

func runHelmTemplate(t *testing.T, extraArgs ...string) string {
	t.Helper()
	output, err := runHelmTemplateWithError(t, extraArgs...)
	if err != nil {
		t.Fatalf("helm template failed: %v\noutput:\n%s", err, output)
	}
	return output
}

func runHelmTemplateWithError(t *testing.T, extraArgs ...string) (string, error) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skipf("helm binary not found: %v", err)
		return "", nil
	}

	chartPath := filepath.Join("..", "..", "..", "deploy", "operator")
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
