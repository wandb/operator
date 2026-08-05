package reconciler

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
	mustContain(t, output, "statsd/dogstatsd:")
	mustContain(t, output, "endpoint: 0.0.0.0:8125")
	mustContain(t, output, "datadog:")
	mustContain(t, output, "endpoint: 0.0.0.0:8126")
	mustContain(t, output, "receivers: [otlp, statsd/dogstatsd, spanmetrics]")
	mustContain(t, output, "receivers: [otlp, datadog]")
	mustContain(t, output, "name: statsd")
	mustContain(t, output, "port: 8125")
	mustContain(t, output, "protocol: UDP")
	mustContain(t, output, "name: datadog-trace")
	mustContain(t, output, "port: 8126")
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
	mustContain(t, output, "receivers: [otlp, statsd/dogstatsd, spanmetrics]")
	mustContain(t, output, "exporters: [otlphttp/vm, otlphttp/external]")
	mustContain(t, output, "exporters: [otlphttp/vl, otlphttp/external]")
	mustContain(t, output, "receivers: [otlp, datadog]")
	mustContain(t, output, "exporters: [otlphttp/vt, spanmetrics, otlphttp/external]")
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
	mustNotContain(t, output, "name: wandb-operator-telemetry-config")
}

func TestTelemetryChartRendersRuntimeConfigWithoutOperatorEnvPassthrough(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb.install=false",
		"--set", "telemetry.mode=full",
		"--set", "victoria-metrics-operator.enabled=true",
		"--set", "grafana-operator.enabled=true",
	)

	mustContain(t, output, "name: wandb-operator-telemetry-config")
	mustContain(t, output, "TELEMETRY_MODE: \"full\"")
	mustContain(t, output, "TELEMETRY_OTEL_SECRET_NAME: \"wandb-otel-connection\"")
	mustNotContain(t, output, "key: TELEMETRY_ENABLED")
	mustNotContain(t, output, "key: TELEMETRY_MANAGED_NAMESPACE")
	mustNotContain(t, output, "key: TELEMETRY_OTEL_SECRET_NAME")
	mustNotContain(t, output, "key: TELEMETRY_OTEL_PROTOCOL")
	mustNotContain(t, output, "key: TELEMETRY_OTEL_SERVICE_NAME")
	mustNotContain(t, output, "key: TELEMETRY_OTEL_RESOURCE_ATTRIBUTES")
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
	mustContain(t, output, "statsd/dogstatsd:")
	mustContain(t, output, "datadog:")
}

// The crd-installer Job installs VM + Grafana CRDs in every mode so an off→full
// upgrade never has to add CRDs (which Helm won't do on upgrade).
func TestCrdInstallerAlwaysRequestsTelemetryGroups(t *testing.T) {
	modes := []struct {
		name  string
		extra []string
	}{
		{"off", []string{"--set", "telemetry.mode=off"}},
		{"forward", []string{
			"--set", "telemetry.mode=forward",
			"--set", "telemetry.forwarding.otlp.endpoint=https://otel.example.com",
			"--set", "victoria-metrics-operator.enabled=true",
		}},
		{"full", []string{
			"--set", "telemetry.mode=full",
			"--set", "victoria-metrics-operator.enabled=true",
			"--set", "grafana-operator.enabled=true",
		}},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			args := append([]string{"--set", "helmHooks.enabled=true", "--set", "wandb.install=false"}, m.extra...)
			output := runHelmTemplate(t, args...)
			groups := crdInstallerGroups(t, output)
			for _, g := range []string{"victoriametrics", "grafana"} {
				if !strings.Contains(groups, g) {
					t.Errorf("crd-installer --groups=%q missing %q", groups, g)
				}
			}
		})
	}
}

// crdInstallerGroups returns the value of the crd-installer Job's --groups flag.
func crdInstallerGroups(t *testing.T, output string) string {
	t.Helper()
	const marker = "--groups="
	for _, line := range strings.Split(output, "\n") {
		if i := strings.Index(line, marker); i >= 0 {
			return strings.TrimSpace(line[i+len(marker):])
		}
	}
	t.Fatalf("crd-installer --groups= flag not found in rendered output")
	return ""
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

	args := []string{"template", "telemetry-test", chartPath, "-n", "wandb-operator", "--set", "helmHooks.enabled=false"}
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
