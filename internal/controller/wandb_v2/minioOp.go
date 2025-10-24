package wandb_v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	miniov2 "github.com/minio/operator/pkg/apis/minio.min.io/v2"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	corev1 "k8s.io/api/core/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const minioFinalizer = "minio.app.wandb.com"

type wandbMinioWrapper struct {
	installed bool
	obj       *miniov2.Tenant
}

func (w *wandbMinioWrapper) IsReady() bool {
	if !w.installed || w.obj == nil {
		return false
	}

	return w.obj.Status.AvailableReplicas > 0 && w.obj.Status.CurrentState == "Initialized"
}

func (w *wandbMinioWrapper) GetStatus() string {
	if !w.installed || w.obj == nil {
		return "NotInstalled"
	}

	if w.obj.Status.CurrentState == "Initialized" && w.obj.Status.AvailableReplicas > 0 {
		return "ready"
	}

	if w.obj.Status.CurrentState != "" {
		return w.obj.Status.CurrentState
	}

	return "pending"
}

type wandbMinioDoReconcile interface {
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState
}

func minioNamespacedName(req ctrl.Request) types.NamespacedName {
	return types.NamespacedName{
		Name:      "wandb-minio",
		Namespace: req.Namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handleMinio(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var desiredMinio wandbMinioWrapper
	var actualMinio wandbMinioWrapper
	var reconciliation wandbMinioDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := minioNamespacedName(req)

	if !wandb.Spec.ObjStorage.Enabled {
		log.Info("ObjStorage not enabled, skipping")
		return CtrlContinue()
	}

	log.Info("Handling MinIO")

	if actualMinio, err = getActualMinio(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual MinIO resources")
		return CtrlError(err)
	}

	if ctrlState := actualMinio.maybeHandleDeletion(ctx, wandb, actualMinio, r); ctrlState.shouldExit(HandlerScope) {
		return ctrlState
	}

	if desiredMinio, err = getDesiredMinio(ctx, wandb, namespacedName); err != nil {
		log.Error(err, "Failed to get desired MinIO configuration")
		return CtrlError(err)
	}

	if reconciliation, err = computeMinioReconcileDrift(ctx, wandb, desiredMinio, actualMinio); err != nil {
		log.Error(err, "Failed to compute MinIO reconcile drift")
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func getActualMinio(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbMinioWrapper, error,
) {
	result := wandbMinioWrapper{
		installed: false,
		obj:       nil,
	}

	obj := &miniov2.Tenant{}
	err := reconciler.Get(ctx, namespacedName, obj)
	if err == nil {
		result.obj = obj
		result.installed = true
	} else if !machErrors.IsNotFound(err) {
		return result, err
	}

	return result, nil
}

func getDesiredMinio(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName,
) (
	wandbMinioWrapper, error,
) {
	result := wandbMinioWrapper{
		installed: false,
		obj:       nil,
	}

	if !wandb.Spec.ObjStorage.Enabled {
		return result, nil
	}

	result.installed = true

	storageSize := wandb.Spec.ObjStorage.StorageSize
	if storageSize == "" {
		storageSize = "10Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}

	replicas := wandb.Spec.ObjStorage.Replicas
	if replicas == 0 {
		replicas = 1
	}

	configSecretName := namespacedName.Name + "-config"
	tenant := &miniov2.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"app": "wandb-minio",
			},
		},
		Spec: miniov2.TenantSpec{
			Image: "quay.io/minio/minio:latest",
			Configuration: &corev1.LocalObjectReference{
				Name: configSecretName,
			},
			Pools: []miniov2.Pool{
				{
					Name:             "pool-0",
					Servers:          replicas,
					VolumesPerServer: 4,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: storageQuantity,
								},
							},
						},
					},
				},
			},
		},
	}

	wandbBackupSpec := wandb.Spec.ObjStorage.Backup
	if wandbBackupSpec.Enabled {
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for MinIO")
		}
	}

	result.obj = tenant
	return result, nil
}

func computeMinioReconcileDrift(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, desiredMinio, actualMinio wandbMinioWrapper,
) (
	wandbMinioDoReconcile, error,
) {
	if !desiredMinio.installed && actualMinio.installed {
		return &wandbMinioDelete{
			actual: actualMinio,
			wandb:  wandb,
		}, nil
	}
	if desiredMinio.installed && !actualMinio.installed {
		return &wandbMinioCreate{
			desired: desiredMinio,
			wandb:   wandb,
		}, nil
	}
	if actualMinio.GetStatus() != wandb.Status.ObjStorageStatus.State ||
		actualMinio.IsReady() != wandb.Status.ObjStorageStatus.Ready {
		return &wandbMinioStatusUpdate{
			wandb:  wandb,
			status: actualMinio.GetStatus(),
			ready:  actualMinio.IsReady(),
		}, nil
	}
	return nil, nil
}

