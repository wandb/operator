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
	"github.com/wandb/operator/internal/controller/infra/mysqlconnection"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
) (map[string][]metav1.Condition, error) {
	if err := reconcileStaleManagedMySQL(ctx, client, wandb); err != nil {
		return nil, err
	}
	desiredInstances := make(map[string]struct{}, len(wandb.Spec.MySQL))
	for key := range wandb.Spec.MySQL {
		desiredInstances[key] = struct{}{}
	}
	if err := mysqlconnection.DeleteStale(ctx, client, wandb, desiredInstances); err != nil {
		return nil, err
	}
	out := map[string][]metav1.Condition{}
	for key, spec := range wandb.Spec.MySQL {
		switch {
		case spec.ManagedMysql != nil:
			out[key] = managedMysqlWriteState(ctx, client, wandb, key, spec.ManagedMysql, mfst)
		case spec.ExternalMysql != nil:
			out[key] = externalmysql.WriteState(ctx, client, wandb, key, spec.ExternalMysql)
		}
	}
	return out, nil
}

func reconcileStaleManagedMySQL(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) error {
	desired := map[string]types.NamespacedName{}
	for key, spec := range wandb.Spec.MySQL {
		if spec.ManagedMysql == nil {
			continue
		}
		desired[mysqlconnection.InstanceID(key)] = managedMysqlSpecNamespacedName(spec.ManagedMysql)
	}

	clusters := &mocov1beta2.MySQLClusterList{}
	if err := c.List(ctx, clusters, client.MatchingLabels(common.BuildWandbLabels(wandb, moco.MysqlModuleName))); err != nil {
		return err
	}
	for i := range clusters.Items {
		cluster := &clusters.Items[i]
		if cluster.Annotations[moco.DetachedAnnotation] == "true" {
			continue
		}
		ownerMatches := !common.IsDetached(cluster, wandb.UID)
		labelUID := cluster.Labels[moco.WandbUIDLabel]
		if labelUID != "" && labelUID != string(wandb.UID) {
			continue
		}
		if labelUID == "" && !ownerMatches {
			continue
		}

		instanceID := cluster.Labels[moco.InstanceLabel]
		clusterName := client.ObjectKeyFromObject(cluster)
		if desiredName, ok := desired[instanceID]; ok && desiredName == clusterName {
			continue
		}
		if instanceID == "" {
			matchedLegacyResource := false
			for _, desiredName := range desired {
				if desiredName == clusterName {
					matchedLegacyResource = true
					break
				}
			}
			if matchedLegacyResource {
				continue
			}
		}

		policy := apiv2.OnDeletePolicy(cluster.Annotations[moco.RetentionPolicyAnnotation])
		if policy == "" {
			policy = wandb.Spec.RetentionPolicy.OnDelete
		}
		if policy == apiv2.PurgeOnDelete {
			selectorLabels := common.BuildWandbLabels(wandb, moco.MysqlModuleName)
			if labelUID != "" {
				selectorLabels[moco.WandbUIDLabel] = labelUID
			}
			if instanceID != "" {
				selectorLabels[moco.InstanceLabel] = instanceID
			}
			if err := moco.PurgeFinalizer(ctx, c, clusterName, common.OnDeleteRule{
				Policy:   common.Purge,
				Selector: labels.SelectorFromSet(selectorLabels),
			}); err != nil {
				return err
			}
			continue
		}
		if err := moco.DetachFinalizer(ctx, c, clusterName, wandb); err != nil {
			return err
		}
	}
	return nil
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
			outConds[key], outConns[key] = managedMysqlReadState(ctx, client, wandb, key, spec.ManagedMysql, conditions[key])
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
		onDeleteRule := moco.ToMysqlOnDeleteRule(wandb, key, wandb.GetRetentionPolicy(managed.ManagedInfraSpec))
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
	key string,
	spec *apiv2.ManagedMysqlSpec,
	mfst manifest.Manifest,
) []metav1.Condition {
	var specNamespacedName = managedMysqlSpecNamespacedName(spec)
	logger := ctrl.LoggerFrom(ctx)

	if conditions := moco.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
		return append(conditions, metav1.Condition{
			Type:   mysqlconnection.ProviderReadyType,
			Status: metav1.ConditionFalse,
			Reason: "Detached",
		})
	}

	var desired *mocov1beta2.MySQLCluster
	var confMap *corev1.ConfigMap
	desired, confMap, err := moco.ToMocoMySQLClusterSpec(ctx, key, *spec, wandb, client.Scheme(), mfst)
	if err != nil {
		logger.Error(err, "failed to translate moco spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
			{
				Type:   mysqlconnection.ProviderReadyType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}
	conditions := moco.WriteState(ctx, client, specNamespacedName, desired, confMap, moco.BuildWandbMysqlLabels(wandb, key))
	providerCondition := metav1.Condition{
		Type:   mysqlconnection.ProviderReadyType,
		Status: metav1.ConditionFalse,
		Reason: "Provisioning",
	}
	for _, condition := range conditions {
		if condition.Type == moco.MySQLCustomResourceType {
			providerCondition.Status = condition.Status
			providerCondition.Reason = condition.Reason
		}
		if condition.Type == common.ReconciledType && condition.Status == metav1.ConditionFalse {
			providerCondition.Status = metav1.ConditionFalse
			providerCondition.Reason = condition.Reason
		}
	}
	return append(conditions, providerCondition)
}

func managedMysqlReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec *apiv2.ManagedMysqlSpec,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.MysqlConnection) {
	specNamespacedName := managedMysqlSpecNamespacedName(spec)

	readConditions, material := moco.ReadState(ctx, client, specNamespacedName, moco.ToMysqlOnDeleteRule(wandb, key, wandb.GetRetentionPolicy(spec.ManagedInfraSpec)))
	newConditions = append(newConditions, readConditions...)
	if material == nil {
		return newConditions, nil
	}

	connection, err := mysqlconnection.Write(ctx, client, wandb, key, *material)
	if err != nil {
		return append(newConditions,
			metav1.Condition{
				Type:    mysqlconnection.BundleReadyType,
				Status:  metav1.ConditionFalse,
				Reason:  common.ApiErrorReason,
				Message: err.Error(),
			},
			metav1.Condition{
				Type:   moco.MySQLConnectionInfoType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
		), nil
	}
	newConditions = append(newConditions,
		metav1.Condition{
			Type:   mysqlconnection.ConnectionResolvedType,
			Status: metav1.ConditionTrue,
			Reason: "MocoCredentialsResolved",
		},
		metav1.Condition{
			Type:   mysqlconnection.BundleReadyType,
			Status: metav1.ConditionTrue,
			Reason: "BundleWritten",
		},
	)
	return newConditions, connection
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
	statusBefore := wandb.DeepCopy().Status
	enabled := true
	oldStatus := wandb.Status.MySQLStatus[key]
	oldConditions := oldStatus.Conditions
	oldInfraConn := oldStatus.Connection
	initStatus := wandb.Status.Wandb.MySQLInit[key]
	initialized := metav1.Condition{
		Type:   mysqlconnection.DatabaseInitializedType,
		Status: metav1.ConditionFalse,
		Reason: "InitializationPending",
	}
	if initStatus.Succeeded {
		initialized.Status = metav1.ConditionTrue
		initialized.Reason = "JobSucceeded"
	} else if initStatus.Failed {
		initialized.Reason = "JobFailed"
	}
	newConditions = append(newConditions, initialized)

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
	err := updateWandbStatusIfChanged(ctx, client, wandb, statusBefore)

	return ctrlResult, err
}

// external

func externalMysqlInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, newConditions []metav1.Condition, newInfraConn *apiv2.MysqlConnection) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	oldStatus := wandb.Status.MySQLStatus[key]
	oldInfraConn := oldStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(oldStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.MySQLStatus[key] = apiv2.MysqlInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, updateWandbStatusIfChanged(ctx, c, wandb, statusBefore)
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

// mysqlManifestConfig returns the manifest infra config for the instance key,
// falling back to the manifest "default" entry.
func mysqlManifestConfig(mfst manifest.Manifest, key string) manifest.InfraConfig {
	cfg, _ := infraSizingConfig(mfst.Mysql, key)
	return cfg
}

func runMysqlInitJob(ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases, mfst manifest.Manifest) (ctrl.Result, error) {
	if wandb.Status.Wandb.MySQLInit == nil {
		wandb.Status.Wandb.MySQLInit = map[string]apiv2.MigrationJobStatus{}
	}

	var results []ctrl.Result
	for key, spec := range wandb.Spec.MySQL {
		if spec.ManagedMysql == nil {
			continue
		}
		res, err := runMysqlInitJobInstance(ctx, client, wandb, key, spec.ManagedMysql, mfst)
		if err != nil {
			return ctrl.Result{}, err
		}
		results = append(results, res)
	}
	return consolidateResults(results), nil
}

func runMysqlInitJobInstance(ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases, key string, spec *apiv2.ManagedMysqlSpec, mfst manifest.Manifest) (ctrl.Result, error) {
	if wandb.Status.Wandb.MySQLInit[key].Succeeded {
		return ctrl.Result{}, nil
	}
	statusBefore := wandb.DeepCopy().Status

	logger := ctrl.LoggerFrom(ctx).WithName("mysqlInit").WithValues("instance", key)

	specNamespacedName := managedMysqlSpecNamespacedName(spec)
	jobName := fmt.Sprintf("%s-moco-init", specNamespacedName.Name)
	logger.Info("Checking for MySQL init job", "job", jobName)
	job := &v1.Job{}
	err := client.Get(ctx, types.NamespacedName{Name: jobName, Namespace: wandb.Namespace}, job)

	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	jobNotFound := errors.IsNotFound(err)
	_, _, bundleChecksum, err := applyMySQLBundlesToWorkload(
		ctx,
		client,
		wandb,
		map[string]struct{}{key: {}},
		nil,
		nil,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !jobNotFound && job.Status.Succeeded == 0 && bundleChecksum != "" &&
		job.Spec.Template.Annotations[mysqlBundlesChecksumAnnotation] != bundleChecksum {
		if err := client.Delete(ctx, job); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if jobNotFound {
		logger.Info("Creating MySQL init job")

		// moco-writable has DDL/DML privileges on all non-system databases,
		// so CREATE DATABASE works. The Oracle-era CREATE USER + GRANT steps
		// are unnecessary — wandb connects directly as the secret's Username.
		connectionStatus, ok := wandb.Status.MySQLStatus[key]
		if !ok || connectionStatus.Connection.URL.Name == "" {
			return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
		}
		connection := connectionStatus.Connection
		selectorOrDefault := func(selector corev1.SecretKeySelector, bundleKey string) corev1.SecretKeySelector {
			if selector.Name != "" && selector.Key != "" {
				return selector
			}
			return corev1.SecretKeySelector{
				LocalObjectReference: connection.URL.LocalObjectReference,
				Key:                  bundleKey,
			}
		}

		envFromConn := func(name string, selector corev1.SecretKeySelector) corev1.EnvVar {
			return corev1.EnvVar{
				Name: name,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &selector,
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
								Image:   moco.MocoMySQLImage(mysqlManifestConfig(mfst, key).Images["mysql"], wandb.Spec.Global.ImageRegistry),
								Command: []string{"mysql"},
								Args: []string{
									"--host=$(MYSQL_HOST)",
									"--port=$(MYSQL_PORT)",
									"--user=$(MYSQL_USER)",
									"--execute=CREATE DATABASE IF NOT EXISTS `wandb_local`;",
								},
								Env: []corev1.EnvVar{
									envFromConn("MYSQL_HOST", selectorOrDefault(connection.Host, mysqlconnection.HostKey)),
									envFromConn("MYSQL_PORT", selectorOrDefault(connection.Port, mysqlconnection.PortKey)),
									envFromConn("MYSQL_USER", selectorOrDefault(connection.Username, mysqlconnection.UsernameKey)),
									envFromConn("MYSQL_PWD", selectorOrDefault(connection.Password, mysqlconnection.PasswordKey)),
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
		setMySQLBundlesChecksumAnnotation(&job.Spec.Template, bundleChecksum)

		if err := client.Create(ctx, job); err != nil {
			return ctrl.Result{}, err
		}

		wandb.Status.Wandb.MySQLInit[key] = apiv2.MigrationJobStatus{
			Name:   jobName,
			Phase:  migrationPhaseRunning,
			Reason: "JobCreated",
		}
		if err := updateWandbStatusIfChanged(ctx, client, wandb, statusBefore); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if job.Status.Succeeded > 0 {
		logger.Info("MySQL init job succeeded")
		wandb.Status.Wandb.MySQLInit[key] = apiv2.MigrationJobStatus{
			Name:      jobName,
			Succeeded: true,
			Phase:     migrationPhaseSucceeded,
			Reason:    "JobSucceeded",
		}
		if err := updateWandbStatusIfChanged(ctx, client, wandb, statusBefore); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		logger.Info("MySQL init job failed")
		wandb.Status.Wandb.MySQLInit[key] = apiv2.MigrationJobStatus{
			Name:   jobName,
			Failed: true,
			Phase:  migrationPhaseFailed,
			Reason: "JobFailed",
		}
		if err := updateWandbStatusIfChanged(ctx, client, wandb, statusBefore); err != nil {
			return ctrl.Result{}, err
		}
		// We might want to return an error or just requeue
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	logger.Info("MySQL init job still running")
	return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
}
