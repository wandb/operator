package bufstream

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
	onDeleteRule ctrlcommon.OnDeleteRule,
) ([]metav1.Condition, *apiv2.KafkaConnection) {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	bufApp := &apiv2.Application{}
	found, err := ctrlcommon.GetResource(ctx, cl, nsnBuilder.BufstreamNsName(), ApplicationResourceType, bufApp)
	if err != nil {
		return []metav1.Condition{{
			Type:   BufstreamApplicationType,
			Status: metav1.ConditionUnknown,
			Reason: ctrlcommon.ApiErrorReason,
		}}, nil
	}

	conditions := make([]metav1.Condition, 0)

	if !found {
		log.Info("Bufstream Application CR not found")
		bufApp = nil
		if onDeleteRule.Policy == ctrlcommon.Purge {
			if err := purgeAssociatedResources(ctx, cl, specNamespacedName.Namespace, onDeleteRule.Selector); err != nil {
				conditions = append(conditions, metav1.Condition{
					Type:   BufstreamApplicationType,
					Status: metav1.ConditionUnknown,
					Reason: ctrlcommon.ApiErrorReason,
				})
			} else {
				conditions = append(conditions, metav1.Condition{
					Type:   BufstreamApplicationType,
					Status: metav1.ConditionFalse,
					Reason: ctrlcommon.PendingDeleteReason,
				})
			}
		}
		return conditions, nil
	}

	etcdApp := &apiv2.Application{}
	etcdReady := false
	if etcdFound, etcdErr := ctrlcommon.GetResource(ctx, cl, nsnBuilder.EtcdNsName(), ApplicationResourceType, etcdApp); etcdErr == nil && etcdFound {
		etcdReady = etcdApp.Status.Ready
	}

	connInfo := readConnectionDetails(nsnBuilder)
	connection, err := writeKafkaConnInfo(ctx, cl, wandbOwner, nsnBuilder, connInfo)
	if err != nil {
		return []metav1.Condition{{
			Type:   KafkaConnectionInfoType,
			Status: metav1.ConditionUnknown,
			Reason: ctrlcommon.ApiErrorReason,
		}}, nil
	}
	conditions = append(conditions, metav1.Condition{
		Type:   KafkaConnectionInfoType,
		Status: metav1.ConditionTrue,
		Reason: ctrlcommon.ResourceExistsReason,
	})

	reportedReady := metav1.ConditionFalse
	if bufApp.Status.Ready && etcdReady {
		reportedReady = metav1.ConditionTrue
	}
	conditions = append(conditions, metav1.Condition{
		Type:   KafkaReportedReadyType,
		Status: reportedReady,
		Reason: ctrlcommon.ReportedStatusReason,
	})

	return conditions, connection
}
