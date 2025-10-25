package wandb_v2

import (
	"context"
	"errors"
	"time"

	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	corev1 "k8s.io/api/core/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

var defaultDbRequeueSeconds = 30
var defaultDbRequeueDuration = time.Duration(defaultDbRequeueSeconds) * time.Second

type wandbPerconaMysqlWrapper struct {
	installed       bool
	obj             *pxcv1.PerconaXtraDBCluster
	secretInstalled bool
	secret          *corev1.Secret
}

func (w *wandbPerconaMysqlWrapper) IsReady() bool {
	if !w.installed || w.obj == nil {
		return false
	}
	return w.obj.Status.Ready == w.obj.Status.Size
}

func (w *wandbPerconaMysqlWrapper) GetStatus() string {
	if !w.installed || w.obj == nil {
		return "NotInstalled"
	}
	return string(w.obj.Status.Status)
}

type wandbPerconaMysqlDoReconcile interface {
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState
}

func perconaMysqlNamespacedName(req ctrl.Request) types.NamespacedName {
	return types.NamespacedName{
		Name:      "wandb-percona-mysql",
		Namespace: req.Namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handlePerconaMysql(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var desiredPercona wandbPerconaMysqlWrapper
	var actualPercona wandbPerconaMysqlWrapper
	var reconciliation wandbPerconaMysqlDoReconcile
	var namespacedName = perconaMysqlNamespacedName(req)

	if actualPercona, err = actualPerconaMysql(ctx, r, namespacedName); err != nil {
		return CtrlError(err)
	}

	if ctrlState := actualPercona.maybeHandleDeletion(ctx, wandb, actualPercona, r); ctrlState.shouldExit(HandlerScope) {
		return ctrlState
	}

	if desiredPercona, err = desiredPerconaMysql(ctx, wandb, namespacedName, r, actualPercona); err != nil {
		return CtrlError(err)
	}

	if reconciliation, err = computePerconaReconcileDrift(ctx, wandb, desiredPercona, actualPercona); err != nil {
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func actualPerconaMysql(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbPerconaMysqlWrapper, error,
) {
	result := wandbPerconaMysqlWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}
	obj := &pxcv1.PerconaXtraDBCluster{}
	err := reconciler.Get(ctx, namespacedName, obj)
	if err != nil {
		if machErrors.IsNotFound(err) {
			return result, nil
		}
		return result, err
	}
	result.obj = obj
	result.installed = true

	secretNamespacedName := types.NamespacedName{
		Name:      "wandb-mysql-connection",
		Namespace: namespacedName.Namespace,
	}
	secret := &corev1.Secret{}
	err = reconciler.Get(ctx, secretNamespacedName, secret)
	if err == nil {
		result.secret = secret
		result.secretInstalled = true
	} else if !machErrors.IsNotFound(err) {
		return result, err
	}

	return result, nil
}

func desiredPerconaMysql(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, reconciler *WeightsAndBiasesV2Reconciler, actual wandbPerconaMysqlWrapper,
) (
	wandbPerconaMysqlWrapper, error,
) {
	result := wandbPerconaMysqlWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	if !wandb.Spec.Database.Enabled {
		return result, nil
	}

	result.installed = true

	storageSize := wandb.Spec.Database.StorageSize
	if storageSize == "" {
		storageSize = "20Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}
	tlsEnabled := false
	wandbBackupSpec := wandb.Spec.Database.Backup

	pxc := &pxcv1.PerconaXtraDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: pxcv1.PerconaXtraDBClusterSpec{
			CRVersion:   "1.18.0",
			SecretsName: "wandb-percona-mysql-secrets",
			Unsafe: pxcv1.UnsafeFlags{
				PXCSize:   true,
				TLS:       true,
				ProxySize: true,
			},
			PXC: &pxcv1.PXCSpec{
				PodSpec: &pxcv1.PodSpec{
					Size:  1,
					Image: "perconalab/percona-xtradb-cluster-operator:main-pxc8.0",
					VolumeSpec: &pxcv1.VolumeSpec{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: storageQuantity,
								},
							},
						},
					},
				},
			},
			TLS: &pxcv1.TLSSpec{
				Enabled: &tlsEnabled,
			},
			HAProxy: &pxcv1.HAProxySpec{
				PodSpec: pxcv1.PodSpec{
					Enabled: false,
				},
			},
			LogCollector: &pxcv1.LogCollectorSpec{
				Enabled: true,
				Image:   "perconalab/percona-xtradb-cluster-operator:main-logcollector",
			},
		},
	}

	if wandbBackupSpec.Enabled {
		storageName := "default-backup"
		if wandbBackupSpec.StorageName != "" {
			storageName = wandbBackupSpec.StorageName
		}
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for now for Percona XtraDB Cluster")
		}
		pxc.Spec.Backup = &pxcv1.PXCScheduledBackup{
			Image: "perconalab/percona-xtradb-cluster-operator:main-pxc8.0-backup",
			Storages: map[string]*pxcv1.BackupStorageSpec{
				storageName: {
					Type: pxcv1.BackupStorageFilesystem,
					Volume: &pxcv1.VolumeSpec{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
							AccessModes: wandbBackupSpec.Filesystem.AccessModes,
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: wandbBackupSpec.Filesystem.StorageSize,
								},
							},
						},
					},
				},
			},
		}
	}

	result.obj = pxc

	if actual.IsReady() {
		namespace := namespacedName.Namespace

		sourceSecretName := types.NamespacedName{
			Name:      "wandb-percona-mysql-secrets",
			Namespace: namespace,
		}
		sourceSecret := &corev1.Secret{}
		if err := reconciler.Get(ctx, sourceSecretName, sourceSecret); err != nil {
			return result, err
		}

		mysqlPassword, ok := sourceSecret.Data["root"]
		if !ok {
			return result, errors.New("root key not found in wandb-percona-mysql-secrets")
		}

		mysqlHost := "wandb-percona-mysql-pxc." + namespace + ".svc.cluster.local"
		mysqlPort := "3306"
		mysqlUser := "root"

		connectionSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-mysql-connection",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"MYSQL_HOST":     mysqlHost,
				"MYSQL_PORT":     mysqlPort,
				"MYSQL_USER":     mysqlUser,
				"MYSQL_PASSWORD": string(mysqlPassword),
			},
		}

		result.secret = connectionSecret
		result.secretInstalled = true
	}

	return result, nil
}

