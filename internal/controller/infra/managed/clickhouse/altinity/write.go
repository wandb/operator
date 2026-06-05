package altinity

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity/keeper"
	"github.com/wandb/operator/internal/logx"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "ClickHouseInstallation"
	AppConnTypeName  = "ClickHouseAppConn"
)

// WriteState reconciles the ClickHouse Keeper ensemble and the ClickHouse
// installation. Keeper is written first because ReplicatedMergeTree depends on
// it for replication coordination. Conditions from both are aggregated; each
// resource reports under its own condition type.
func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredKeeper *chkv1.ClickHouseKeeperInstallation,
	desired *chiv1.ClickHouseInstallation,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.ClickHouse)
	results := make([]metav1.Condition, 0)

	results = append(results, keeper.WriteState(
		ctx, client,
		types.NamespacedName{Namespace: desiredKeeper.Namespace, Name: desiredKeeper.Name},
		desiredKeeper,
	)...)
	results = append(results, writeClickHouseInstallation(ctx, client, specNamespacedName, desired)...)

	return results
}

func writeClickHouseInstallation(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desired *chiv1.ClickHouseInstallation,
) []metav1.Condition {
	var actual = &chiv1.ClickHouseInstallation{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := common.GetResource(
		ctx, client, nsnBuilder.InstallationNsName(), ResourceTypeName, actual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   ClickHouseCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		actual = nil
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
			Type:   ClickHouseCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   ClickHouseCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   ClickHouseCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   ClickHouseCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}
