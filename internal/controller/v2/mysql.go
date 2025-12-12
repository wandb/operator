package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	v1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	logger := ctrl.LoggerFrom(ctx)
	// TODO Move this secret creation to the correct spot
	dbPasswordSecret := &corev1.Secret{}
	err = client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", specNamespacedName.Name, "user-db-password"), Namespace: specNamespacedName.Namespace}, dbPasswordSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			dbPasswordSecret.Name = fmt.Sprintf("%s-%s", specNamespacedName.Name, "user-db-password")
			dbPasswordSecret.Namespace = specNamespacedName.Namespace
			password, err := utils.GenerateRandomPassword(32)
			if err != nil {
				logger.Error(err, "failed to generate random password")
				return err
			}
			fmt.Printf("Password write: %s\n", password)
			dbPasswordSecret.Data = map[string][]byte{
				"password": []byte(password),
			}
			if err = client.Create(ctx, dbPasswordSecret); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to get secret: %w", err)
		}
	}

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
	var status *translator.MysqlStatus
	var specNamespacedName = mysqlSpecNamespacedName(wandb.Spec.MySQL)

	if status, err = percona.ReadState(ctx, client, specNamespacedName, wandb); err != nil {
		return err
	}
	if status != nil {
		wandb.Status.MySQLStatus = translatorv2.ToWBMysqlStatus(ctx, *status)
		if err = client.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "failed to update status")
			return err
		}
	}

	return nil
}

func mysqlSpecNamespacedName(mysql apiv2.WBMySQLSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: mysql.Namespace,
		Name:      mysql.Name,
	}
}
