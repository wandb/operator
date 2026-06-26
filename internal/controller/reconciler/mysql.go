package reconciler

import (
	"context"
	"fmt"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	externalmysql "github.com/wandb/operator/internal/controller/infra/external/mysql"
	"github.com/wandb/operator/internal/controller/infra/managed/mysql/moco"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func mysqlWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	mfst manifest.Manifest,
) []metav1.Condition {
	if wandb.Spec.MySQL.ManagedMysql != nil {
		return managedMysqlWriteState(ctx, client, wandb, mfst)
	}
	if wandb.Spec.MySQL.ExternalMysql != nil {
		return externalMysqlWriteState(ctx, client, wandb)
	}
	return nil
}

func mysqlReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.MysqlConnection) {
	if wandb.Spec.MySQL.ManagedMysql != nil {
		return managedMysqlReadState(ctx, client, wandb, newConditions)
	}
	if wandb.Spec.MySQL.ExternalMysql != nil {
		return externalMysqlReadState(ctx, client, wandb, newConditions)
	}
	return newConditions, nil
}

func mysqlInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.MysqlConnection,
) (ctrl.Result, error) {
	if wandb.Spec.MySQL.ManagedMysql != nil {
		return managedMysqlInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
	}
	if wandb.Spec.MySQL.ExternalMysql != nil {
		return externalMysqlInferStatus(ctx, client, wandb, newConditions, newInfraConn)
	}
	return ctrl.Result{}, nil
}

func mysqlPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	if spec := wandb.Spec.MySQL.ManagedMysql; spec != nil {
		specNamespacedName := managedMysqlSpecNamespacedName(spec)
		onDeleteRule := moco.ToMysqlOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return moco.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if wandb.Spec.MySQL.ExternalMysql != nil {
		return externalmysql.DeleteConnectionSecret(ctx, client, wandb)
	}
	return nil
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
	return moco.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedMysqlWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	mfst manifest.Manifest,
) []metav1.Condition {
	spec := wandb.Spec.MySQL.ManagedMysql

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

			dbPasswordSecret.Labels = moco.BuildWandbMysqlLabels(wandb)
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

	if conditions := moco.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
		return conditions
	}

	var desired *mocov1beta2.MySQLCluster
	var confMap *corev1.ConfigMap
	desired, confMap, err = moco.ToMocoMySQLClusterSpec(ctx, *spec, wandb, client.Scheme(), mfst)
	if err != nil {
		logger.Error(err, "failed to translate moco spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}
	return moco.WriteState(ctx, client, specNamespacedName, desired, confMap, moco.BuildWandbMysqlLabels(wandb))
}

func managedMysqlReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.MysqlConnection) {
	spec := wandb.Spec.MySQL.ManagedMysql
	specNamespacedName := managedMysqlSpecNamespacedName(spec)

	readConditions, newInfraConn := moco.ReadState(ctx, client, specNamespacedName, wandb, moco.ToMysqlOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec)))
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func managedMysqlInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.MysqlConnection,
) (ctrl.Result, error) {
	enabled := true
	oldConditions := wandb.Status.MySQLStatus.Conditions
	oldInfraConn := wandb.Status.MySQLStatus.Connection

	updatedStatus, events, ctrlResult := moco.ComputeStatus(
		ctx,
		enabled,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)

	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.MySQLStatus = updatedStatus
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalMysqlWriteState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) []metav1.Condition {
	return externalmysql.WriteState(ctx, c, wandb)
}

func externalMysqlReadState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition) ([]metav1.Condition, *apiv2.MysqlConnection) {
	return externalmysql.ReadState(ctx, c, wandb, newConditions)
}

func externalMysqlInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition, newInfraConn *apiv2.MysqlConnection) (ctrl.Result, error) {
	oldInfraConn := wandb.Status.MySQLStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(wandb.Status.MySQLStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.MySQLStatus = apiv2.MysqlInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, c.Status().Update(ctx, wandb)
}

// helpers

func managedMysqlSpecNamespacedName(spec *apiv2.ManagedMysqlSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}

func runMysqlInitJob(ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases, manifest manifest.Manifest) (ctrl.Result, error) {
	if wandb.Spec.MySQL.ManagedMysql == nil {
		return ctrl.Result{}, nil
	}

	if wandb.Status.Wandb.MySQLInit.Succeeded {
		return ctrl.Result{}, nil
	}

	logger := ctrl.LoggerFrom(ctx).WithName("mysqlInit")

	jobName := fmt.Sprintf("%s-moco-init", wandb.Name)
	logger.Info("Checking for MySQL init job", "job", jobName)
	job := &v1.Job{}
	err := client.Get(ctx, types.NamespacedName{Name: jobName, Namespace: wandb.Namespace}, job)

	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if errors.IsNotFound(err) {
		logger.Info("Creating MySQL init job")

		specNamespacedName := managedMysqlSpecNamespacedName(wandb.Spec.MySQL.ManagedMysql)
		connSecretName := fmt.Sprintf("%s-connection", specNamespacedName.Name)

		// moco-writable has DDL/DML privileges on all non-system databases,
		// so CREATE DATABASE works. The Oracle-era CREATE USER + GRANT steps
		// are unnecessary — wandb connects directly as the secret's Username.
		mysqlCmd := `mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PWD" ` +
			`-e "CREATE DATABASE IF NOT EXISTS $MYSQL_DB;"`

		envFromConn := func(name, key string) corev1.EnvVar {
			return corev1.EnvVar{
				Name: name,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: connSecretName},
						Key:                  key,
					},
				},
			}
		}

		job = &v1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: wandb.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "wandb-operator",
					"app.kubernetes.io/instance":   wandb.Name,
					"app.kubernetes.io/component":  "moco-init",
				},
			},
			Spec: v1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyOnFailure,
						Containers: []corev1.Container{
							{
								Name:    "moco-init",
								Image:   common.ApplyImageRegistry(moco.MocoMySQLImage, wandb.Spec.Global.ImageRegistry),
								Command: []string{"/bin/sh", "-c", mysqlCmd},
								Env: []corev1.EnvVar{
									envFromConn("MYSQL_HOST", "Host"),
									envFromConn("MYSQL_PORT", "Port"),
									envFromConn("MYSQL_USER", "Username"),
									envFromConn("MYSQL_PWD", "Password"),
									envFromConn("MYSQL_DB", "Database"),
								},
							},
						},
					},
				},
			},
		}

		if err := controllerutil.SetOwnerReference(wandb, job, client.Scheme()); err != nil {
			return ctrl.Result{}, err
		}

		if err := client.Create(ctx, job); err != nil {
			return ctrl.Result{}, err
		}

		wandb.Status.Wandb.MySQLInit.Name = jobName
		wandb.Status.Wandb.MySQLInit.Succeeded = false
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if job.Status.Succeeded > 0 {
		logger.Info("MySQL init job succeeded")
		wandb.Status.Wandb.MySQLInit.Succeeded = true
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		logger.Info("MySQL init job failed")
		wandb.Status.Wandb.MySQLInit.Failed = true
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}
		// We might want to return an error or just requeue
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	logger.Info("MySQL init job still running")
	return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
}
