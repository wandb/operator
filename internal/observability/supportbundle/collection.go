package supportbundle

import (
	"slices"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
)

// CollectorName is a constrained enum holding all compatible collectors
type CollectorName string

const (
	ClusterInfo      CollectorName = "cluster-info"
	ClusterResources CollectorName = "cluster-resources"
	PodLogs          CollectorName = "pod-logs"
	DNS              CollectorName = "dns"
	VictoriaMetrics  CollectorName = "victoria-metrics"
	VictoriaLogs     CollectorName = "victoria-logs"
	VictoriaTraces   CollectorName = "victoria-traces"
)

var allCollectors = []CollectorName{ClusterInfo, ClusterResources, PodLogs, DNS, VictoriaMetrics, VictoriaLogs, VictoriaTraces}

const defaultPreset = "full"

// presets become ConfigMap keys "<preset>.yaml"; a launch picks one with --support-bundle-spec.
var presets = map[string][]CollectorName{defaultPreset: allCollectors}

// Sets the collection bounds
const (
	victoriaLookback = "24h"
	victoriaTimeout  = "2m"

	podLogsMaxAge   = "24h"
	podLogsMaxBytes = 50000000
)

// presetCollects checks whether any preset needs a collector
func presetCollects(name CollectorName) bool {
	for _, collectors := range presets {
		if slices.Contains(collectors, name) {
			return true
		}
	}
	return false
}

// collectsTelemetry reports whether any preset reads from the operator's Victoria stack
func collectsTelemetry(cfg telemetry.TelemetryRuntimeConfig) bool {
	// If telemetry is present, and we're asking for one of the victoria resources
	return cfg.Enabled && (presetCollects(VictoriaMetrics) || presetCollects(VictoriaLogs) || presetCollects(VictoriaTraces))
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

// ConfigMap creates a map of a key per preset
func ConfigMap(wandb *apiv2.WeightsAndBiases, cfg telemetry.TelemetryRuntimeConfig) (*corev1.ConfigMap, error) {
	data := map[string]string{}
	for preset, names := range presets {
		rendered, err := renderCollection(wandb, cfg, names)
		if err != nil {
			return nil, err
		}
		data[preset+".yaml"] = string(rendered)
	}
	return &corev1.ConfigMap{
		Name: Name(wandb), Namespace: wandb.Namespace, Labels: objectLabels(wandb),
		Data: data,
	}, nil
}

// renderCollection renders a YAML based on the compilation of all collectors, redactors, and docs
func renderCollection(wandb *apiv2.WeightsAndBiases, cfg telemetry.TelemetryRuntimeConfig, names []CollectorName) ([]byte, error) {
	selected := map[CollectorName]bool{}
	for _, name := range names {
		selected[name] = true
	}

	var collectors []collector
	if selected[ClusterInfo] {
		collectors = append(collectors, collector{ClusterInfo: &struct{}{}})
	}
	if selected[ClusterResources] {
		collectors = append(collectors, collector{ClusterResources: &clusterResources{Namespaces: collectionNamespaces(wandb, cfg)}})
	}
	if selected[PodLogs] {
		collectors = append(collectors, collector{Logs: &podLogs{
			Name:      wandb.Name + "-pod-logs",
			Namespace: wandb.Namespace,
			Selector:  []string{},
			Limits:    podLogLimit{MaxAge: podLogsMaxAge, MaxBytes: podLogsMaxBytes},
		}})
	}
	if selected[DNS] {
		collectors = append(collectors, collector{DNS: &dnsCheck{}})
	}
	if victoria := victoriaCollector(wandb, cfg, selected); victoria != nil {
		collectors = append(collectors, collector{VictoriaStack: victoria})
	}

	return yaml.Marshal(collection{
		APIVersion: collectionAPIVersion,
		Kind:       collectionKind,
		Metadata:   collectionMetadata{Name: Name(wandb)},
		Spec:       collectionSpec{Collectors: collectors, Redactors: defaultRedactors},
	})
}

// victoriaCollector checks if Victoria is Ready in order to be collected
func victoriaCollector(wandb *apiv2.WeightsAndBiases, cfg telemetry.TelemetryRuntimeConfig, selected map[CollectorName]bool) *victoriaStack {
	// if not available, don't do anything
	if !cfg.Enabled || !wandb.Status.TelemetryStatus.Ready {
		return nil
	}
	endpoints := cfg.ResolveEndpoints()
	stack := &victoriaStack{
		Name:      victoriaCollectorName,
		Namespace: telemetryNamespace(wandb, cfg),
		Lookback:  victoriaLookback,
		Timeout:   victoriaTimeout,
	}
	if selected[VictoriaMetrics] {
		stack.Metrics = &victoriaEndpoint{Endpoint: endpoints.MetricsReadEndpoint}
	}
	if selected[VictoriaLogs] {
		stack.Logs = &victoriaEndpoint{Endpoint: endpoints.LogsReadEndpoint}
	}
	if selected[VictoriaTraces] {
		stack.Traces = &victoriaEndpoint{Endpoint: endpoints.TracesReadEndpoint}
	}
	if stack.Metrics == nil && stack.Logs == nil && stack.Traces == nil {
		return nil
	}
	return stack
}
