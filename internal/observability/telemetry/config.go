package telemetry

import (
	"fmt"
	"strings"
)

const (
	telemetryOTLPGatewayName     = "victoria-otlp-gateway"
	telemetryOTLPGatewayHTTPPort = 4318
	telemetryStatsdPort          = 8125
	telemetryDatadogTracePort    = 8126
	telemetryModeOff             = "off"
	telemetryModeForward         = "forward"
	telemetryModeFull            = "full"

	telemetryMetricsReadEndpoint = "metricsReadEndpoint"
	telemetryLogsReadEndpoint    = "logsReadEndpoint"
	telemetryTracesReadEndpoint  = "tracesReadEndpoint"

	telemetryMetricsReadName = "vmsingle-victoria-instance"
	telemetryMetricsReadPort = 8428
	telemetryLogsReadName    = "vlsingle-victoria-logs"
	telemetryLogsReadPort    = 9428
	telemetryTracesReadName  = "vtsingle-victoria-traces"
	telemetryTracesReadPort  = 10428
)

type TelemetryEndpoints struct {
	MetricsEndpoint       string
	MetricsReadEndpoint   string
	LogsEndpoint          string
	LogsReadEndpoint      string
	TracesEndpoint        string
	TracesReadEndpoint    string
	StatsdAddress         string
	DatadogTraceAgentURL  string
	DatadogTraceAgentHost string
	DatadogTraceAgentPort string
}

type TelemetryOTelConfig struct {
	SecretName         string
	Protocol           string
	ServiceName        string
	ResourceAttributes string
}

type TelemetryRuntimeConfig struct {
	Enabled   bool
	Mode      string
	Namespace string
	OTel      TelemetryOTelConfig
}

func DefaultTelemetryRuntimeConfig() TelemetryRuntimeConfig {
	return TelemetryRuntimeConfig{
		Enabled:   false,
		Mode:      telemetryModeOff,
		Namespace: "",
		OTel: TelemetryOTelConfig{
			Protocol:    "http/protobuf",
			ServiceName: "wandb-service",
		},
	}
}

func (cfg *TelemetryRuntimeConfig) Normalize() {
	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	cfg.Namespace = strings.TrimSpace(cfg.Namespace)

	cfg.OTel.SecretName = strings.TrimSpace(cfg.OTel.SecretName)
	cfg.OTel.Protocol = strings.TrimSpace(cfg.OTel.Protocol)
	cfg.OTel.ServiceName = strings.TrimSpace(cfg.OTel.ServiceName)
	cfg.OTel.ResourceAttributes = strings.TrimSpace(cfg.OTel.ResourceAttributes)
	if cfg.OTel.Protocol == "" {
		cfg.OTel.Protocol = "http/protobuf"
	}
	if cfg.OTel.ServiceName == "" {
		cfg.OTel.ServiceName = "wandb-service"
	}
	switch cfg.Mode {
	case "":
		if cfg.Enabled {
			cfg.Mode = telemetryModeForward
		} else {
			cfg.Mode = telemetryModeOff
		}
	case telemetryModeOff:
		if cfg.Enabled {
			cfg.Mode = telemetryModeForward
		} else {
			cfg.Enabled = false
		}
	default:
		cfg.Enabled = true
	}
}

func (cfg TelemetryRuntimeConfig) Validate() error {
	switch cfg.Mode {
	case telemetryModeOff, telemetryModeForward, telemetryModeFull:
	default:
		return fmt.Errorf("telemetry mode must be one of %q, %q, or %q", telemetryModeOff, telemetryModeForward, telemetryModeFull)
	}
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.OTel.SecretName) == "" {
		return fmt.Errorf("telemetry OTel secret name must not be empty")
	}
	return nil
}

func (cfg TelemetryRuntimeConfig) ResolveEndpoints() TelemetryEndpoints {
	if !cfg.Enabled {
		return TelemetryEndpoints{}
	}

	namespace := cfg.Namespace
	host := resolveServiceHost(telemetryOTLPGatewayName, namespace)
	baseURL := fmt.Sprintf("http://%s:%d", host, telemetryOTLPGatewayHTTPPort)
	return TelemetryEndpoints{
		MetricsEndpoint:       fmt.Sprintf("%s/v1/metrics", baseURL),
		MetricsReadEndpoint:   resolveReadEndpoint(telemetryMetricsReadName, namespace, telemetryMetricsReadPort),
		LogsEndpoint:          fmt.Sprintf("%s/v1/logs", baseURL),
		LogsReadEndpoint:      resolveReadEndpoint(telemetryLogsReadName, namespace, telemetryLogsReadPort),
		TracesEndpoint:        fmt.Sprintf("%s/v1/traces", baseURL),
		TracesReadEndpoint:    resolveReadEndpoint(telemetryTracesReadName, namespace, telemetryTracesReadPort),
		StatsdAddress:         fmt.Sprintf("udp://%s:%d", host, telemetryStatsdPort),
		DatadogTraceAgentURL:  fmt.Sprintf("http://%s:%d", host, telemetryDatadogTracePort),
		DatadogTraceAgentHost: host,
		DatadogTraceAgentPort: fmt.Sprintf("%d", telemetryDatadogTracePort),
	}
}

func resolveServiceHost(name, namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		return name
	}
	return fmt.Sprintf("%s.%s.svc", name, namespace)
}

func resolveReadEndpoint(readName string, namespace string, port int) string {
	host := resolveServiceHost(readName, namespace)
	return fmt.Sprintf("http://%s:%d", host, port)
}
