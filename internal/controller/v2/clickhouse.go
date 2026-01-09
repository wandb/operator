package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func clickHouseWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	var specNamespacedName = clickHouseSpecNamespacedName(wandb.Spec.ClickHouse)

	desired, err := translatorv2.ToClickHouseVendorSpec(ctx, wandb.Spec.ClickHouse, wandb, client.Scheme())
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	results := altinity.WriteState(ctx, client, specNamespacedName, desired)
	return results
}

func clickHouseReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.InfraConnection) {
	specNamespacedName := clickHouseSpecNamespacedName(wandb.Spec.ClickHouse)
	readConditions, newInfraConn := altinity.ReadState(ctx, client, specNamespacedName, wandb)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func clickHouseInferStatus(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.InfraConnection,
) error {
	oldConditions := wandb.Status.ClickHouseStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorInfraConnection(wandb.Status.ClickHouseStatus.Connection)

	updatedStatus := altinity.ComputeStatus(
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	wandb.Status.ClickHouseStatus = translatorv2.ToWbInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return err
}

func clickHouseSpecNamespacedName(clickHouse apiv2.WBClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: clickHouse.Namespace,
		Name:      clickHouse.Name,
	}
}
