package supportbundle

import (
	"fmt"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
)

// operatorResources are the operator's custom resources; collection and the collector's RBAC both use this list.
var operatorResources = []struct{ kind, plural string }{
	{"ActionRun", "actionruns"},
	{"Application", "applications"},
	{"WeightsAndBiases", "weightsandbiases"},
}

const crdKind = "CustomResourceDefinition"

func operatorPlurals() []string {
	plurals := make([]string, 0, len(operatorResources))
	for _, r := range operatorResources {
		plurals = append(plurals, r.plural)
	}
	return plurals
}

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
	return append(selectorArgs(wandb), victoriaArgs(wandb)...)
}

// BlockedReason explains why a collection would gather nothing, or is empty when it has something to collect.
func BlockedReason(wandb *apiv2.WeightsAndBiases) string {
	if len(collectionArgs(wandb)) > 0 {
		return ""
	}
	return "nothing to collect: selectedResources is disabled and no Victoria export is enabled with telemetry on"
}

// selectorArgs sends the CR's selectors, or each operator CR and its CRD when none are set
func selectorArgs(wandb *apiv2.WeightsAndBiases) []string {
	resources := wandb.Spec.SupportBundle.SelectedResources
	if resources.Enabled != nil && !*resources.Enabled {
		return nil
	}
	selectors := resources.Selectors
	if len(selectors) == 0 {
		for _, r := range operatorResources {
			resource := r.plural + "." + apiv2.GroupVersion.Group
			selectors = append(selectors,
				apiv2.SupportBundleResourceSelector{Kind: r.kind, Resource: resource},
				apiv2.SupportBundleResourceSelector{Kind: crdKind, Resource: resource},
			)
		}
	}

	var args []string
	for i, s := range selectors {
		// the Role only covers the install's namespace, so all namespaces would just be a 403
		if s.Namespace == "" && s.Kind != crdKind {
			s.Namespace = wandb.Namespace
		}
		option := fmt.Sprintf("%s=s%d.", flagSelectedResourceOption, i)
		args = append(args,
			option+selectorKind+"="+escape(s.Kind),
			option+selectorResource+"="+escape(s.Resource),
			option+selectorNamespace+"="+escape(s.Namespace),
			option+selectorLabelSelector+"="+escape(s.LabelSelector),
			option+selectorFieldSelector+"="+escape(s.FieldSelector),
		)
	}
	return args
}

// escape stops the kubelet expanding $(VAR) in CR values, which could pull Secret-backed env into the args
func escape(value string) string {
	return strings.ReplaceAll(value, "$", "$$")
}

type victoriaExport struct {
	name     string
	flag     string
	endpoint string
	capKey   string
	spec     apiv2.SupportBundleVictoriaExport
}

// victoriaExports are the exports that will run; the collector's args and its export RBAC both come from this.
func victoriaExports(wandb *apiv2.WeightsAndBiases) (string, []victoriaExport) {
	conn := wandb.Status.TelemetryStatus.Connection
	stack := wandb.Spec.SupportBundle.VictoriaStack
	var exports []victoriaExport
	for _, export := range []victoriaExport{
		{"metrics", flagVictoriaMetrics, conn.MetricsReadEndpoint, victoriaMaxLines, stack.Metrics},
		{"logs", flagVictoriaLogs, conn.LogsReadEndpoint, victoriaMaxLines, stack.Logs},
		{"traces", flagVictoriaTraces, conn.TracesReadEndpoint, victoriaMaxSpans, stack.Traces},
	} {
		if export.endpoint == "" || (export.spec.Enabled != nil && !*export.spec.Enabled) {
			continue
		}
		exports = append(exports, export)
	}
	namespace := conn.ManagedNamespace
	if namespace == "" {
		namespace = wandb.Namespace
	}
	return namespace, exports
}

// victoriaArgs sends the enabled exports and their bounds; anything unset keeps the collector's default.
func victoriaArgs(wandb *apiv2.WeightsAndBiases) []string {
	namespace, exports := victoriaExports(wandb)
	if len(exports) == 0 {
		return nil
	}
	stack := wandb.Spec.SupportBundle.VictoriaStack
	args := []string{flagVictoriaNamespace + "=" + namespace}
	var options []string
	for _, export := range exports {
		args = append(args, export.flag+"="+export.endpoint)
		// the collector rejects options for an export it wasn't given, so they're only sent with the endpoint
		if export.spec.LookbackDuration != "" {
			options = append(options, flagVictoriaOption+"="+export.name+"."+victoriaLookbackDuration+"="+export.spec.LookbackDuration)
		}
		if export.spec.MaxRecords != nil {
			options = append(options, fmt.Sprintf("%s=%s.%s=%d", flagVictoriaOption, export.name, export.capKey, *export.spec.MaxRecords))
		}
	}
	if stack.LookbackDuration != "" {
		args = append(args, flagVictoriaOption+"="+victoriaLookbackDuration+"="+stack.LookbackDuration)
	}
	if stack.Timeout != "" {
		args = append(args, flagVictoriaOption+"="+victoriaTimeout+"="+stack.Timeout)
	}
	return append(args, options...)
}
