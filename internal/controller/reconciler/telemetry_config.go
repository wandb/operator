package reconciler

import (
	"fmt"
	"strings"
)

const (
	telemetryOTLPGatewayName     = "victoria-otlp-gateway"
	telemetryOTLPGatewayHTTPPort = 4318
)

type TelemetryEndpoints struct {
	MetricsEndpoint string
	LogsEndpoint    string
	TracesEndpoint  string
}

type TelemetryOTelConfig struct {
	SecretName         string
	Protocol           string
	ServiceName        string
	ResourceAttributes string
}

type TelemetryRuntimeConfig struct {
	Enabled   bool
	Namespace string
	OTel      TelemetryOTelConfig
}

func DefaultTelemetryRuntimeConfig() TelemetryRuntimeConfig {
	return TelemetryRuntimeConfig{
		Enabled:   false,
		Namespace: "",
		OTel: TelemetryOTelConfig{
			SecretName:  "wandb-otel-connection",
			Protocol:    "http/protobuf",
			ServiceName: "wandb-service",
		},
	}
}

func (cfg *TelemetryRuntimeConfig) Normalize() {
	cfg.Namespace = strings.TrimSpace(cfg.Namespace)

	cfg.OTel.SecretName = strings.TrimSpace(cfg.OTel.SecretName)
	cfg.OTel.Protocol = strings.TrimSpace(cfg.OTel.Protocol)
	cfg.OTel.ServiceName = strings.TrimSpace(cfg.OTel.ServiceName)
	cfg.OTel.ResourceAttributes = strings.TrimSpace(cfg.OTel.ResourceAttributes)
	if cfg.OTel.SecretName == "" {
		cfg.OTel.SecretName = "wandb-otel-connection"
	}
	if cfg.OTel.Protocol == "" {
		cfg.OTel.Protocol = "http/protobuf"
	}
	if cfg.OTel.ServiceName == "" {
		cfg.OTel.ServiceName = "wandb-service"
	}
}

func (cfg TelemetryRuntimeConfig) Validate() error {
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

	host := resolveServiceHost(telemetryOTLPGatewayName, cfg.Namespace)
	baseURL := fmt.Sprintf("http://%s:%d", host, telemetryOTLPGatewayHTTPPort)
	return TelemetryEndpoints{
		MetricsEndpoint: fmt.Sprintf("%s/v1/metrics", baseURL),
		LogsEndpoint:    fmt.Sprintf("%s/v1/logs", baseURL),
		TracesEndpoint:  fmt.Sprintf("%s/v1/traces", baseURL),
	}
}

func resolveServiceHost(name, namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		return name
	}
	return fmt.Sprintf("%s.%s.svc", name, namespace)
}
