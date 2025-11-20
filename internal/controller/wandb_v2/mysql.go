package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	v1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
)

func (r *WeightsAndBiasesV2Reconciler) mysqlResourceReconcile(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desired *v1.PerconaXtraDBCluster

	if desired, err = translatorv2.ToMySQLVendorSpec(ctx, wandb.Spec.MySQL, wandb, r.Scheme); err != nil {
		return err
	}
	if err = percona.CrudResource(ctx, r.Client, translatorv2.MysqlNamespacedName(wandb.Spec.MySQL), desired); err != nil {
		return err
	}

	//wandb.Status.MySQLStatus = translatorv2.ExtractMySQLStatus(ctx, results)
	//if err = r.Status().Update(ctx, wandb); err != nil {
	//	results.AddErrors(err)
	//}

	return nil
}
