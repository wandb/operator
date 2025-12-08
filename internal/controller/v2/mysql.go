package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	v1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func mysqlWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desired *v1.PerconaXtraDBCluster
	var specNamespacedName = mysqlSpecNamespacedName(wandb.Spec.MySQL)

	if desired, err = translatorv2.ToMySQLVendorSpec(ctx, wandb.Spec.MySQL, wandb, client.Scheme()); err != nil {
		return err
	}
	if err = percona.WriteState(ctx, client, specNamespacedName, desired); err != nil {
		return err
	}

	return nil
}

func mysqlReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var conditions []common.MySQLCondition
	var specNamespacedName = mysqlSpecNamespacedName(wandb.Spec.MySQL)

	if conditions, err = percona.ReadState(ctx, client, specNamespacedName, wandb); err != nil {
		return err
	}
	wandb.Status.MySQLStatus = translatorv2.ToMySQLStatus(ctx, conditions)
	if err = client.Status().Update(ctx, wandb); err != nil {
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
