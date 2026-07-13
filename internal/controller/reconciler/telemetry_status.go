package reconciler

import (
	"context"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	telemetryStateDisabled = "Disabled"
	telemetryStatePending  = "Pending"
	telemetryStateNotReady = "NotReady"
	telemetryStateReady    = "Ready"
	telemetryStateUnknown  = "Unknown"
)

type telemetryResourceRef struct {
	name      string
	namespace string
	gvk       schema.GroupVersionKind
}

func summarizeTelemetryInfraStatus(
	ctx context.Context,
	client ctrlClient.Client,
	cfg TelemetryRuntimeConfig,
) apiv2.TelemetryInfraStatus {
	return apiv2.TelemetryInfraStatus{
		WBInfraStatus: summarizeTelemetryWBInfraStatus(ctx, client, cfg),
		Mode:          cfg.Mode,
		Connection:    summarizeTelemetryConnectionStatus(cfg),
	}
}

func summarizeTelemetryConnectionStatus(cfg TelemetryRuntimeConfig) apiv2.TelemetryConnectionStatus {
	if !cfg.Enabled {
		return apiv2.TelemetryConnectionStatus{}
	}

	resolvedEndpoints := cfg.ResolveEndpoints()

	return apiv2.TelemetryConnectionStatus{
		ManagedNamespace:      cfg.Namespace,
		ConnectionSecret:      cfg.OTel.SecretName,
		Protocol:              cfg.OTel.Protocol,
		MetricsExporter:       "otlp",
		LogsExporter:          "otlp",
		TracesExporter:        "otlp",
		MetricsEndpoint:       resolvedEndpoints.MetricsEndpoint,
		LogsEndpoint:          resolvedEndpoints.LogsEndpoint,
		TracesEndpoint:        resolvedEndpoints.TracesEndpoint,
		ServiceName:           cfg.OTel.ServiceName,
		ResourceAttributes:    cfg.OTel.ResourceAttributes,
		GorillaTracer:         resolveGorillaTracerConnection(cfg.OTel.Protocol, resolvedEndpoints.TracesEndpoint),
		StatsdHost:            resolvedEndpoints.StatsdHost,
		DatadogTraceAgentURL:  resolvedEndpoints.DatadogTraceAgentURL,
		DatadogTraceAgentHost: resolvedEndpoints.DatadogTraceAgentHost,
		DatadogTraceAgentPort: resolvedEndpoints.DatadogTraceAgentPort,
	}
}

func summarizeTelemetryWBInfraStatus(
	ctx context.Context,
	client ctrlClient.Client,
	cfg TelemetryRuntimeConfig,
) apiv2.WBInfraStatus {
	if !cfg.Enabled {
		return apiv2.WBInfraStatus{Ready: false, State: telemetryStateDisabled}
	}

	resources := telemetryResourceRefs(cfg)
	if len(resources) == 0 {
		return apiv2.WBInfraStatus{Ready: false, State: telemetryStateUnknown}
	}

	allReady := true
	anyMissing := false
	for _, resource := range resources {
		ready, found, err := telemetryResourceReady(ctx, client, resource)
		if err != nil {
			return apiv2.WBInfraStatus{Ready: false, State: telemetryStateUnknown}
		}
		if !found {
			anyMissing = true
			allReady = false
			continue
		}
		if !ready {
			allReady = false
		}
	}

	switch {
	case allReady:
		return apiv2.WBInfraStatus{Ready: true, State: telemetryStateReady}
	case anyMissing:
		return apiv2.WBInfraStatus{Ready: false, State: telemetryStatePending}
	default:
		return apiv2.WBInfraStatus{Ready: false, State: telemetryStateNotReady}
	}
}

