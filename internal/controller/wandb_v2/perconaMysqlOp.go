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

type wandbPerconaMysqlWrapper struct {
	installed bool
	obj       *pxcv1.PerconaXtraDBCluster
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

	if ctrlState := actualPercona.maybeHandleDeletion(ctx, wandb, r); ctrlState.isDone() {
		return ctrlState
	}

	if desiredPercona, err = desiredPerconaMysql(ctx, wandb, namespacedName); err != nil {
		return CtrlError(err)
	}

	if reconciliation, err = computePerconaReconcileDrift(ctx, desiredPercona, actualPercona); err != nil {
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
		installed: false,
		obj:       nil,
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
	return result, nil
}

func desiredPerconaMysql(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName,
) (
	wandbPerconaMysqlWrapper, error,
) {
	result := wandbPerconaMysqlWrapper{
		installed: false,
		obj:       nil,
	}

	if !wandb.Spec.Database.Enabled {
		return result, nil
	}

	result.installed = true
	gvk := wandb.GroupVersionKind()
	if gvk.GroupVersion().Group == "" || gvk.GroupVersion().Version == "" || gvk.Kind == "" {
		return result, errors.New("no GroupKindVersion for WeightsAndBiases CR")
	}

	storageSize := wandb.Spec.Database.StorageSize
	if storageSize == "" {
		storageSize = "20Gi"
	}

	storageQuantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return result, errors.New("invalid storage size: " + storageSize)
	}

	pxc := &pxcv1.PerconaXtraDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: pxcv1.PerconaXtraDBClusterSpec{
			CRVersion:   "1.18.0",
			SecretsName: "wandb-percona-mysql-secrets",
			Unsafe: pxcv1.UnsafeFlags{
				PXCSize: true,
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
			HAProxy: &pxcv1.HAProxySpec{
				PodSpec: pxcv1.PodSpec{
					Enabled: false,
				},
			},
			LogCollector: &pxcv1.LogCollectorSpec{
				Enabled: true,
				Image:   "percona/percona-xtradb-cluster-operator:main-log-collector",
			},
		},
	}

	result.obj = pxc
	return result, nil
}

func computePerconaReconcileDrift(
	ctx context.Context, desiredPercona, actualPercona wandbPerconaMysqlWrapper,
) (
	wandbPerconaMysqlDoReconcile, error,
) {
	if !desiredPercona.installed && actualPercona.installed {
		return &wandbPerconaMysqlDelete{
			actual: actualPercona,
		}, nil
	}
	if desiredPercona.installed && !actualPercona.installed {
		return &wandbPerconaMysqlCreate{
			desired: desiredPercona,
		}, nil
	}
	return nil, nil
}

type wandbPerconaMysqlCreate struct {
	desired wandbPerconaMysqlWrapper
}

func (c *wandbPerconaMysqlCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing Percona XtraDB Cluster")
	if err := r.Create(ctx, c.desired.obj); err != nil {
		return CtrlError(err)
	}
	return CtrlContinue()
}

type wandbPerconaMysqlDelete struct {
	actual wandbPerconaMysqlWrapper
}

func (d *wandbPerconaMysqlDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling Percona XtraDB Cluster")
	if err := r.Delete(ctx, d.actual.obj); err != nil {
		return CtrlError(err)
	}
	return CtrlContinue()
}

func (w *wandbPerconaMysqlWrapper) maybeHandleDeletion(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, reconciler *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrllog.FromContext(ctx)

	var flaggedForDeletion = !wandb.ObjectMeta.DeletionTimestamp.IsZero()
	var hasDbFinalizer = ctrlqueue.ContainsString(wandb.GetFinalizers(), dbFinalizer)

	if !hasDbFinalizer && !flaggedForDeletion {
		wandb.ObjectMeta.Finalizers = append(wandb.ObjectMeta.Finalizers, dbFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			return CtrlError(err)
		}
		return CtrlContinue()
	}

	if flaggedForDeletion {
		if err := w.handleDatabaseBackup(ctx, wandb, reconciler); err != nil {
			log.Error(err, "Failed to backup database before deletion")
			return CtrlError(err)
		}

		if wandb.Status.DatabaseStatus.BackupStatus.State == "InProgress" {
			log.Info("Backup in progress, requeuing", "backup", wandb.Status.DatabaseStatus.BackupStatus.BackupName)
			requeueSeconds := wandb.Status.DatabaseStatus.BackupStatus.RequeueAfter
			if requeueSeconds == 0 {
				requeueSeconds = 30
			}
			return CtrlDone(ctrl.Result{RequeueAfter: time.Duration(requeueSeconds) * time.Second})
		}

		controllerutil.RemoveFinalizer(wandb, dbFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			return CtrlError(err)
		}
		return CtrlDone(ctrl.Result{})
	}
	return CtrlContinue()
}

func (w *wandbPerconaMysqlWrapper) handleDatabaseBackup(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, reconciler *WeightsAndBiasesV2Reconciler,
) error {
	log := ctrl.LoggerFrom(ctx)

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
