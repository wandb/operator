package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/mysql/mariadb"
	"github.com/wandb/operator/internal/controller/infra/mysql/mysql"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/vendored/mariadb-operator/k8s.mariadb.com/v1alpha1"
	mysqlv2 "github.com/wandb/operator/pkg/vendored/mysql-operator/v2"
	"github.com/wandb/operator/pkg/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func mysqlWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.MySQL.ManagedMysql
	if spec == nil {
		return nil
	}

	var specNamespacedName = managedMysqlSpecNamespacedName(spec)
	logger := ctrl.LoggerFrom(ctx)

	dbPasswordSecret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", specNamespacedName.Name, "db-password"), Namespace: specNamespacedName.Namespace}, dbPasswordSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			dbPasswordSecret.Name = fmt.Sprintf("%s-%s", specNamespacedName.Name, "db-password")
			dbPasswordSecret.Namespace = specNamespacedName.Namespace
			userPassword, err := utils.GenerateRandomPassword(32)
			if err != nil {
				logger.Error(err, "failed to generate random password")
				return []metav1.Condition{
					{
						Type:   common.ReconciledType,
						Status: metav1.ConditionFalse,
						Reason: common.ControllerErrorReason,
					},
				}
			}
			rootPassword, err := utils.GenerateRandomPassword(32)
			if err != nil {
				logger.Error(err, "failed to generate random password")
				return []metav1.Condition{
					{
						Type:   common.ReconciledType,
						Status: metav1.ConditionFalse,
						Reason: common.ControllerErrorReason,
					},
				}
			}

			dbPasswordSecret.Labels = translatorv2.BuildWandbMysqlLabels(wandb)
			dbPasswordSecret.Data = map[string][]byte{
				"rootUser":     []byte("root"),
				"rootPassword": []byte(rootPassword),
				"rootHost":     []byte("%"),
				"password":     []byte(userPassword),
			}
			if err = client.Create(ctx, dbPasswordSecret); err != nil {
				logger.Error(err, "failed to create db password secret")
				return []metav1.Condition{
					{
						Type:   common.ReconciledType,
						Status: metav1.ConditionFalse,
						Reason: common.ApiErrorReason,
					},
				}
			}
		} else {
			logger.Error(err, "failed to retrieve db password secret")
			return []metav1.Condition{
				{
					Type:   common.ReconciledType,
					Status: metav1.ConditionFalse,
					Reason: common.ApiErrorReason,
				},
			}
		}
	}

	if spec.DeploymentType == apiv2.MySQLTypeMysql {
		if conditions := mysql.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
			return conditions
		}
	}

	switch spec.DeploymentType {
	case apiv2.MySQLTypePercona:
		var desired *v1.PerconaXtraDBCluster
		desired, err = translatorv2.ToPerconaMySQLVendorSpec(ctx, wandb, client.Scheme())
		if err != nil {
			logger.Error(err, "failed to translate mysql spec")
			return []metav1.Condition{
				{
					Type:   common.ReconciledType,
					Status: metav1.ConditionFalse,
					Reason: common.ControllerErrorReason,
				},
			}
		}

		results := percona.WriteState(ctx, client, specNamespacedName, desired)
		return results
	case apiv2.MySQLTypeMariadb:
		var desired *v1alpha1.MariaDB
		desired, err = translatorv2.ToMariaDBMySQLVendorSpec(ctx, *spec, wandb, client.Scheme())
		if err != nil {
			logger.Error(err, "failed to translate mysql spec")
			return []metav1.Condition{
				{
					Type:   common.ReconciledType,
					Status: metav1.ConditionFalse,
					Reason: common.ControllerErrorReason,
				},
			}
		}
		return mariadb.WriteState(ctx, client, specNamespacedName, desired)
	case apiv2.MySQLTypeMysql:
		var desired *mysqlv2.InnoDBCluster
		desired, err = translatorv2.ToMysqlMySQLVendorSpec(ctx, *spec, wandb, client.Scheme())
		if err != nil {
			logger.Error(err, "failed to translate mysql spec")
			return []metav1.Condition{
				{
					Type:   common.ReconciledType,
					Status: metav1.ConditionFalse,
					Reason: common.ControllerErrorReason,
				},
			}
		}
		return mysql.WriteState(ctx, client, specNamespacedName, desired, translatorv2.BuildWandbMysqlLabels(wandb))
	}
	return nil
}

func mysqlReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.InfraConnection) {
	spec := wandb.Spec.MySQL.ManagedMysql
	if spec == nil {
		return newConditions, nil
	}

	specNamespacedName := managedMysqlSpecNamespacedName(spec)

	var readConditions []metav1.Condition
	var newInfraConn *translator.InfraConnection

	switch spec.DeploymentType {
	case apiv2.MySQLTypeMariadb:
		readConditions, newInfraConn = mariadb.ReadState(ctx, client, specNamespacedName, wandb)
	case apiv2.MySQLTypeMysql:
		readConditions, newInfraConn = mysql.ReadState(ctx, client, specNamespacedName, wandb, translatorv2.ToMysqlOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec)))
	case apiv2.MySQLTypePercona:
		readConditions, newInfraConn = percona.ReadState(ctx, client, specNamespacedName, wandb)
	}

	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func mysqlInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.InfraConnection,
) (ctrl.Result, error) {
	spec := wandb.Spec.MySQL.ManagedMysql
	if spec == nil {
		return ctrl.Result{}, nil
	}

	enabled := true
	oldConditions := wandb.Status.MySQLStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorInfraConnection(wandb.Status.MySQLStatus.Connection)

	var updatedStatus translator.InfraStatus
	var events []corev1.Event
	var ctrlResult ctrl.Result

	fmt.Println(spec.DeploymentType)
	switch spec.DeploymentType {
	case apiv2.MySQLTypeMariadb:
		updatedStatus, events, ctrlResult = mariadb.ComputeStatus(
			ctx,
			enabled,
			oldConditions,
			newConditions,
			utils.Coalesce(newInfraConn, &oldInfraConn),
			wandb.Generation,
		)
	case apiv2.MySQLTypeMysql:
		updatedStatus, events, ctrlResult = mysql.ComputeStatus(
			ctx,
			enabled,
			oldConditions,
			newConditions,
			utils.Coalesce(newInfraConn, &oldInfraConn),
			wandb.Generation,
		)
	case apiv2.MySQLTypePercona:
		updatedStatus, events, ctrlResult = percona.ComputeStatus(
			ctx,
			enabled,
			oldConditions,
			newConditions,
			utils.Coalesce(newInfraConn, &oldInfraConn),
			wandb.Generation,
		)
	}

	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.MySQLStatus = translatorv2.ToWbInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

func mysqlPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.MySQL.ManagedMysql
	if spec == nil {
		return nil
	}
	specNamespacedName := managedMysqlSpecNamespacedName(spec)
	onDeleteRule := translatorv2.ToMysqlOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	return mysql.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
}

func mysqlDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.MySQL.ManagedMysql
	if spec == nil {
		return nil
	}
	specNamespacedName := managedMysqlSpecNamespacedName(spec)
	switch spec.DeploymentType {
	case apiv2.MySQLTypeMariadb:
		return mariadb.DetachFinalizer(ctx, client, specNamespacedName, wandb)
	case apiv2.MySQLTypePercona:
		return percona.DetachFinalizer(ctx, client, specNamespacedName, wandb)
	default:
		return mysql.DetachFinalizer(ctx, client, specNamespacedName, wandb)
	}
}

func managedMysqlSpecNamespacedName(spec *apiv2.ManagedMysqlSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