func computePerconaReconcileDrift(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, desiredPercona, actualPercona wandbPerconaMysqlWrapper,
) (
	wandbPerconaMysqlDoReconcile, error,
) {
	if !desiredPercona.installed && actualPercona.installed {
		if actualPercona.secretInstalled {
			return &wandbMysqlConnInfoDelete{
				wandb: wandb,
			}, nil
		}
		return &wandbPerconaMysqlDelete{
			actual: actualPercona,
			wandb:  wandb,
		}, nil
	}
	if desiredPercona.installed && !actualPercona.installed {
		return &wandbPerconaMysqlCreate{
			desired: desiredPercona,
			wandb:   wandb,
		}, nil
	}

	if desiredPercona.secretInstalled && !actualPercona.secretInstalled {
		return &wandbMysqlConnInfoCreate{
			desired: desiredPercona,
			wandb:   wandb,
		}, nil
	}

	if actualPercona.GetStatus() != wandb.Status.DatabaseStatus.State ||
		actualPercona.IsReady() != wandb.Status.DatabaseStatus.Ready {
		return &wandbPerconaMysqlStausUpdate{
			wandb:  wandb,
			status: actualPercona.GetStatus(),
			ready:  actualPercona.IsReady(),
		}, nil
	}
	return nil, nil
}

