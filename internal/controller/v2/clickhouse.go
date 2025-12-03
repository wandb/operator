package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func clickHouseResourceReconcile(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desired *chiv1.ClickHouseInstallation
	var specNamespacedName = clickHouseSpecNamespacedName(wandb.Spec.ClickHouse)

	if desired, err = translatorv2.ToClickHouseVendorSpec(ctx, wandb.Spec.ClickHouse, wandb, client.Scheme()); err != nil {
		return err
	}
	if err = altinity.CrudResource(ctx, client, specNamespacedName, desired); err != nil {
		return err
	}

	//wandb.Status.ClickHouseStatus = translatorv2.ExtractClickHouseStatus(ctx, results)
	//if err = client.Status().Update(ctx, wandb); err != nil {
	//	results.AddErrors(err)
	//}

	return nil
}

func clickHouseStatusUpdate(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var conditions []common.ClickHouseCondition
	var specNamespacedName = clickHouseSpecNamespacedName(wandb.Spec.ClickHouse)

	if conditions, err = altinity.GetConditions(ctx, client, specNamespacedName); err != nil {
		return err
	}
	wandb.Status.ClickHouseStatus = translatorv2.ExtractClickHouseStatus(ctx, conditions)
	if err = client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "failed to update status")
		return err
	}

	return nil
}

func clickHouseSpecNamespacedName(clickHouse apiv2.WBClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: clickHouse.Namespace,
		Name:      clickHouse.Name,
	}
}
