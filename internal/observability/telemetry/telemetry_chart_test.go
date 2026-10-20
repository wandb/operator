package telemetry

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestTelemetryChartFullModeRendersCoreStack(t *testing.T) {
	output := runHelmTemplate(t,
		"--set", "wandb-operator.enabled=false",
		"--set", "telemetry.mode=full",
		"--set", "telemetry.grafana.pluginImage=wandb/operator:test",
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
	mustContain(t, output, "name: grafana-plugins")
	mustContain(t, output, `image: "wandb/operator:test"`)
	mustContain(t, output, `command: ["cp", "-R", "/opt/wandb/grafana-plugins/.", "/plugins/"]`)
	mustNotContain(t, output, "GF_INSTALL_PLUGINS")
	mustContain(t, output, "url: \"http://vmsingle-victoria-instance:8428\"")
	mustContain(t, output, "url: \"http://vlsingle-victoria-logs:9428\"")
	mustContain(t, output, "url: \"http://vtsingle-victoria-traces:10428/select/tempo\"")
	mustContain(t, output, "type: tempo")
	mustNotContain(t, output, "type: jaeger")
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
	mustContain(t, output, "name: victoria-log-agent")
	mustContain(t, output, "filelog:")
	mustContain(t, output, "storage: file_storage")
	mustContain(t, output, "type: regex_parser")
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
	mustNotContain(t, output, "name: victoria-log-agent")
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

func TestTelemetryChartUsesPrivateRegistry(t *testing.T) {
	output := runHelmTemplate(t,
		"--values", filepath.Join(operatorChartPath, "profiles", "telemetry-full.yaml"),
		"--set", "global.image.registry=mirror.example:5000",
		"--set", "global.imagePullSecrets[0].name=mirror-pull",
		"--set", "grafana-operator.global.imageRegistry=mirror.example:5000",
		"--set", "grafana-operator.imagePullSecrets[0].name=mirror-pull",
		"--set", "wandb-operator.image.repository=mirror.example:5000/wandb/operator",
	)

	for _, value := range []string{
		`image: "mirror.example:5000/otel/opentelemetry-collector-contrib:0.102.1"`,
		"repository: mirror.example:5000/victoriametrics/victoria-metrics",
		`configReloaderImage: "mirror.example:5000/victoriametrics/operator:config-reloader-v0.73.1"`,
		`version: "mirror.example:5000/grafana/grafana:12.3.0"`,
		"mirror.example:5000/victoriametrics/operator:v0.73.1",
		"mirror.example:5000/grafana/grafana-operator:v5.21.4",
		`image: "mirror.example:5000/wandb/operator:2.0.0-beta.8"`,
		"- name: mirror-pull",
	} {
		mustContain(t, output, value)
	}
}

func TestGrafanaPluginImageDefaultsToOperatorImage(t *testing.T) {
	full := []string{"--values", filepath.Join(operatorChartPath, "profiles", "telemetry-full.yaml")}
	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"chart default": {want: "us-docker.pkg.dev/wandb-production/public/wandb/operator:2.0.0-beta.8"},
		"wsm mirror":    {args: []string{"--set", "wandb-operator.image.repository=mirror.example:5000/wandb/operator"}, want: "mirror.example:5000/wandb/operator:2.0.0-beta.8"},
		"override":      {args: []string{"--set", "telemetry.grafana.pluginImage=registry.example/wandb/operator:1.2.3"}, want: "registry.example/wandb/operator:1.2.3"},
	} {
		t.Run(name, func(t *testing.T) {
			output := runHelmTemplate(t, append(full, tc.args...)...)
			if count := strings.Count(output, "\nkind: Grafana\n"); count != 1 {
				t.Fatalf("rendered %d Grafana resources, want 1", count)
			}
			mustContain(t, output, "- name: grafana-plugins\n              image: \""+tc.want+"\"")
		})
	}
}

func TestTelemetryImagePinsMatchOperatorCharts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(operatorChartPath, "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var chart struct {
		Dependencies []struct{ Name, Version string }
	}
	if err := yaml.Unmarshal(data, &chart); err != nil {
		t.Fatal(err)
	}
	pinned := map[string]string{
		"victoria-metrics-operator": "0.66.3",
		"grafana-operator":          "5.21.4",
	}
	for _, dependency := range chart.Dependencies {
		want, ok := pinned[dependency.Name]
		if !ok {
			continue
		}
		delete(pinned, dependency.Name)
		if dependency.Version != want {
			t.Errorf("%s chart is %s, telemetry pins were chosen for %s: update the image tags in deploy/telemetry/templates (_helpers.tpl, telemetry-victoria-core.yaml, telemetry-alerting.yaml), then this test",
				dependency.Name, dependency.Version, want)
		}
	}
	if len(pinned) > 0 {
		t.Errorf("operator chart no longer depends on %v", pinned)
	}
}