type wandbPerconaMysqlCreate struct {
	desired wandbPerconaMysqlWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbPerconaMysqlCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing Percona XtraDB Cluster")
	wandb := c.wandb
	if err = controllerutil.SetOwnerReference(wandb, c.desired.obj, r.Scheme); err != nil {
		return CtrlError(err)
	}
	if err = r.Create(ctx, c.desired.obj); err != nil {
		return CtrlError(err)
	}
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Creating Database"
	wandb.Status.DatabaseStatus.State = string(pxcv1.AppStateInit)
	if err = r.Status().Update(ctx, wandb); err != nil {
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbPerconaMysqlDelete struct {
	actual wandbPerconaMysqlWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbPerconaMysqlDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling Percona XtraDB Cluster")
	if err = r.Delete(ctx, d.actual.obj); err != nil {
		return CtrlError(err)
	}
	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Deleting Database"
	if err = r.Status().Update(ctx, wandb); err != nil {
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbPerconaMysqlStausUpdate struct {
	wandb  *apiv2.WeightsAndBiases
	status string
	ready  bool
}

func (s *wandbPerconaMysqlStausUpdate) Execute(
	ctx context.Context, r *WeightsAndBiasesV2Reconciler,
) CtrlState {
	s.wandb.Status.DatabaseStatus.State = s.status
	s.wandb.Status.DatabaseStatus.Ready = s.ready
	if err := r.Status().Update(ctx, s.wandb); err != nil {
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

func (w *wandbPerconaMysqlWrapper) maybeHandleDeletion(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, actualPercona wandbPerconaMysqlWrapper, reconciler *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrllog.FromContext(ctx)

	//requeueSeconds := wandb.Status.DatabaseStatus.BackupStatus.RequeueAfter
	//if requeueSeconds == 0 {
	//	requeueSeconds = 30
	//}
	//requeueDuration := time.Duration(requeueSeconds) * time.Second

	var deletionPaused = wandb.Status.State == apiv2.WBStateDeletionPaused
	var backupEnabled = wandb.Spec.Database.Backup.Enabled
	var flaggedForDeletion = !wandb.ObjectMeta.DeletionTimestamp.IsZero()
	var hasDbFinalizer = ctrlqueue.ContainsString(wandb.GetFinalizers(), dbFinalizer)

	if flaggedForDeletion && !backupEnabled {
		log.Info("Database backup is disabled.")
		controllerutil.RemoveFinalizer(wandb, dbFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if deletionPaused && backupEnabled {
		log.Info("Deletion paused for Percona XtraDB Cluster Backup; disable backups to continue with deletion")
		return CtrlContinue()
	}

	if !hasDbFinalizer && !flaggedForDeletion {
		wandb.ObjectMeta.Finalizers = append(wandb.ObjectMeta.Finalizers, dbFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if flaggedForDeletion {
		if err := w.handleDatabaseBackup(ctx, wandb, reconciler); err != nil {
			log.Info("Failed to backup database, pausing deletion")
			wandb.ObjectMeta.DeletionTimestamp = nil
			if err = reconciler.Update(ctx, wandb); err != nil {
				return CtrlError(err)
			}
			wandb.Status.State = apiv2.WBStateDeletionPaused
			wandb.Status.Message = "Database backup before deletion failed, deletion paused. Disable backups to continue with deletion."
			if err = reconciler.Status().Update(ctx, wandb); err != nil {
				return CtrlError(err)
			}
			return CtrlDoneUntil(ReconcilerScope, defaultDbRequeueDuration)
		}

		if wandb.Status.DatabaseStatus.BackupStatus.State == "InProgress" {
			log.Info("Backup in progress, requeuing", "backup", wandb.Status.DatabaseStatus.BackupStatus.BackupName)
			if wandb.Status.State != apiv2.WBStateDeleting {
				wandb.Status.State = apiv2.WBStateDeleting
				wandb.Status.DatabaseStatus.State = string(pxcv1.AppStateStopping)
				wandb.Status.Message = "Waiting for database backup to complete before deletion"
				if err := reconciler.Status().Update(ctx, wandb); err != nil {
					return CtrlError(err)
				}
			}
			return CtrlDone(HandlerScope)
		}

		controllerutil.RemoveFinalizer(wandb, dbFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			return CtrlError(err)
		}
		if actualPercona.obj != nil {
			if err := reconciler.Client.Delete(ctx, actualPercona.obj); err != nil {
				return CtrlError(err)
			}
		}

		return CtrlDone(HandlerScope)
	}
	return CtrlContinue()
}

func (w *wandbPerconaMysqlWrapper) handleDatabaseBackup(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, reconciler *WeightsAndBiasesV2Reconciler,
) error {
	log := ctrl.LoggerFrom(ctx)

	if w.obj == nil {
		log.Info("Percona XtraDB Cluster object is nil, skipping backup")
		return nil
	}

	if !wandb.Spec.Database.Enabled {
		log.Info("Database not enabled, skipping backup")
		return nil
	}

	if !wandb.Spec.Database.Backup.Enabled {
		log.Info("Database backup not enabled, skipping backup")
		reconciler.updateDbBackupStatus(ctx, wandb, "Skipped", "Backup disabled in spec")
		return nil
	}

	log.Info("Executing database backup before deletion")

	storageName := wandb.Spec.Database.Backup.StorageName
	if storageName == "" {
		storageName = "default-backup"
	}

	executor := &PerconaBackupExecutor{
		Client:         reconciler.Client,
		ClusterName:    w.obj.GetName(),
		Namespace:      w.obj.Namespace,
		StorageName:    storageName,
		TimeoutSeconds: wandb.Spec.Database.Backup.TimeoutSeconds,
	}

	currentBackupState := &BackupState{
		BackupName:  wandb.Status.DatabaseStatus.BackupStatus.BackupName,
		StartedAt:   wandb.Status.DatabaseStatus.BackupStatus.StartedAt,
		CompletedAt: wandb.Status.DatabaseStatus.BackupStatus.CompletedAt,
		State:       wandb.Status.DatabaseStatus.BackupStatus.State,
	}

	newState, result, err := executor.EnsureBackup(ctx, currentBackupState)

	if newState != nil {
		wandb.Status.DatabaseStatus.BackupStatus = apiv2.WBBackupStatus{
			BackupName:     newState.BackupName,
			StartedAt:      newState.StartedAt,
			CompletedAt:    newState.CompletedAt,
			LastBackupTime: newState.CompletedAt,
			State:          newState.State,
			Message:        result.Message,
		}

		if result.InProgress {
			wandb.Status.DatabaseStatus.BackupStatus.State = "InProgress"
			wandb.Status.DatabaseStatus.BackupStatus.RequeueAfter = int64(result.RequeueAfter.Seconds())
		}

		if statusErr := reconciler.Client.Status().Update(ctx, wandb); statusErr != nil {
			log.Error(statusErr, "Failed to update backup status")
		}
	}

	if err != nil && !result.InProgress {
		reconciler.updateDbBackupStatus(ctx, wandb, "Failed", err.Error())
		return err
	}

	if result.Completed {
		return nil
	}

	if result.InProgress {
		return nil
	}

	return err
}

type wandbMysqlConnInfoCreate struct {
	desired wandbPerconaMysqlWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbMysqlConnInfoCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Creating MySQL connection secret")

	if c.desired.secret == nil {
		log.Error(nil, "Desired secret is nil")
		return CtrlError(errors.New("desired secret is nil"))
	}

	if err := controllerutil.SetOwnerReference(c.wandb, c.desired.secret, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for MySQL connection secret")
		return CtrlError(err)
	}

	if err := r.Create(ctx, c.desired.secret); err != nil {
		log.Error(err, "Failed to create MySQL connection secret")
		return CtrlError(err)
	}

	log.Info("MySQL connection secret created successfully")
	return CtrlDone(HandlerScope)
}

type wandbMysqlConnInfoDelete struct {
	wandb *apiv2.WeightsAndBiases
}

func (d *wandbMysqlConnInfoDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Deleting MySQL connection secret")

	namespacedName := types.NamespacedName{
		Name:      "wandb-mysql-connection",
		Namespace: d.wandb.Namespace,
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, namespacedName, secret)
	if err != nil {
		if machErrors.IsNotFound(err) {
			log.Info("MySQL connection secret already deleted")
			return CtrlContinue()
		}
		log.Error(err, "Failed to get MySQL connection secret for deletion")
		return CtrlError(err)
	}

	if err := r.Delete(ctx, secret); err != nil {
		log.Error(err, "Failed to delete MySQL connection secret")
		return CtrlError(err)
	}

	log.Info("MySQL connection secret deleted successfully")
	return CtrlDone(HandlerScope)
}
