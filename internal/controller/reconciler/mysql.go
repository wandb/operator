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
) map[string][]metav1.Condition {
	out := map[string][]metav1.Condition{}
	for key, spec := range wandb.Spec.MySQL {
		switch {
		case spec.ManagedMysql != nil:
			out[key] = managedMysqlWriteState(ctx, client, wandb, spec.ManagedMysql)
		case spec.ExternalMysql != nil:
			out[key] = externalmysql.WriteState(ctx, client, wandb, key, spec.ExternalMysql)
		}
	}
	return out
}

func mysqlReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	conditions map[string][]metav1.Condition,
) (map[string][]metav1.Condition, map[string]*apiv2.MysqlConnection) {
	outConds := map[string][]metav1.Condition{}
	outConns := map[string]*apiv2.MysqlConnection{}
	for key, spec := range wandb.Spec.MySQL {
		switch {
		case spec.ManagedMysql != nil:
			outConds[key], outConns[key] = managedMysqlReadState(ctx, client, wandb, spec.ManagedMysql, conditions[key])
		case spec.ExternalMysql != nil:
			outConds[key], outConns[key] = externalmysql.ReadState(ctx, client, wandb, key, conditions[key])
		default:
			outConds[key] = conditions[key]
		}
	}
	return outConds, outConns
}

func mysqlInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	conditions map[string][]metav1.Condition,
	infraConns map[string]*apiv2.MysqlConnection,
) (ctrl.Result, error) {
	if wandb.Status.MySQLStatus == nil {
		wandb.Status.MySQLStatus = map[string]apiv2.MysqlInfraStatus{}
	}
	var results []ctrl.Result
	var firstErr error
	for key, spec := range wandb.Spec.MySQL {
		var res ctrl.Result
		var err error
		switch {
		case spec.ManagedMysql != nil:
			res, err = managedMysqlInferStatus(ctx, client, recorder, wandb, key, conditions[key], infraConns[key])
		case spec.ExternalMysql != nil:
			res, err = externalMysqlInferStatus(ctx, client, wandb, key, conditions[key], infraConns[key])
		}
		results = append(results, res)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return consolidateResults(results), firstErr
}

// runMysqlRetentionFinalizer applies the configured retention policy for a
// single MySQL instance during deletion.
func runMysqlRetentionFinalizer(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, spec apiv2.MySQLSpec) error {
	switch wandb.GetRetentionPolicy(mysqlInstanceInfraSpec(spec)).OnDelete {
	case apiv2.PurgeOnDelete:
		return mysqlPurgeFinalizer(ctx, c, wandb, key, spec)
	case apiv2.DetachOnDelete:
		return mysqlDetachFinalizer(ctx, c, wandb, key, spec)
	}
	return nil
}

func mysqlInstanceInfraSpec(spec apiv2.MySQLSpec) apiv2.ManagedInfraSpec {
	if spec.ManagedMysql != nil {
		return spec.ManagedMysql.ManagedInfraSpec
	}
	return apiv2.ManagedInfraSpec{}
}

func mysqlPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec apiv2.MySQLSpec,
) error {
	if managed := spec.ManagedMysql; managed != nil {
		specNamespacedName := managedMysqlSpecNamespacedName(managed)
		onDeleteRule := moco.ToMysqlOnDeleteRule(wandb, wandb.GetRetentionPolicy(managed.ManagedInfraSpec))
		return moco.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if spec.ExternalMysql != nil {
		return externalmysql.DeleteConnectionSecret(ctx, client, wandb, key)
	}
	return nil
}

func mysqlDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	_ string,
	spec apiv2.MySQLSpec,
) error {
	managed := spec.ManagedMysql
	if managed == nil {
		return nil
	}
	specNamespacedName := managedMysqlSpecNamespacedName(managed)
	return moco.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedMysqlWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	spec *apiv2.ManagedMysqlSpec,
) []metav1.Condition {
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
	desired, confMap, err = moco.ToMocoMySQLClusterSpec(ctx, *spec, wandb, client.Scheme())
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
	spec *apiv2.ManagedMysqlSpec,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.MysqlConnection) {
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
	key string,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.MysqlConnection,
) (ctrl.Result, error) {
	enabled := true
	oldStatus := wandb.Status.MySQLStatus[key]
	oldConditions := oldStatus.Conditions
	oldInfraConn := oldStatus.Connection

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
	wandb.Status.MySQLStatus[key] = updatedStatus
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalMysqlInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, newConditions []metav1.Condition, newInfraConn *apiv2.MysqlConnection) (ctrl.Result, error) {
	oldStatus := wandb.Status.MySQLStatus[key]
	oldInfraConn := oldStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(oldStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.MySQLStatus[key] = apiv2.MysqlInfraStatus{
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

// allMysqlInitSucceeded reports whether every managed MySQL instance has a
// successful database-initialization job.
func allMysqlInitSucceeded(wandb *apiv2.WeightsAndBiases) bool {
	for key, spec := range wandb.Spec.MySQL {
		if spec.ManagedMysql == nil {
			continue
		}
		if !wandb.Status.Wandb.MySQLInit[key].Succeeded {
			return false
		}
	}
	return true
}

func runMysqlInitJob(ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases, manifest manifest.Manifest) (ctrl.Result, error) {
	if wandb.Status.Wandb.MySQLInit == nil {
		wandb.Status.Wandb.MySQLInit = map[string]apiv2.MigrationJobStatus{}
	}

	var results []ctrl.Result
	for key, spec := range wandb.Spec.MySQL {
		if spec.ManagedMysql == nil {
			continue
		}
		res, err := runMysqlInitJobInstance(ctx, client, wandb, key, spec.ManagedMysql)
		if err != nil {
			return ctrl.Result{}, err
		}
		results = append(results, res)
	}
	return consolidateResults(results), nil
}

func runMysqlInitJobInstance(ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases, key string, spec *apiv2.ManagedMysqlSpec) (ctrl.Result, error) {
	if wandb.Status.Wandb.MySQLInit[key].Succeeded {
		return ctrl.Result{}, nil
	}

	logger := ctrl.LoggerFrom(ctx).WithName("mysqlInit").WithValues("instance", key)

	specNamespacedName := managedMysqlSpecNamespacedName(spec)
	jobName := fmt.Sprintf("%s-moco-init", specNamespacedName.Name)
	logger.Info("Checking for MySQL init job", "job", jobName)
	job := &v1.Job{}
	err := client.Get(ctx, types.NamespacedName{Name: jobName, Namespace: wandb.Namespace}, job)

	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if errors.IsNotFound(err) {
		logger.Info("Creating MySQL init job")

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
								Image:   "ghcr.io/cybozu-go/moco/mysql:8.4.8",
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

		wandb.Status.Wandb.MySQLInit[key] = apiv2.MigrationJobStatus{Name: jobName, Succeeded: false}
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if job.Status.Succeeded > 0 {
		logger.Info("MySQL init job succeeded")
		wandb.Status.Wandb.MySQLInit[key] = apiv2.MigrationJobStatus{Name: jobName, Succeeded: true}
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		logger.Info("MySQL init job failed")
		wandb.Status.Wandb.MySQLInit[key] = apiv2.MigrationJobStatus{Name: jobName, Failed: true}
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}
		// We might want to return an error or just requeue
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	logger.Info("MySQL init job still running")
	return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
}