type wandbMinioCreate struct {
	desired wandbMinioWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbMinioCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing MinIO Tenant")
	wandb := c.wandb

	configSecretName := c.desired.obj.Name + "-config"
	configSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configSecretName,
			Namespace: c.desired.obj.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"config.env": `export MINIO_ROOT_USER="admin"
export MINIO_ROOT_PASSWORD="admin123456"
export MINIO_BROWSER="on"`,
		},
	}

	if err = controllerutil.SetOwnerReference(wandb, configSecret, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for MinIO config secret")
		return CtrlError(err)
	}

	existingSecret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: configSecretName, Namespace: c.desired.obj.Namespace}, existingSecret)
	if err != nil && machErrors.IsNotFound(err) {
		if err = r.Create(ctx, configSecret); err != nil {
			log.Error(err, "Failed to create MinIO config secret")
			return CtrlError(err)
		}
		log.Info("Created MinIO configuration secret", "secret", configSecretName)
	} else if err != nil {
		log.Error(err, "Failed to check for existing MinIO config secret")
		return CtrlError(err)
	}

	c.desired.obj.Spec.Configuration = &corev1.LocalObjectReference{
		Name: configSecretName,
	}

	if err = controllerutil.SetOwnerReference(wandb, c.desired.obj, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for MinIO Tenant")
		return CtrlError(err)
	}

	if err = r.Create(ctx, c.desired.obj); err != nil {
		log.Error(err, "Failed to create MinIO Tenant")
		return CtrlError(err)
	}

	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Creating MinIO"
	wandb.Status.ObjStorageStatus.State = "pending"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after creating MinIO Tenant")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbMinioDelete struct {
	actual wandbMinioWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbMinioDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling MinIO Tenant")

	if err = r.Delete(ctx, d.actual.obj); err != nil {
		log.Error(err, "Failed to delete MinIO Tenant")
		return CtrlError(err)
	}

	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Deleting MinIO"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after deleting MinIO Tenant")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbMinioStatusUpdate struct {
	wandb  *apiv2.WeightsAndBiases
	status string
	ready  bool
}

