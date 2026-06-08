package keeper

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceTypeName is the kind used for logging/error reporting of the CHK CR.
const ResourceTypeName = "ClickHouseKeeperInstallation"

// WriteState reconciles the desired ClickHouseKeeperInstallation CR.
func WriteState(
	ctx context.Context,
	cl client.Client,
	keeperNsName types.NamespacedName,
	desired *chkv1.ClickHouseKeeperInstallation,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.ClickHouse)
	var actual = &chkv1.ClickHouseKeeperInstallation{}

	found, err := common.GetResource(ctx, cl, keeperNsName, ResourceTypeName, actual)
	if err != nil {
		return []metav1.Condition{
			{Type: common.ReconciledType, Status: metav1.ConditionFalse, Reason: common.ApiErrorReason},
			{Type: KeeperCustomResourceType, Status: metav1.ConditionUnknown, Reason: common.ApiErrorReason},
		}
	}
	if !found {
		actual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, cl, desired, actual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{Type: KeeperCustomResourceType, Status: metav1.ConditionFalse, Reason: common.PendingCreateReason})
	case common.DeleteAction:
		result = append(result, metav1.Condition{Type: KeeperCustomResourceType, Status: metav1.ConditionFalse, Reason: common.PendingDeleteReason})
	case common.UpdateAction:
		result = append(result, metav1.Condition{Type: KeeperCustomResourceType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason})
	case common.NoAction:
		result = append(result, metav1.Condition{Type: KeeperCustomResourceType, Status: metav1.ConditionFalse, Reason: common.NoResourceReason})
	}

	return result
}
