package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	chiv2 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
)

func (r *WeightsAndBiasesV2Reconciler) clickHouseResourceReconcile(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desired *chiv2.ClickHouseInstallation

	if desired, err = translatorv2.ToClickHouseVendorSpec(ctx, wandb.Spec.ClickHouse, wandb, r.Scheme); err != nil {
		return err
	}
	if err = altinity.CrudResource(ctx, r.Client, translatorv2.ClickHouseNamespacedName(wandb.Spec.ClickHouse), desired); err != nil {
		return err
	}

	//wandb.Status.ClickHouseStatus = translatorv2.ExtractClickHouseStatus(ctx, results)
	//if err = r.Status().Update(ctx, wandb); err != nil {
	//	results.AddErrors(err)
	//}

	return nil
}
