package tenant

import (
	"context"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	miniov2 "github.com/wandb/operator/pkg/vendored/minio-operator/minio.min.io/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	k8sClient client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule ctrlcommon.OnDeleteRule,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.ObjectStore)
	log := logx.GetSlog(ctx)

	var actualResource = &miniov2.Tenant{}
	conditions := make([]metav1.Condition, 0)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := ctrlcommon.GetResource(
		ctx, k8sClient, nsnBuilder.SpecNsName(), ResourceTypeName, actualResource,
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
		if onDeleteRule.Policy == ctrlcommon.Purge {
			log.Debug(
				"Attempting to purge associated minio resources after deletion",
				"tenantName", TenantName(specNamespacedName.Name),
			)
			err = purgeAssociatedResources(ctx, k8sClient, specNamespacedName.Namespace, onDeleteRule.Selector)
			if err != nil {
				conditions = append(
					conditions,
					metav1.Condition{
						Type:   MinioCustomResourceType,
						Status: metav1.ConditionUnknown,
						Reason: ctrlcommon.ApiErrorReason,
					},
				)
			} else {
				conditions = append(conditions, metav1.Condition{
					Type:   MinioReportedReadyType,
					Status: metav1.ConditionFalse,
					Reason: ctrlcommon.PendingDeleteReason,
				},
				)
			}
		}
	}

	if actualResource != nil {
		conditions = append(conditions, computeMinioReportedReadyCondition(ctx, actualResource)...)
	}
	log.Debug("read", "actualResource", actualResource, "rule", onDeleteRule.Policy)
	return conditions
}

func computeMinioReportedReadyCondition(_ context.Context, tenantCR *miniov2.Tenant) []metav1.Condition {
	if tenantCR == nil {
		return []metav1.Condition{}
	}

	var status metav1.ConditionStatus
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
