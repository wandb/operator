package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/internal/utils"
	v1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	pkgutils "github.com/wandb/operator/pkg/utils"
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
	var specNamespacedName = mysqlSpecNamespacedName(wandb.Spec.MySQL)
	logger := ctrl.LoggerFrom(ctx)

	dbPasswordSecret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s-%s", specNamespacedName.Name, "user-db-password"), Namespace: specNamespacedName.Namespace}, dbPasswordSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			dbPasswordSecret.Name = fmt.Sprintf("%s-%s", specNamespacedName.Name, "user-db-password")
			dbPasswordSecret.Namespace = specNamespacedName.Namespace
			password, err := pkgutils.GenerateRandomPassword(32)
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

			dbPasswordSecret.Data = map[string][]byte{
				"password": []byte(password),
			}
			if err = client.Create(ctx, dbPasswordSecret); err != nil {
				return []metav1.Condition{
					{
						Type:   common.ReconciledType,
						Status: metav1.ConditionFalse,
						Reason: common.ApiErrorReason,
					},
				}
			}
		} else {
			return []metav1.Condition{
				{
					Type:   common.ReconciledType,
					Status: metav1.ConditionFalse,
					Reason: common.ApiErrorReason,
				},
			}
		}
	}

	if wandb.Spec.MySQL.Affinity == nil {
		wandb.Spec.MySQL.Affinity = wandb.Spec.Affinity
	}
	if wandb.Spec.MySQL.Tolerations == nil {
		wandb.Spec.MySQL.Tolerations = wandb.Spec.Tolerations
	}

	var desired *v1.PerconaXtraDBCluster
	desired, err = translatorv2.ToMySQLVendorSpec(ctx, wandb.Spec.MySQL, wandb, client.Scheme())
	if err != nil {
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
}

func mysqlReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.InfraConnection) {
	specNamespacedName := mysqlSpecNamespacedName(wandb.Spec.MySQL)
	readConditions, newInfraConn := percona.ReadState(ctx, client, specNamespacedName, wandb)
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
	oldConditions := wandb.Status.MySQLStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorInfraConnection(wandb.Status.MySQLStatus.Connection)

	updatedStatus, events, ctrlResult := percona.ComputeStatus(
		ctx,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.MySQLStatus = translatorv2.ToWbInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

func mysqlSpecNamespacedName(mysql apiv2.WBMySQLSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: mysql.Namespace,
		Name:      mysql.Name,
	}
}
