package supportbundle

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
)

// operatorResources are the operator's custom resources; collection and the collector's RBAC both use this list.
var operatorResources = []string{"actionruns", "applications", "weightsandbiases"}

// collectsTelemetry reports whether collection reads from the operator's Victoria stack
func collectsTelemetry(cfg telemetry.TelemetryRuntimeConfig) bool {
	return cfg.Enabled
}

// telemetryNamespace is where the Victoria stack runs; an unset namespace means the install's own.
func telemetryNamespace(wandb *apiv2.WeightsAndBiases, cfg telemetry.TelemetryRuntimeConfig) string {
	if cfg.Namespace != "" {
		return cfg.Namespace
	}
	return wandb.Namespace
}

// collectionNamespaces are the namespaces whose Kubernetes resources are collected
func collectionNamespaces(wandb *apiv2.WeightsAndBiases, cfg telemetry.TelemetryRuntimeConfig) []string {
	namespaces := []string{wandb.Namespace}
	if ns := telemetryNamespace(wandb, cfg); collectsTelemetry(cfg) && ns != wandb.Namespace {
		namespaces = append(namespaces, ns)
	}
	return namespaces
}

// collectionArgs tells the collector what to gather
func collectionArgs(wandb *apiv2.WeightsAndBiases) []string {
	var args []string
	for _, resource := range operatorResources {
		args = append(args, "--custom-resource="+resource+"."+apiv2.GroupVersion.Group)
	}
	args = append(args, "--custom-resource-namespace="+wandb.Namespace)

	conn := wandb.Status.TelemetryStatus.Connection
	if conn.MetricsReadEndpoint == "" {
		return args
	}
	namespace := conn.ManagedNamespace
	if namespace == "" {
		namespace = wandb.Namespace
	}
	args = append(args,
		"--victoria-namespace="+namespace,
		"--victoria-metrics="+conn.MetricsReadEndpoint,
		"--victoria-logs="+conn.LogsReadEndpoint,
		"--victoria-traces="+conn.TracesReadEndpoint,
	)
	return append(args, victoriaOptions(wandb.Spec.SupportBundle.VictoriaStack)...)
}

// victoriaOptions passes the CR's bounds through; anything unset keeps the collector's default.
func victoriaOptions(stack apiv2.SupportBundleVictoriaStack) []string {
	var options []string
	text := func(key, value string) {
		if value != "" {
			options = append(options, "--victoria-option="+key+"="+value)
		}
	}
	limit := func(key string, value *int64) {
		if value != nil {
			options = append(options, fmt.Sprintf("--victoria-option=%s=%d", key, *value))
		}
	}
	text("lookback", stack.Lookback)
	text("timeout", stack.Timeout)
	text("metrics.lookback", stack.Metrics.Lookback)
	text("logs.lookback", stack.Logs.Lookback)
	text("traces.lookback", stack.Traces.Lookback)
	limit("metrics.maxLines", stack.Metrics.MaxRecords)
	limit("logs.maxLines", stack.Logs.MaxRecords)
	limit("traces.maxSpans", stack.Traces.MaxRecords)
	return options
}
