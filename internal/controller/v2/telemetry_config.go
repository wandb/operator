package v2

import (
	"fmt"
	"strings"
)

type TelemetryMode string

const (
	TelemetryModeManaged  TelemetryMode = "managed"
	TelemetryModeExternal TelemetryMode = "external"
)

type TelemetryEndpoints struct {
	MetricsEndpoint string
	LogsEndpoint    string
	TracesEndpoint  string
}

type TelemetryManagedConfig struct {
	Namespace    string
	VMSingleName string
	VLSingleName string
	VTSingleName string
	OTLPGateway  TelemetryManagedOTLPGatewayConfig
}

type TelemetryManagedOTLPGatewayConfig struct {
	Enabled  bool
	Name     string
	HTTPPort int
}

type TelemetryOTelConfig struct {
	SecretName         string
	Protocol           string
	ServiceName        string
	ResourceAttributes string
}

type TelemetryRuntimeConfig struct {
	Enabled  bool
	Mode     TelemetryMode
	Managed  TelemetryManagedConfig
	External TelemetryEndpoints
	OTel     TelemetryOTelConfig
}

func DefaultTelemetryRuntimeConfig() TelemetryRuntimeConfig {
	return TelemetryRuntimeConfig{
		Enabled: false,
		Mode:    TelemetryModeManaged,
		Managed: TelemetryManagedConfig{
			Namespace:    "",
			VMSingleName: "victoria-instance",
			VLSingleName: "victoria-logs",
			VTSingleName: "victoria-traces",
			OTLPGateway: TelemetryManagedOTLPGatewayConfig{
				Enabled:  true,
				Name:     "victoria-otlp-gateway",
				HTTPPort: 4318,
			},
		},
		OTel: TelemetryOTelConfig{
			SecretName:  "wandb-otel-connection",
			Protocol:    "http/protobuf",
			ServiceName: "wandb-service",
		},
	}
}

func (cfg *TelemetryRuntimeConfig) Normalize() {
	cfg.Mode = TelemetryMode(strings.ToLower(strings.TrimSpace(string(cfg.Mode))))
	if cfg.Mode == "" {
		cfg.Mode = TelemetryModeManaged
	}

	cfg.Managed.Namespace = strings.TrimSpace(cfg.Managed.Namespace)
	cfg.Managed.VMSingleName = strings.TrimSpace(cfg.Managed.VMSingleName)
	cfg.Managed.VLSingleName = strings.TrimSpace(cfg.Managed.VLSingleName)
	cfg.Managed.VTSingleName = strings.TrimSpace(cfg.Managed.VTSingleName)
	cfg.Managed.OTLPGateway.Name = strings.TrimSpace(cfg.Managed.OTLPGateway.Name)
	if cfg.Managed.VMSingleName == "" {
		cfg.Managed.VMSingleName = "victoria-instance"
	}
	if cfg.Managed.VLSingleName == "" {
		cfg.Managed.VLSingleName = "victoria-logs"
	}
	if cfg.Managed.VTSingleName == "" {
		cfg.Managed.VTSingleName = "victoria-traces"
	}
	if cfg.Managed.OTLPGateway.Name == "" {
		cfg.Managed.OTLPGateway.Name = "victoria-otlp-gateway"
	}
	if cfg.Managed.OTLPGateway.HTTPPort <= 0 {
		cfg.Managed.OTLPGateway.HTTPPort = 4318
	}

	cfg.External.MetricsEndpoint = strings.TrimSpace(cfg.External.MetricsEndpoint)
	cfg.External.LogsEndpoint = strings.TrimSpace(cfg.External.LogsEndpoint)
	cfg.External.TracesEndpoint = strings.TrimSpace(cfg.External.TracesEndpoint)

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

	switch cfg.Mode {
	case TelemetryModeManaged:
		return nil
	case TelemetryModeExternal:
		if cfg.External.MetricsEndpoint == "" || cfg.External.LogsEndpoint == "" || cfg.External.TracesEndpoint == "" {
			return fmt.Errorf("external telemetry mode requires metrics, logs, and traces endpoints")
		}
		return nil
	default:
		return fmt.Errorf("unsupported telemetry mode %q", cfg.Mode)
	}
}

func (cfg TelemetryRuntimeConfig) ResolveEndpoints() TelemetryEndpoints {
	if !cfg.Enabled {
		return TelemetryEndpoints{}
	}

	if cfg.Mode == TelemetryModeExternal {
		return cfg.External
	}

	if cfg.Managed.OTLPGateway.Enabled {
		host := resolveServiceHost(cfg.Managed.OTLPGateway.Name, cfg.Managed.Namespace)
		baseURL := fmt.Sprintf("http://%s:%d", host, cfg.Managed.OTLPGateway.HTTPPort)
		return TelemetryEndpoints{
			MetricsEndpoint: fmt.Sprintf("%s/v1/metrics", baseURL),
			LogsEndpoint:    fmt.Sprintf("%s/v1/logs", baseURL),
			TracesEndpoint:  fmt.Sprintf("%s/v1/traces", baseURL),
		}
	}

	vmHost := resolveServiceHost(fmt.Sprintf("vmsingle-%s", cfg.Managed.VMSingleName), cfg.Managed.Namespace)
	vlHost := resolveServiceHost(fmt.Sprintf("vlsingle-%s", cfg.Managed.VLSingleName), cfg.Managed.Namespace)
	vtHost := resolveServiceHost(fmt.Sprintf("vtsingle-%s", cfg.Managed.VTSingleName), cfg.Managed.Namespace)
	return TelemetryEndpoints{
		MetricsEndpoint: fmt.Sprintf("http://%s:8428/opentelemetry/v1/metrics", vmHost),
		LogsEndpoint:    fmt.Sprintf("http://%s:9428/insert/opentelemetry/v1/logs", vlHost),
		TracesEndpoint:  fmt.Sprintf("http://%s:10428/insert/opentelemetry/v1/traces", vtHost),
	}
}

func resolveServiceHost(name, namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		return name
	}
	return fmt.Sprintf("%s.%s.svc", name, namespace)
}