func (s *wandbMinioStatusUpdate) Execute(
	ctx context.Context, r *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Updating MinIO status", "status", s.status, "ready", s.ready)
	s.wandb.Status.ObjStorageStatus.State = s.status
	s.wandb.Status.ObjStorageStatus.Ready = s.ready
	if err := r.Status().Update(ctx, s.wandb); err != nil {
		log.Error(err, "Failed to update MinIO status")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

func (w *wandbMinioWrapper) maybeHandleDeletion(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, actualMinio wandbMinioWrapper, reconciler *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrllog.FromContext(ctx)

	var deletionPaused = wandb.Status.State == apiv2.WBStateDeletionPaused
	var backupEnabled = wandb.Spec.ObjStorage.Backup.Enabled
	var flaggedForDeletion = !wandb.ObjectMeta.DeletionTimestamp.IsZero()
	var hasMinioFinalizer = ctrlqueue.ContainsString(wandb.GetFinalizers(), minioFinalizer)

	if flaggedForDeletion && !backupEnabled {
		log.Info("MinIO backup is disabled.")
		controllerutil.RemoveFinalizer(wandb, minioFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to remove MinIO finalizer")
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if deletionPaused && backupEnabled {
		log.Info("Deletion paused for MinIO Backup; disable backups to continue with deletion")
		return CtrlContinue()
	}

	if !hasMinioFinalizer && !flaggedForDeletion {
		wandb.ObjectMeta.Finalizers = append(wandb.ObjectMeta.Finalizers, minioFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to add MinIO finalizer")
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if flaggedForDeletion {
		if err := w.handleMinioBackup(ctx, wandb, reconciler); err != nil {
			log.Info("Failed to backup MinIO, pausing deletion")
			wandb.ObjectMeta.DeletionTimestamp = nil
			if err = reconciler.Update(ctx, wandb); err != nil {
				log.Error(err, "Failed to update WeightsAndBiases during backup failure")
				return CtrlError(err)
			}
			wandb.Status.State = apiv2.WBStateDeletionPaused
			wandb.Status.Message = "MinIO backup before deletion failed, deletion paused. Disable backups to continue with deletion."
			if err = reconciler.Status().Update(ctx, wandb); err != nil {
				log.Error(err, "Failed to update status to deletion paused")
				return CtrlError(err)
			}
			return CtrlDone(HandlerScope)
		}

		if wandb.Status.ObjStorageStatus.BackupStatus.State == "InProgress" {
			log.Info("Backup in progress, requeuing", "backup", wandb.Status.ObjStorageStatus.BackupStatus.BackupName)
			if wandb.Status.State != apiv2.WBStateDeleting {
				wandb.Status.State = apiv2.WBStateDeleting
				wandb.Status.ObjStorageStatus.State = "stopping"
				wandb.Status.Message = "Waiting for MinIO backup to complete before deletion"
				if err := reconciler.Status().Update(ctx, wandb); err != nil {
					log.Error(err, "Failed to update status while backup in progress")
					return CtrlError(err)
				}
			}
			return CtrlDone(HandlerScope)
		}

		controllerutil.RemoveFinalizer(wandb, minioFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to remove MinIO finalizer after backup")
			return CtrlError(err)
		}

		if actualMinio.obj != nil {
			if err := reconciler.Client.Delete(ctx, actualMinio.obj); err != nil {
				log.Error(err, "Failed to delete MinIO Tenant during cleanup")
				return CtrlError(err)
			}
		}

		return CtrlDone(HandlerScope)
	}
	return CtrlContinue()
}

func (w *wandbMinioWrapper) handleMinioBackup(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, reconciler *WeightsAndBiasesV2Reconciler,
) error {
	log := ctrl.LoggerFrom(ctx)

	if w.obj == nil {
		log.Info("MinIO Tenant object is nil, skipping backup")
		return nil
	}

	if !wandb.Spec.ObjStorage.Enabled {
		log.Info("ObjStorage not enabled, skipping backup")
		return nil
	}

	if !wandb.Spec.ObjStorage.Backup.Enabled {
		log.Info("MinIO backup not enabled, skipping backup")
		return nil
	}

	log.Info("Executing MinIO backup before deletion")

	storageName := wandb.Spec.ObjStorage.Backup.StorageName
	if storageName == "" {
		storageName = "default-backup"
	}

	executor := &MinioBackupExecutor{
		Client:         reconciler.Client,
		TenantName:     w.obj.GetName(),
		Namespace:      w.obj.Namespace,
		StorageName:    storageName,
		TimeoutSeconds: wandb.Spec.ObjStorage.Backup.TimeoutSeconds,
	}

	currentBackupState := &BackupState{
		BackupName:  wandb.Status.ObjStorageStatus.BackupStatus.BackupName,
		StartedAt:   wandb.Status.ObjStorageStatus.BackupStatus.StartedAt,
		CompletedAt: wandb.Status.ObjStorageStatus.BackupStatus.CompletedAt,
		State:       wandb.Status.ObjStorageStatus.BackupStatus.State,
	}

	newState, result, err := executor.EnsureMinioBackup(ctx, currentBackupState)

	if newState != nil {
		wandb.Status.ObjStorageStatus.BackupStatus = apiv2.WBBackupStatus{
			BackupName:     newState.BackupName,
			StartedAt:      newState.StartedAt,
			CompletedAt:    newState.CompletedAt,
			LastBackupTime: newState.CompletedAt,
			State:          newState.State,
			Message:        result.Message,
		}

		if result.InProgress {
			wandb.Status.ObjStorageStatus.BackupStatus.State = "InProgress"
			wandb.Status.ObjStorageStatus.BackupStatus.RequeueAfter = int64(result.RequeueAfter.Seconds())
		}

		if statusErr := reconciler.Client.Status().Update(ctx, wandb); statusErr != nil {
			log.Error(statusErr, "Failed to update backup status")
		}
	}

	if err != nil && !result.InProgress {
		reconciler.updateMinioBackupStatus(ctx, wandb, "Failed", err.Error())
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

func (r *WeightsAndBiasesV2Reconciler) updateMinioBackupStatus(ctx context.Context, wandb *apiv2.WeightsAndBiases, state, message string) {
	log := ctrl.LoggerFrom(ctx)
	now := metav1.Now()

	wandb.Status.ObjStorageStatus.BackupStatus = apiv2.WBBackupStatus{
		LastBackupTime: &now,
		State:          state,
		Message:        message,
	}

	if err := r.Client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update MinIO backup status")
	}
}

type MinioBackupExecutor struct {
	Client         client.Client
	TenantName     string
	Namespace      string
	StorageName    string
	TimeoutSeconds int
}

func (m *MinioBackupExecutor) EnsureMinioBackup(ctx context.Context, currentState *BackupState) (*BackupState, BackupResult, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("MinIO backup requested", "tenant", m.TenantName, "storage", m.StorageName)

	backupName := fmt.Sprintf("%s-backup-%d", m.TenantName, time.Now().Unix())
	now := metav1.Now()

	state := &BackupState{
		BackupName:  backupName,
		StartedAt:   &now,
		CompletedAt: &now,
		State:       "completed",
	}

	result := BackupResult{
		Completed:    true,
		InProgress:   false,
		Failed:       false,
		Message:      "MinIO backup completed (placeholder implementation)",
		RequeueAfter: 0,
	}

	return state, result, nil
}