func TestTelemetryChartRejectsStringPullSecrets(t *testing.T) {
	output, err := runHelmTemplateWithError(t, operatorChartPath,
		"--set", "telemetry.mode=forward",
		"--set", "telemetry.forwarding.otlp.endpoint=https://otel.example.com",
		"--set", "victoria-metrics-operator.enabled=true",
		"--set", "global.imagePullSecrets[0]=mirror-pull",
	)
	if err == nil {
		t.Fatalf("expected helm template to fail")
	}
	mustContain(t, output, "global.imagePullSecrets entry to be a {name: <secret>} object")
}

func TestStandaloneTelemetryChartFullModeRendersCoreStack(t *testing.T) {
	output := runHelmTemplateForChart(t, filepath.Join("..", "..", "..", "deploy", "telemetry"),
		"--set", "mode=full",
		"--set", "namespace=wandb",
		"--set", "grafana.pluginImage=wandb/operator:test",
	)

	mustContain(t, output, "kind: VMSingle")
	mustContain(t, output, "kind: VMAgent")
	mustContain(t, output, "kind: Grafana")
	mustContain(t, output, "url: \"http://vmsingle-victoria-instance:8428\"")
	mustContain(t, output, "statsd/dogstatsd:")
	mustContain(t, output, "datadog:")
}

func TestTelemetryReadEndpointsMatchChartServices(t *testing.T) {
	output := runHelmTemplateForChart(t, filepath.Join("..", "..", "..", "deploy", "telemetry"),
		"--set", "mode=full",
		"--set", "grafana.pluginImage=wandb/operator:test",
	)

	cfg := DefaultTelemetryRuntimeConfig()
	cfg.Enabled = true
	cfg.Normalize()
	resolved := cfg.ResolveEndpoints()

	for _, endpoint := range []string{
		resolved.MetricsReadEndpoint,
		resolved.LogsReadEndpoint,
		resolved.TracesReadEndpoint,
	} {
		mustContain(t, output, endpoint)
	}
}

func TestCrdInstallerTelemetryGroupsAreOptIn(t *testing.T) {
	baseArgs := []string{"--set", "helmHooks.enabled=true", "--set", "wandb.install=false"}

	offGroups := crdInstallerGroups(t, runHelmTemplate(t, baseArgs...))
	for _, group := range []string{"victoriametrics", "grafana"} {
		if hasGroup(offGroups, group) {
			t.Errorf("crd-installer --groups=%q unexpectedly includes %q", offGroups, group)
		}
	}

	forwardArgs := append(baseArgs, "--set", "telemetry.crds.victoriaMetrics=true")
	forwardGroups := crdInstallerGroups(t, runHelmTemplate(t, forwardArgs...))
	if !hasGroup(forwardGroups, "victoriametrics") {
		t.Errorf("crd-installer --groups=%q missing %q", forwardGroups, "victoriametrics")
	}
	if hasGroup(forwardGroups, "grafana") {
		t.Errorf("crd-installer --groups=%q unexpectedly includes %q", forwardGroups, "grafana")
	}

	fullArgs := append(baseArgs,
		"--set", "telemetry.crds.victoriaMetrics=true",
		"--set", "telemetry.crds.grafana=true",
	)
	fullGroups := crdInstallerGroups(t, runHelmTemplate(t, fullArgs...))
	for _, group := range []string{"victoriametrics", "grafana"} {
		if !hasGroup(fullGroups, group) {
			t.Errorf("crd-installer --groups=%q missing %q", fullGroups, group)
		}
	}
}

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

func hasGroup(groups, target string) bool {
	for _, group := range strings.Split(groups, ",") {
		if strings.TrimSpace(group) == target {
			return true
		}
	}
	return false
}

var operatorChartPath = filepath.Join("..", "..", "..", "deploy", "operator")

func runHelmTemplate(t *testing.T, extraArgs ...string) string {
	t.Helper()
	output, err := runHelmTemplateWithError(t, operatorChartPath, extraArgs...)
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
