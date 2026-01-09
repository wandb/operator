package tenant

import (
	"context"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
) []metav1.Condition {
	var actualResource = &miniov2.Tenant{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := ctrlcommon.GetResource(
		ctx, client, nsnBuilder.SpecNsName(), ResourceTypeName, actualResource,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   MinioCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}
	}
	if !found {
		actualResource = nil
	}

	conditions := make([]metav1.Condition, 0)

	if actualResource != nil {
		conditions = append(conditions, computeMinioReportedReadyCondition(ctx, actualResource)...)
	}

	return conditions
}

func computeMinioReportedReadyCondition(_ context.Context, tenantCR *miniov2.Tenant) []metav1.Condition {
	if tenantCR == nil {
		return []metav1.Condition{}
	}

	status := metav1.ConditionUnknown
	reason := string(tenantCR.Status.HealthStatus)

	switch tenantCR.Status.HealthStatus {
	case miniov2.HealthStatusGreen:
		status = metav1.ConditionTrue
	case miniov2.HealthStatusYellow:
		status = metav1.ConditionFalse
		reason = "yellow"
	case miniov2.HealthStatusRed:
		status = metav1.ConditionFalse
		reason = "red"
	default:
		status = metav1.ConditionUnknown
		reason = ctrlcommon.UnknownReason
	}

	return []metav1.Condition{
		{
			Type:   MinioReportedReadyType,
			Status: status,
			Reason: reason,
		},
	}
}