func telemetryResourceRefs(cfg TelemetryRuntimeConfig) []telemetryResourceRef {
	namespace := cfg.Namespace
	resources := []telemetryResourceRef{
		{
			name:      "victoria-instance",
			namespace: namespace,
			gvk: schema.GroupVersionKind{
				Group:   "operator.victoriametrics.com",
				Version: "v1beta1",
				Kind:    "VMSingle",
			},
		},
		{
			name:      "victoria-agent",
			namespace: namespace,
			gvk: schema.GroupVersionKind{
				Group:   "operator.victoriametrics.com",
				Version: "v1beta1",
				Kind:    "VMAgent",
			},
		},
		{
			name:      "victoria-logs",
			namespace: namespace,
			gvk: schema.GroupVersionKind{
				Group:   "operator.victoriametrics.com",
				Version: "v1",
				Kind:    "VLSingle",
			},
		},
		{
			name:      "victoria-traces",
			namespace: namespace,
			gvk: schema.GroupVersionKind{
				Group:   "operator.victoriametrics.com",
				Version: "v1",
				Kind:    "VTSingle",
			},
		},
		{
			name:      telemetryOTLPGatewayName,
			namespace: namespace,
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
		},
	}

	if cfg.Mode == telemetryModeFull {
		resources = append(resources, telemetryResourceRef{
			name:      "grafana",
			namespace: namespace,
			gvk: schema.GroupVersionKind{
				Group:   "grafana.integreatly.org",
				Version: "v1beta1",
				Kind:    "Grafana",
			},
		})
	}

	return resources
}

func telemetryResourceReady(
	ctx context.Context,
	client ctrlClient.Client,
	resource telemetryResourceRef,
) (bool, bool, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(resource.gvk)

	if err := client.Get(ctx, types.NamespacedName{Name: resource.name, Namespace: resource.namespace}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}

	return telemetryObjectReady(obj), true, nil
}

func telemetryObjectReady(obj *unstructured.Unstructured) bool {
	if conditions := telemetryConditions(obj); len(conditions) > 0 {
		if cond := findTelemetryReadyCondition(conditions); cond != nil {
			return cond.Status == metav1.ConditionTrue
		}
		if cond := meta.FindStatusCondition(conditions, "Degraded"); cond != nil && cond.Status == metav1.ConditionTrue {
			return false
		}
		if cond := meta.FindStatusCondition(conditions, "Progressing"); cond != nil && cond.Status == metav1.ConditionTrue {
			return false
		}
	}

	stage, _, _ := unstructured.NestedString(obj.Object, "status", "stage")
	stageStatus, _, _ := unstructured.NestedString(obj.Object, "status", "stageStatus")
	updateStatus, _, _ := unstructured.NestedString(obj.Object, "status", "updateStatus")
	return strings.EqualFold(stage, "complete") ||
		strings.EqualFold(stageStatus, "complete") ||
		strings.EqualFold(stageStatus, "success") ||
		strings.EqualFold(updateStatus, "operational")
}

func telemetryConditions(obj *unstructured.Unstructured) []metav1.Condition {
	rawConditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return nil
	}

	conditions := make([]metav1.Condition, 0, len(rawConditions))
	for _, rawCondition := range rawConditions {
		rawMap, ok := rawCondition.(map[string]any)
		if !ok {
			continue
		}

		condition := metav1.Condition{}
		if value, ok := rawMap["type"].(string); ok {
			condition.Type = value
		}
		if value, ok := rawMap["status"].(string); ok {
			condition.Status = metav1.ConditionStatus(value)
		}
		if value, ok := rawMap["reason"].(string); ok {
			condition.Reason = value
		}
		if value, ok := rawMap["message"].(string); ok {
			condition.Message = value
		}
		conditions = append(conditions, condition)
	}

	return conditions
}

func findTelemetryReadyCondition(conditions []metav1.Condition) *metav1.Condition {
	for _, conditionType := range []string{"Available", "Ready", "Reconciled"} {
		if cond := meta.FindStatusCondition(conditions, conditionType); cond != nil {
			return cond
		}
	}
	return nil
}
