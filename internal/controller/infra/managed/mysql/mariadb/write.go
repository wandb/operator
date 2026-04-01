package mariadb

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/vendored/mariadb-operator/k8s.mariadb.com/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "MariaDB"
	AppConnTypeName  = "MySQLAppConn"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desired *v1alpha1.MariaDB,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.Mysql)
	var actual = &v1alpha1.MariaDB{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := common.GetResource(
		ctx, client, nsnBuilder.ClusterNsName(), ResourceTypeName, actual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   MySQLCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		actual = nil
	} else {
		desired.Spec.RootEmptyPassword = actual.Spec.RootEmptyPassword
		desired.Spec.RootPasswordSecretKeyRef = actual.Spec.RootPasswordSecretKeyRef
		desired.Spec.ServiceAccountName = actual.Spec.ServiceAccountName
		desired.Spec.Storage.Ephemeral = actual.Spec.Storage.Ephemeral
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, client, desired, actual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   MySQLCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   MySQLCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   MySQLCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   MySQLCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}
