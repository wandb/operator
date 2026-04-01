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

func TestTelemetryChartDisabledSkipsManagedStack(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.enabled=false",
	)

	mustNotContain(t, output, "kind: VMSingle")
	mustNotContain(t, output, "kind: VMAgent")
	mustNotContain(t, output, "kind: VLSingle")
	mustNotContain(t, output, "kind: VTSingle")
	mustNotContain(t, output, "name: victoria-otlp-gateway-config")
	mustNotContain(t, output, "name: victoria-otlp-gateway")
	mustNotContain(t, output, "kind: Grafana")
}

func TestStandaloneTelemetryChartManagedModeRendersCoreStack(t *testing.T) {
	output := runHelmTemplateForChart(t, filepath.Join("..", "..", "..", "deploy", "telemetry"),
		"--set", "enabled=true",
		"--set", "namespace=wandb",
	)

	mustContain(t, output, "kind: VMSingle")
	mustContain(t, output, "kind: VMAgent")
	mustContain(t, output, "kind: Grafana")
	mustContain(t, output, "url: \"http://vmsingle-victoria-instance:8428\"")
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
