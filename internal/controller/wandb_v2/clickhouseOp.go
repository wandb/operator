package wandb_v2

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	chiv1 "github.com/wandb/operator/api/altinity-clickhouse-vendored/clickhouse.altinity.com/v1"
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

const clickhouseFinalizer = "clickhouse.app.wandb.com"

type wandbClickHouseWrapper struct {
	installed       bool
	obj             *chiv1.ClickHouseInstallation
	secretInstalled bool
	secret          *corev1.Secret
}

func (w *wandbClickHouseWrapper) IsReady() bool {
	if !w.installed || w.obj == nil {
		return false
	}

	if w.obj.Status == nil {
		return false
	}

	return w.obj.Status.Status == chiv1.StatusCompleted
}

func (w *wandbClickHouseWrapper) GetStatus() string {
	if !w.installed || w.obj == nil {
		return "NotInstalled"
	}

	if w.obj.Status == nil {
		return "pending"
	}

	status := w.obj.Status.Status
	if status == chiv1.StatusCompleted {
		return "ready"
	}

	if status != "" {
		return status
	}

	return "pending"
}

type wandbClickHouseDoReconcile interface {
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState
}

func clickhouseNamespacedName(req ctrl.Request) types.NamespacedName {
	namespace := req.Namespace
	if namespace == "" {
		namespace = "default"
	}
	return types.NamespacedName{
		Name:      "wandb-clickhouse",
		Namespace: namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handleClickHouse(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var desiredClickHouse wandbClickHouseWrapper
	var actualClickHouse wandbClickHouseWrapper
	var reconciliation wandbClickHouseDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := clickhouseNamespacedName(req)

	if !wandb.Spec.ClickHouse.Enabled {
		log.Info("ClickHouse not enabled, skipping")
		return CtrlContinue()
	}

	log.Info("Handling ClickHouse")

	if actualClickHouse, err = getActualClickHouse(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual ClickHouse resources")
		return CtrlError(err)
	}

	if ctrlState := actualClickHouse.maybeHandleDeletion(ctx, wandb, actualClickHouse, r); ctrlState.shouldExit(HandlerScope) {
		return ctrlState
	}

	if desiredClickHouse, err = getDesiredClickHouse(ctx, wandb, namespacedName, actualClickHouse); err != nil {
		log.Error(err, "Failed to get desired ClickHouse configuration")
		return CtrlError(err)
	}

	if reconciliation, err = computeClickHouseReconcileDrift(ctx, wandb, desiredClickHouse, actualClickHouse); err != nil {
		log.Error(err, "Failed to compute ClickHouse reconcile drift")
		return CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return CtrlContinue()
}

func getActualClickHouse(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbClickHouseWrapper, error,
) {
	result := wandbClickHouseWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	obj := &chiv1.ClickHouseInstallation{}
	err := reconciler.Get(ctx, namespacedName, obj)
	if err == nil {
		result.obj = obj
		result.installed = true
	} else if !machErrors.IsNotFound(err) {
		return result, err
	}

	secretNamespacedName := types.NamespacedName{
		Name:      "wandb-clickhouse-connection",
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

func getDesiredClickHouse(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, actual wandbClickHouseWrapper,
) (
	wandbClickHouseWrapper, error,
) {
	result := wandbClickHouseWrapper{
		installed:       false,
		obj:             nil,
		secretInstalled: false,
		secret:          nil,
	}

	if !wandb.Spec.ClickHouse.Enabled {
		return result, nil
	}

	result.installed = true

	storageSize := wandb.Spec.ClickHouse.StorageSize
	if storageSize == "" {
		storageSize = "10Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}

	replicas := wandb.Spec.ClickHouse.Replicas
	if replicas == 0 {
		replicas = 1
	}

	canaryUsername := "test_user"
	canaryPassword := "test_password"
	passwordSha256 := fmt.Sprintf("%x", sha256.Sum256([]byte(canaryPassword)))
	settings := chiv1.NewSettings()
	settings.Set(
		fmt.Sprintf("%s/password_sha256_hex", canaryUsername),
		chiv1.NewSettingScalar(passwordSha256),
	)
	settings.Set(
		fmt.Sprintf("%s/networks/ip", canaryUsername),
		chiv1.NewSettingScalar("::/0"),
	)

	chi := &chiv1.ClickHouseInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"app": "wandb-clickhouse",
			},
		},
		Spec: chiv1.ChiSpec{
			Configuration: &chiv1.Configuration{
				Clusters: []*chiv1.Cluster{
					{
						Name: "cluster",
						Layout: &chiv1.ChiClusterLayout{
							ShardsCount:   1,
							ReplicasCount: int(replicas),
						},
					},
				},
				Users: settings,
			},
			Defaults: &chiv1.Defaults{
				Templates: &chiv1.TemplatesList{
					DataVolumeClaimTemplate: "default-volume",
				},
			},
			Templates: &chiv1.Templates{
				VolumeClaimTemplates: []chiv1.VolumeClaimTemplate{
					{
						Name: "default-volume",
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

	wandbBackupSpec := wandb.Spec.ClickHouse.Backup
	if wandbBackupSpec.Enabled {
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for ClickHouse")
		}
	}

	result.obj = chi

	if actual.IsReady() {
		namespace := namespacedName.Namespace
		clickhouseHost := "clickhouse-wandb-clickhouse." + namespace + ".svc.cluster.local"
		clickhousePort := "9000"
		clickhouseHTTPPort := "8123"

		connectionSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-clickhouse-connection",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"CLICKHOUSE_CANARY_USERNAME": canaryUsername,
				"CLICKHOUSE_CANARY_PASSWORD": canaryPassword,
				"CLICKHOUSE_HOST":            clickhouseHost,
				"CLICKHOUSE_PORT":            clickhousePort,
				"CLICKHOUSE_HTTP_PORT":       clickhouseHTTPPort,
			},
		}

		result.secret = connectionSecret
		result.secretInstalled = true
	}

	return result, nil
}

func computeClickHouseReconcileDrift(
	_ context.Context, wandb *apiv2.WeightsAndBiases, desiredClickHouse, actualClickHouse wandbClickHouseWrapper,
) (
	wandbClickHouseDoReconcile, error,
) {
	if !desiredClickHouse.installed && actualClickHouse.installed {
		if actualClickHouse.secretInstalled {
			return &wandbClickHouseConnInfoDelete{
				wandb: wandb,
			}, nil
		}
		return &wandbClickHouseDelete{
			actual: actualClickHouse,
			wandb:  wandb,
		}, nil
	}
	if desiredClickHouse.installed && !actualClickHouse.installed {
		return &wandbClickHouseCreate{
			desired: desiredClickHouse,
			wandb:   wandb,
		}, nil
	}

	if desiredClickHouse.secretInstalled && !actualClickHouse.secretInstalled {
		return &wandbClickHouseConnInfoCreate{
			desired: desiredClickHouse,
			wandb:   wandb,
		}, nil
	}

	if actualClickHouse.GetStatus() != wandb.Status.ClickHouseStatus.State ||
		actualClickHouse.IsReady() != wandb.Status.ClickHouseStatus.Ready {
		return &wandbClickHouseStatusUpdate{
			wandb:  wandb,
			status: actualClickHouse.GetStatus(),
			ready:  actualClickHouse.IsReady(),
		}, nil
	}
	return nil, nil
}

type wandbClickHouseCreate struct {
	desired wandbClickHouseWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbClickHouseCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing ClickHouse")
	wandb := c.wandb

	if err = controllerutil.SetOwnerReference(wandb, c.desired.obj, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for ClickHouse")
		return CtrlError(err)
	}

	if err = r.Create(ctx, c.desired.obj); err != nil {
		log.Error(err, "Failed to create ClickHouse")
		return CtrlError(err)
	}

	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Creating ClickHouse"
	wandb.Status.ClickHouseStatus.State = "pending"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after creating ClickHouse")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbClickHouseDelete struct {
	actual wandbClickHouseWrapper
	wandb  *apiv2.WeightsAndBiases
}

func (d *wandbClickHouseDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling ClickHouse")

	if err = r.Delete(ctx, d.actual.obj); err != nil {
		log.Error(err, "Failed to delete ClickHouse")
		return CtrlError(err)
	}

	wandb := d.wandb
	wandb.Status.State = apiv2.WBStateInfraUpdate
	wandb.Status.Message = "Deleting ClickHouse"
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status after deleting ClickHouse")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

type wandbClickHouseStatusUpdate struct {
	wandb  *apiv2.WeightsAndBiases
	status string
	ready  bool
}

func (s *wandbClickHouseStatusUpdate) Execute(
	ctx context.Context, r *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Updating ClickHouse status", "status", s.status, "ready", s.ready)
	s.wandb.Status.ClickHouseStatus.State = s.status
	s.wandb.Status.ClickHouseStatus.Ready = s.ready
	if err := r.Status().Update(ctx, s.wandb); err != nil {
		log.Error(err, "Failed to update ClickHouse status")
		return CtrlError(err)
	}
	return CtrlDone(HandlerScope)
}

func (w *wandbClickHouseWrapper) maybeHandleDeletion(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, actualClickHouse wandbClickHouseWrapper, reconciler *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrllog.FromContext(ctx)

	var deletionPaused = wandb.Status.State == apiv2.WBStateDeletionPaused
	var backupEnabled = wandb.Spec.ClickHouse.Backup.Enabled
	var flaggedForDeletion = !wandb.ObjectMeta.DeletionTimestamp.IsZero()
	var hasClickHouseFinalizer = ctrlqueue.ContainsString(wandb.GetFinalizers(), clickhouseFinalizer)

	if flaggedForDeletion && !backupEnabled {
		log.Info("ClickHouse backup is disabled.")
		controllerutil.RemoveFinalizer(wandb, clickhouseFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to remove ClickHouse finalizer")
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if deletionPaused && backupEnabled {
		log.Info("Deletion paused for ClickHouse Backup; disable backups to continue with deletion")
		return CtrlContinue()
	}

	if !hasClickHouseFinalizer && !flaggedForDeletion {
		wandb.ObjectMeta.Finalizers = append(wandb.ObjectMeta.Finalizers, clickhouseFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to add ClickHouse finalizer")
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if flaggedForDeletion {
		if err := w.handleClickHouseBackup(ctx, wandb, reconciler); err != nil {
			log.Info("Failed to backup ClickHouse, pausing deletion")
			wandb.ObjectMeta.DeletionTimestamp = nil
			if err = reconciler.Update(ctx, wandb); err != nil {
				log.Error(err, "Failed to update WeightsAndBiases during backup failure")
				return CtrlError(err)
			}
			wandb.Status.State = apiv2.WBStateDeletionPaused
			wandb.Status.Message = "ClickHouse backup before deletion failed, deletion paused. Disable backups to continue with deletion."
			if err = reconciler.Status().Update(ctx, wandb); err != nil {
				log.Error(err, "Failed to update status to deletion paused")
				return CtrlError(err)
			}
			return CtrlDone(HandlerScope)
		}

		if wandb.Status.ClickHouseStatus.BackupStatus.State == "InProgress" {
			log.Info("Backup in progress, requeuing", "backup", wandb.Status.ClickHouseStatus.BackupStatus.BackupName)
			if wandb.Status.State != apiv2.WBStateDeleting {
				wandb.Status.State = apiv2.WBStateDeleting
				wandb.Status.ClickHouseStatus.State = "stopping"
				wandb.Status.Message = "Waiting for ClickHouse backup to complete before deletion"
				if err := reconciler.Status().Update(ctx, wandb); err != nil {
					log.Error(err, "Failed to update status while backup in progress")
					return CtrlError(err)
				}
			}
			return CtrlDone(HandlerScope)
		}

		controllerutil.RemoveFinalizer(wandb, clickhouseFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to remove ClickHouse finalizer after backup")
			return CtrlError(err)
		}

		if actualClickHouse.obj != nil {
			if err := reconciler.Client.Delete(ctx, actualClickHouse.obj); err != nil {
				log.Error(err, "Failed to delete ClickHouse during cleanup")
				return CtrlError(err)
			}
		}

		return CtrlDone(HandlerScope)
	}
	return CtrlContinue()
}

func (w *wandbClickHouseWrapper) handleClickHouseBackup(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, reconciler *WeightsAndBiasesV2Reconciler,
) error {
	log := ctrl.LoggerFrom(ctx)

	if w.obj == nil {
		log.Info("ClickHouse object is nil, skipping backup")
		return nil
	}

	if !wandb.Spec.ClickHouse.Enabled {
		log.Info("ClickHouse not enabled, skipping backup")
		return nil
	}

	if !wandb.Spec.ClickHouse.Backup.Enabled {
		log.Info("ClickHouse backup not enabled, skipping backup")
		return nil
	}

	log.Info("Executing ClickHouse backup before deletion")

	storageName := wandb.Spec.ClickHouse.Backup.StorageName
	if storageName == "" {
		storageName = "default-backup"
	}

	executor := &ClickHouseBackupExecutor{
		Client:         reconciler.Client,
		InstallName:    w.obj.GetName(),
		Namespace:      w.obj.Namespace,
		StorageName:    storageName,
		TimeoutSeconds: wandb.Spec.ClickHouse.Backup.TimeoutSeconds,
	}

	currentBackupState := &BackupState{
		BackupName:  wandb.Status.ClickHouseStatus.BackupStatus.BackupName,
		StartedAt:   wandb.Status.ClickHouseStatus.BackupStatus.StartedAt,
		CompletedAt: wandb.Status.ClickHouseStatus.BackupStatus.CompletedAt,
		State:       wandb.Status.ClickHouseStatus.BackupStatus.State,
	}

	newState, result, err := executor.EnsureClickHouseBackup(ctx, currentBackupState)

	if newState != nil {
		wandb.Status.ClickHouseStatus.BackupStatus = apiv2.WBBackupStatus{
			BackupName:     newState.BackupName,
			StartedAt:      newState.StartedAt,
			CompletedAt:    newState.CompletedAt,
			LastBackupTime: newState.CompletedAt,
			State:          newState.State,
			Message:        result.Message,
		}

		if result.InProgress {
			wandb.Status.ClickHouseStatus.BackupStatus.State = "InProgress"
			wandb.Status.ClickHouseStatus.BackupStatus.RequeueAfter = int64(result.RequeueAfter.Seconds())
		}

		if statusErr := reconciler.Client.Status().Update(ctx, wandb); statusErr != nil {
			log.Error(statusErr, "Failed to update backup status")
		}
	}

	if err != nil && !result.InProgress {
		reconciler.updateClickHouseBackupStatus(ctx, wandb, "Failed", err.Error())
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

func (r *WeightsAndBiasesV2Reconciler) updateClickHouseBackupStatus(ctx context.Context, wandb *apiv2.WeightsAndBiases, state, message string) {
	log := ctrl.LoggerFrom(ctx)
	now := metav1.Now()

	wandb.Status.ClickHouseStatus.BackupStatus = apiv2.WBBackupStatus{
		LastBackupTime: &now,
		State:          state,
		Message:        message,
	}

	if err := r.Client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update ClickHouse backup status")
	}
}

type ClickHouseBackupExecutor struct {
	Client         client.Client
	InstallName    string
	Namespace      string
	StorageName    string
	TimeoutSeconds int
}

func (c *ClickHouseBackupExecutor) EnsureClickHouseBackup(ctx context.Context, currentState *BackupState) (*BackupState, BackupResult, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("ClickHouse backup requested", "installation", c.InstallName, "storage", c.StorageName)

	backupName := fmt.Sprintf("%s-backup-%d", c.InstallName, time.Now().Unix())
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
		Message:      "ClickHouse backup completed (placeholder implementation)",
		RequeueAfter: 0,
	}

	return state, result, nil
}

type wandbClickHouseConnInfoCreate struct {
	desired wandbClickHouseWrapper
	wandb   *apiv2.WeightsAndBiases
}

func (c *wandbClickHouseConnInfoCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Creating ClickHouse connection secret")

	if c.desired.secret == nil {
		log.Error(nil, "Desired secret is nil")
		return CtrlError(errors.New("desired secret is nil"))
	}

	if err := controllerutil.SetOwnerReference(c.wandb, c.desired.secret, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference for ClickHouse connection secret")
		return CtrlError(err)
	}

	if err := r.Create(ctx, c.desired.secret); err != nil {
		log.Error(err, "Failed to create ClickHouse connection secret")
		return CtrlError(err)
	}

	log.Info("ClickHouse connection secret created successfully")
	return CtrlDone(HandlerScope)
}

type wandbClickHouseConnInfoDelete struct {
	wandb *apiv2.WeightsAndBiases
}

func (d *wandbClickHouseConnInfoDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Deleting ClickHouse connection secret")

	namespacedName := types.NamespacedName{
		Name:      "wandb-clickhouse-connection",
		Namespace: d.wandb.Namespace,
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, namespacedName, secret)
	if err != nil {
		if machErrors.IsNotFound(err) {
			log.Info("ClickHouse connection secret already deleted")
			return CtrlContinue()
		}
		log.Error(err, "Failed to get ClickHouse connection secret for deletion")
		return CtrlError(err)
	}

	if err := r.Delete(ctx, secret); err != nil {
		log.Error(err, "Failed to delete ClickHouse connection secret")
		return CtrlError(err)
	}

	log.Info("ClickHouse connection secret deleted successfully")
	return CtrlDone(HandlerScope)
}
