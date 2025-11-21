package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	v1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *WeightsAndBiasesV2Reconciler) mysqlResourceReconcile(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desired *v1.PerconaXtraDBCluster
	var specNamespacedName = mysqlSpecNamespacedName(wandb.Spec.MySQL)

	if desired, err = translatorv2.ToMySQLVendorSpec(ctx, wandb.Spec.MySQL, wandb, r.Scheme); err != nil {
		return err
	}
	if err = percona.CrudResource(ctx, r.Client, specNamespacedName, desired); err != nil {
		return err
	}

	//wandb.Status.MySQLStatus = translatorv2.ExtractMySQLStatus(ctx, results)
	//if err = r.Status().Update(ctx, wandb); err != nil {
	//	results.AddErrors(err)
	//}

	return nil
}

func (r *WeightsAndBiasesV2Reconciler) mysqlStatusUpdate(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var conditions []common.MySQLCondition
	var specNamespacedName = mysqlSpecNamespacedName(wandb.Spec.MySQL)

	if conditions, err = percona.GetConditions(ctx, r.Client, specNamespacedName); err != nil {
		return err
	}
	wandb.Status.MySQLStatus = translatorv2.ExtractMySQLStatus(ctx, conditions)
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "failed to update status")
		return err
	}

	return nil
}

func mysqlSpecNamespacedName(mysql apiv2.WBMySQLSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: mysql.Namespace,
		Name:      mysql.Name,
	}
}
