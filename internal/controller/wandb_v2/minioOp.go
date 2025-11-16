package wandb_v2

import (
	"context"
	"errors"
	"fmt"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
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
	installed       bool
	obj             *miniov2.Tenant
	secretInstalled bool
	secret          *corev1.Secret
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
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState
}

func minioNamespacedName(wandb *apiv2.WeightsAndBiases) types.NamespacedName {
	namespace := wandb.Spec.Minio.Namespace
	if namespace == "" {
		namespace = wandb.Namespace
	}
	return types.NamespacedName{
		Name:      "wandb-minio",
		Namespace: namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handleMinio(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) ctrlqueue.CtrlState {
	var err error
	var desiredMinio wandbMinioWrapper
	var actualMinio wandbMinioWrapper
	var reconciliation wandbMinioDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := minioNamespacedName(wandb)

	if !wandb.Spec.Minio.Enabled {
		log.Info("ObjStorage not enabled, skipping")
		return ctrlqueue.CtrlContinue()
	}

	log.Info("Handling MinIO")

	if actualMinio, err = getActualMinio(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual MinIO resources")
		return ctrlqueue.CtrlError(err)
	}

	if ctrlState := actualMinio.maybeHandleDeletion(ctx, wandb, actualMinio, r); ctrlState.ShouldExit(ctrlqueue.PackageScope) {
		return ctrlState
	}

	if desiredMinio, err = getDesiredMinio(ctx, wandb, namespacedName, actualMinio); err != nil {
		log.Error(err, "Failed to get desired MinIO configuration")
		return ctrlqueue.CtrlError(err)
	}

	if reconciliation, err = computeMinioReconcileDrift(ctx, wandb, desiredMinio, actualMinio); err != nil {
		log.Error(err, "Failed to compute MinIO reconcile drift")
		return ctrlqueue.CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return ctrlqueue.CtrlContinue()
}

func getActualMinio(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbMinioWrapper, error,
) {
	result := wandbMinioWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	obj := &miniov2.Tenant{}
	err := reconciler.Get(ctx, namespacedName, obj)
	if err == nil {
		result.obj = obj
		result.installed = true
	} else if !machErrors.IsNotFound(err) {
		return result, err
	}

	secretNamespacedName := types.NamespacedName{
		Name:      "wandb-minio-connection",
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

func getDesiredMinio(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, actual wandbMinioWrapper,
) (
	wandbMinioWrapper, error,
) {
	result := wandbMinioWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	if !wandb.Spec.Minio.Enabled {
		return result, nil
	}

	result.installed = true

	storageSize := wandb.Spec.Minio.StorageSize
	if storageSize == "" {
		storageSize = "10Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}

	replicas := wandb.Spec.Minio.Replicas
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
					VolumesPerServer: 1,
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

	wandbBackupSpec := wandb.Spec.Minio.Backup
	if wandbBackupSpec.Enabled {
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for MinIO")
		}
	}

	result.obj = tenant

	if actual.IsReady() {
		namespace := namespacedName.Namespace
		minioEndpoint := "https://minio." + namespace + ".svc.cluster.local:443"
		minioConfigSecretName := "wandb-minio-config"
		minioConfigSecretKey := "config.env"
		minioConfigMountPath := "/etc/minio/config.env"

		connectionSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-minio-connection",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"MINIO_ENDPOINT":           minioEndpoint,
				"MINIO_CONFIG_SECRET_NAME": minioConfigSecretName,
				"MINIO_CONFIG_SECRET_KEY":  minioConfigSecretKey,
				"MINIO_CONFIG_MOUNT_PATH":  minioConfigMountPath,
			},
		}

		result.secret = connectionSecret
		result.secretInstalled = true
	}

	return result, nil
}

func computeMinioReconcileDrift(
	_ context.Context, wandb *apiv2.WeightsAndBiases, desiredMinio, actualMinio wandbMinioWrapper,
) (
	wandbMinioDoReconcile, error,
) {
	if !desiredMinio.installed && actualMinio.installed {
		if actualMinio.secretInstalled {
			return &wandbMinioConnInfoDelete{
				wandb: wandb,
			}, nil
		}
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

	if desiredMinio.secretInstalled && !actualMinio.secretInstalled {
		return &wandbMinioConnInfoCreate{
			desired: desiredMinio,
			wandb:   wandb,
		}, nil
	}

	if actualMinio.GetStatus() != string(wandb.Status.MinioStatus.State) ||
		actualMinio.IsReady() != wandb.Status.MinioStatus.Ready {
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

func (c *wandbMinioCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState {
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
		return ctrlqueue.CtrlError(err)
	}

	existingSecret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: configSecretName, Namespace: c.desired.obj.Namespace}, existingSecret)
	if err != nil && machErrors.IsNotFound(err) {
		if err = r.Create(ctx, configSecret); err != nil {
			log.Error(err, "Failed to create MinIO config secret")
			return ctrlqueue.CtrlError(err)
		}
		log.Info("Created MinIO configuration secret", "secret", configSecretName)
	} else if err != nil {
		log.Error(err, "Failed to check for existing MinIO config secret")
		return ctrlqueue.CtrlError(err)
	}

	c.desired.obj.Spec.Configuration = &corev1.LocalObjectReference{
		Name: configSecretName,
	}

	if err = controllerutil.SetOwnerReference(wandb, c.desired.obj, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for MinIO Tenant")
		return ctrlqueue.CtrlError(err)
	}

	if err = r.Create(ctx, c.desired.obj); err != nil {
		log.Error(err, "Failed to create MinIO Tenant")
		return ctrlqueue.CtrlError(err)
	}

	wandb.Status.State = apiv2.WBStateUpdating
	wandb.Status.MinioStatus.State = "pending"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after creating MinIO Tenant")
		return ctrlqueue.CtrlError(err)
	}
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}

type wandbMinioDelete struct {
	actual wandbMinioWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbMinioDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling MinIO Tenant")

	if err = r.Delete(ctx, d.actual.obj); err != nil {
		log.Error(err, "Failed to delete MinIO Tenant")
		return ctrlqueue.CtrlError(err)
	}

	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateUpdating
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after deleting MinIO Tenant")
		return ctrlqueue.CtrlError(err)
	}
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}

type wandbMinioStatusUpdate struct {
	wandb  *apiv2.WeightsAndBiases
	status string
	ready  bool
}

func (s *wandbMinioStatusUpdate) Execute(
	ctx context.Context, r *WeightsAndBiasesV2Reconciler,
) ctrlqueue.CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Updating MinIO status", "status", s.status, "ready", s.ready)
	s.wandb.Status.MinioStatus.State = apiv2.WBStateUpdating
	s.wandb.Status.MinioStatus.Ready = s.ready
	if err := r.Status().Update(ctx, s.wandb); err != nil {
		log.Error(err, "Failed to update MinIO status")
		return ctrlqueue.CtrlError(err)
	}
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}

func (w *wandbMinioWrapper) maybeHandleDeletion(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, actualMinio wandbMinioWrapper, reconciler *WeightsAndBiasesV2Reconciler,
) ctrlqueue.CtrlState {
	log := ctrllog.FromContext(ctx)

	var deletionPaused = wandb.Status.State == apiv2.WBStateOffline
	var backupEnabled = wandb.Spec.Minio.Backup.Enabled
	var flaggedForDeletion = !wandb.ObjectMeta.DeletionTimestamp.IsZero()
	var hasMinioFinalizer = ctrlqueue.ContainsString(wandb.GetFinalizers(), minioFinalizer)

	if flaggedForDeletion && !backupEnabled {
		log.Info("MinIO backup is disabled.")
		controllerutil.RemoveFinalizer(wandb, minioFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to remove MinIO finalizer")
			return ctrlqueue.CtrlError(err)
		}
		return ctrlqueue.CtrlContinue()
	}

	if deletionPaused && backupEnabled {
		log.Info("Deletion paused for MinIO Backup; disable backups to continue with deletion")
		return ctrlqueue.CtrlContinue()
	}

	if !hasMinioFinalizer && !flaggedForDeletion {
		wandb.ObjectMeta.Finalizers = append(wandb.ObjectMeta.Finalizers, minioFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to add MinIO finalizer")
			return ctrlqueue.CtrlError(err)
		}
		return ctrlqueue.CtrlContinue()
	}

	if flaggedForDeletion {
		if err := w.handleMinioBackup(ctx, wandb, reconciler); err != nil {
			log.Info("Failed to backup MinIO, pausing deletion")
			wandb.ObjectMeta.DeletionTimestamp = nil
			if err = reconciler.Update(ctx, wandb); err != nil {
				log.Error(err, "Failed to update WeightsAndBiases during backup failure")
				return ctrlqueue.CtrlError(err)
			}
			wandb.Status.State = apiv2.WBStateOffline
			if err = reconciler.Status().Update(ctx, wandb); err != nil {
				log.Error(err, "Failed to update status to deletion paused")
				return ctrlqueue.CtrlError(err)
			}
			return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
		}

		if wandb.Status.MinioStatus.BackupStatus.State == "InProgress" {
			log.Info("Backup in progress, requeuing", "backup", wandb.Status.MinioStatus.BackupStatus.BackupName)
			if wandb.Status.State != apiv2.WBStateDeleting {
				wandb.Status.State = apiv2.WBStateDeleting
				wandb.Status.MinioStatus.State = "stopping"
				if err := reconciler.Status().Update(ctx, wandb); err != nil {
					log.Error(err, "Failed to update status while backup in progress")
					return ctrlqueue.CtrlError(err)
				}
			}
			return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
		}

		controllerutil.RemoveFinalizer(wandb, minioFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to remove MinIO finalizer after backup")
			return ctrlqueue.CtrlError(err)
		}

		if actualMinio.obj != nil {
			if err := reconciler.Client.Delete(ctx, actualMinio.obj); err != nil {
				log.Error(err, "Failed to delete MinIO Tenant during cleanup")
				return ctrlqueue.CtrlError(err)
			}
		}

		return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
	}
	return ctrlqueue.CtrlContinue()
}

func (w *wandbMinioWrapper) handleMinioBackup(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, reconciler *WeightsAndBiasesV2Reconciler,
) error {
	log := ctrl.LoggerFrom(ctx)

	if w.obj == nil {
		log.Info("MinIO Tenant object is nil, skipping backup")
		return nil
	}

	if !wandb.Spec.Minio.Enabled {
		log.Info("ObjStorage not enabled, skipping backup")
		return nil
	}

	if !wandb.Spec.Minio.Backup.Enabled {
		log.Info("MinIO backup not enabled, skipping backup")
		return nil
	}

	log.Info("Executing MinIO backup before deletion")

	storageName := wandb.Spec.Minio.Backup.StorageName
	if storageName == "" {
		storageName = "default-backup"
	}

	executor := &MinioBackupExecutor{
		Client:         reconciler.Client,
		TenantName:     w.obj.GetName(),
		Namespace:      w.obj.Namespace,
		StorageName:    storageName,
		TimeoutSeconds: wandb.Spec.Minio.Backup.TimeoutSeconds,
	}

	currentBackupState := &BackupState{
		BackupName:  wandb.Status.MinioStatus.BackupStatus.BackupName,
		StartedAt:   wandb.Status.MinioStatus.BackupStatus.StartedAt,
		CompletedAt: wandb.Status.MinioStatus.BackupStatus.CompletedAt,
		State:       wandb.Status.MinioStatus.BackupStatus.State,
	}

	newState, result, err := executor.EnsureMinioBackup(ctx, currentBackupState)

	if newState != nil {
		wandb.Status.MinioStatus.BackupStatus = apiv2.WBBackupStatus{
			BackupName:     newState.BackupName,
			StartedAt:      newState.StartedAt,
			CompletedAt:    newState.CompletedAt,
			LastBackupTime: newState.CompletedAt,
			State:          newState.State,
			Message:        result.Message,
		}

		if result.InProgress {
			wandb.Status.MinioStatus.BackupStatus.State = "InProgress"
			wandb.Status.MinioStatus.BackupStatus.RequeueAfter = int64(result.RequeueAfter.Seconds())
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

	wandb.Status.MinioStatus.BackupStatus = apiv2.WBBackupStatus{
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

type wandbMinioConnInfoCreate struct {
	desired wandbMinioWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbMinioConnInfoCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Creating MinIO connection secret")

	if c.desired.secret == nil {
		log.Error(nil, "Desired secret is nil")
		return ctrlqueue.CtrlError(errors.New("desired secret is nil"))
	}

	if err := controllerutil.SetOwnerReference(c.wandb, c.desired.secret, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for MinIO connection secret")
		return ctrlqueue.CtrlError(err)
	}

	if err := r.Create(ctx, c.desired.secret); err != nil {
		log.Error(err, "Failed to create MinIO connection secret")
		return ctrlqueue.CtrlError(err)
	}

	log.Info("MinIO connection secret created successfully")
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}

type wandbMinioConnInfoDelete struct {
	wandb *apiv2.WeightsAndBiases
}

func (d *wandbMinioConnInfoDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) ctrlqueue.CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Deleting MinIO connection secret")

	namespace := d.wandb.Spec.Minio.Namespace
	if namespace == "" {
		namespace = d.wandb.Namespace
	}
	namespacedName := types.NamespacedName{
		Name:      "wandb-minio-connection",
		Namespace: namespace,
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, namespacedName, secret)
	if err != nil {
		if machErrors.IsNotFound(err) {
			log.Info("MinIO connection secret already deleted")
			return ctrlqueue.CtrlContinue()
		}
		log.Error(err, "Failed to get MinIO connection secret for deletion")
		return ctrlqueue.CtrlError(err)
	}

	if err := r.Delete(ctx, secret); err != nil {
		log.Error(err, "Failed to delete MinIO connection secret")
		return ctrlqueue.CtrlError(err)
	}

	log.Info("MinIO connection secret deleted successfully")
	return ctrlqueue.CtrlDone(ctrlqueue.PackageScope)
}
