package seaweedfs

import (
	"context"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
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

	var actualResource = &seaweedv1.Seaweed{}
	conditions := make([]metav1.Condition, 0)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := ctrlcommon.GetResource(
		ctx, k8sClient, nsnBuilder.SpecNsName(), ResourceTypeName, actualResource,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   SeaweedCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: ctrlcommon.ApiErrorReason,
			},
		}
	}
	if !found {
		actualResource = nil
		if onDeleteRule.Policy == ctrlcommon.Purge {
			log.Debug(
				"Attempting to purge associated seaweedfs resources after deletion",
				"seaweedName", SeaweedName(specNamespacedName.Name),
			)
			err = purgeAssociatedResources(ctx, k8sClient, specNamespacedName.Namespace, onDeleteRule.Selector)
			if err != nil {
				conditions = append(
					conditions,
					metav1.Condition{
						Type:   SeaweedCustomResourceType,
						Status: metav1.ConditionUnknown,
						Reason: ctrlcommon.ApiErrorReason,
					},
				)
			} else {
				conditions = append(conditions, metav1.Condition{
					Type:   SeaweedReportedReadyType,
					Status: metav1.ConditionFalse,
					Reason: ctrlcommon.PendingDeleteReason,
				},
				)
			}
		}
	}

	if actualResource != nil {
		conditions = append(conditions, computeSeaweedReportedReadyCondition(ctx, actualResource)...)
	}
	log.Debug("read", "resourceExists", actualResource != nil, "rule", onDeleteRule.Policy)
	return conditions
}

func computeSeaweedReportedReadyCondition(_ context.Context, cr *seaweedv1.Seaweed) []metav1.Condition {
	if cr == nil {
		return []metav1.Condition{}
	}

	allReady := true
	anyRunning := false

	components := []struct {
		name   string
		status seaweedv1.ComponentStatus
	}{
		{"master", cr.Status.Master},
		{"volume", cr.Status.Volume},
		{"filer", cr.Status.Filer},
	}

	for _, c := range components {
		if c.status.Replicas == 0 {
			continue
		}
		if c.status.ReadyReplicas > 0 {
			anyRunning = true
		}
		if c.status.ReadyReplicas < c.status.Replicas {
			allReady = false
		}
	}

	var status metav1.ConditionStatus
	var reason string

	switch {
	case cr.Status.Filer.Replicas > 0 && cr.Status.Filer.ReadyReplicas == 0:
		status = metav1.ConditionFalse
		reason = "red"
	case allReady && anyRunning:
		status = metav1.ConditionTrue
		reason = "green"
	case anyRunning:
		status = metav1.ConditionFalse
		reason = "yellow"
	default:
		status = metav1.ConditionUnknown
		reason = ctrlcommon.UnknownReason
	}

	return []metav1.Condition{
		{
			Type:   SeaweedReportedReadyType,
			Status: status,
			Reason: reason,
		},
	}
}
