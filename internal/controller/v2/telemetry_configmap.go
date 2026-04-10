package v2

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	telemetryConfigKeyEnabled            = "TELEMETRY_ENABLED"
	telemetryConfigKeyMode               = "TELEMETRY_MODE"
	telemetryConfigKeyManagedNamespace   = "TELEMETRY_MANAGED_NAMESPACE"
	telemetryConfigKeyOTelSecretName     = "TELEMETRY_OTEL_SECRET_NAME"
	telemetryConfigKeyOTelProtocol       = "TELEMETRY_OTEL_PROTOCOL"
	telemetryConfigKeyOTelServiceName    = "TELEMETRY_OTEL_SERVICE_NAME"
	telemetryConfigKeyResourceAttributes = "TELEMETRY_OTEL_RESOURCE_ATTRIBUTES"
)

func LoadTelemetryRuntimeConfigFromConfigMap(
	ctx context.Context,
	client ctrlClient.Client,
	ref types.NamespacedName,
	defaults TelemetryRuntimeConfig,
) (TelemetryRuntimeConfig, error) {
	cfg := defaults
	if ref.Name == "" {
		cfg.Normalize()
		return cfg, cfg.Validate()
	}

	cm := &corev1.ConfigMap{}
	if err := client.Get(ctx, ref, cm); err != nil {
		if apierrors.IsNotFound(err) {
			cfg.Normalize()
			return cfg, cfg.Validate()
		}
		return TelemetryRuntimeConfig{}, err
	}

	if value, ok := cm.Data[telemetryConfigKeyEnabled]; ok && strings.TrimSpace(value) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return TelemetryRuntimeConfig{}, fmt.Errorf("parse %s: %w", telemetryConfigKeyEnabled, err)
		}
		cfg.Enabled = enabled
	}
	if value, ok := cm.Data[telemetryConfigKeyMode]; ok {
		cfg.Mode = value
	}
	if value, ok := cm.Data[telemetryConfigKeyManagedNamespace]; ok {
		cfg.Namespace = value
	}
	if value, ok := cm.Data[telemetryConfigKeyOTelSecretName]; ok {
		cfg.OTel.SecretName = value
	}
	if value, ok := cm.Data[telemetryConfigKeyOTelProtocol]; ok {
		cfg.OTel.Protocol = value
	}
	if value, ok := cm.Data[telemetryConfigKeyOTelServiceName]; ok {
		cfg.OTel.ServiceName = value
	}
	if value, ok := cm.Data[telemetryConfigKeyResourceAttributes]; ok {
		cfg.OTel.ResourceAttributes = value
	}

	cfg.Normalize()
	return cfg, cfg.Validate()
}
