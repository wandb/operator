package wandb_v2

import (
	"context"
	"errors"

	ndbv1 "github.com/mysql/ndb-operator/pkg/apis/ndbcontroller/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type wandbNdkMysqlWrapper struct {
	installed bool
	obj       *ndbv1.NdbCluster
}

type wandbNdkMysqlDoReconcile interface {
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState
}

func mysqlNamespacedName(req ctrl.Request) types.NamespacedName {
	return types.NamespacedName{
		Name:      "wandb-mysql",
		Namespace: req.Namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handleNdkMysql(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	var err error
	var ctrlState CtrlState
	var desiredMysql wandbNdkMysqlWrapper
	var actualMysql wandbNdkMysqlWrapper
	var reconciliation wandbNdkMysqlDoReconcile
	var namespacedName = mysqlNamespacedName(req)

	if actualMysql, err = actualNdkMysql(ctx, r, namespacedName); err != nil {
		return CtrlError(err)
	}

	if ctrlState = actualMysql.maybeHandleDeletion(ctx, wandb, r); ctrlState.isDone() {
		return ctrlState
	}

	if desiredMysql, err = desiredNdkMysql(ctx, wandb, namespacedName); err != nil {
		return CtrlError(err)
	}
	if reconciliation, err = computeReconcileDrift(ctx, desiredMysql, actualMysql); err != nil {
		return CtrlError(err)
	}
	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}
	return CtrlContinue()
}

// groupVersionKind may be necessary if there is a case where WeightsAndBiases object is missing GroupVersionKind
func (r *WeightsAndBiasesV2Reconciler) groupVersionKind(wandb *apiv2.WeightsAndBiases) (schema.GroupVersionKind, error) {
	gvks, _, err := r.Scheme.ObjectKinds(wandb)
	if err != nil || len(gvks) == 0 {
		return schema.GroupVersionKind{}, err
	}
	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, errors.New("no GroupKindVersion for WeightsAndBiases Scheme")
	}
	gvk := gvks[0]
	return gvk, nil
}

func actualNdkMysql(
	ctx context.Context, reconciler *WeightsAndBiasesV2Reconciler, namespacedName types.NamespacedName,
) (
	wandbNdkMysqlWrapper, error,
) {
	result := wandbNdkMysqlWrapper{
		installed: false,
		obj:       nil,
	}
	obj := &ndbv1.NdbCluster{}
	err := reconciler.Get(ctx, namespacedName, obj)
	if err != nil {
		if machErrors.IsNotFound(err) {
			return result, nil
		} else {
			return result, err
		}
	} else {
		result.obj = obj
		result.installed = true
	}
	obj.SetOwnerReferences([]metav1.OwnerReference{})
	return result, nil
}

func desiredNdkMysql(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName,
) (
	wandbNdkMysqlWrapper, error,
) {
	result := wandbNdkMysqlWrapper{
		installed: false,
		obj:       nil,
	}
	if wandb.Spec.Database.Enabled {
		result.installed = true
		gvk := wandb.GroupVersionKind()
		if gvk.GroupVersion().Group == "" || gvk.GroupVersion().Version == "" || gvk.Kind == "" {
			return result, errors.New("no GroupKindVersion for WeightsAndBiases CR")
		}
		blockOwnerDeletion := true
		isController := true
		result.obj = &ndbv1.NdbCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      namespacedName.Name,
				Namespace: namespacedName.Namespace,
				// NOTE! NDK Mysql overwrites GetOwnerReferences() which causes
				// controllerutil.SetControllerReference() to not function.
				// Therefore, we set OwnerReferences directly here.
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         gvk.GroupVersion().String(),
						Kind:               gvk.Kind,
						Name:               wandb.GetName(),
						UID:                wandb.GetUID(),
						BlockOwnerDeletion: &blockOwnerDeletion,
						Controller:         &isController,
					},
				},
			},
			Spec: ndbv1.NdbClusterSpec{
				DataNode: &ndbv1.NdbDataNodeSpec{
					NodeCount: 2,
				},
			},
		}
	}
	return result, nil
}

func computeReconcileDrift(
	ctx context.Context, desiredMysql, actualMysql wandbNdkMysqlWrapper,
) (
	wandbNdkMysqlDoReconcile, error,
) {
	if (!desiredMysql.installed) && (actualMysql.installed) {
		return &wandbNdkMysqlDelete{
			actual: actualMysql,
		}, nil
	}
	if (desiredMysql.installed) && (!actualMysql.installed) {
		return &wandbNdkMysqlCreate{
			desired: desiredMysql,
		}, nil
	}
	return nil, nil
}

type wandbNdkMysqlCreate struct {
	desired wandbNdkMysqlWrapper
}

func (c *wandbNdkMysqlCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing NDK Mysql")
	if err := r.Create(ctx, c.desired.obj); err != nil {
		return CtrlDone(ctrl.Result{})
	}
	return CtrlContinue()
}

type wandbNdkMysqlDelete struct {
	actual wandbNdkMysqlWrapper
}

func (d *wandbNdkMysqlDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) CtrlState {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling NDK Mysql")
	if err := r.Delete(ctx, d.actual.obj); err != nil {
		return CtrlDone(ctrl.Result{})
	}
	return CtrlContinue()
}

func (w *wandbNdkMysqlWrapper) maybeHandleDeletion(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, reconciler *WeightsAndBiasesV2Reconciler,
) CtrlState {
	log := ctrllog.FromContext(ctx)

	var flaggedForDeletion = !wandb.ObjectMeta.DeletionTimestamp.IsZero()
	var hasDbFinalizer = ctrlqueue.ContainsString(wandb.GetFinalizers(), dbFinalizer)

	// Make sure finalizer is present, when not in deletion flow
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

		controllerutil.RemoveFinalizer(wandb, dbFinalizer)
		if err := reconciler.Client.Update(ctx, wandb); err != nil {
			return CtrlError(err)
		}
		return CtrlDone(ctrl.Result{})
	}
	return CtrlContinue()
}

func (w *wandbNdkMysqlWrapper) handleDatabaseBackup(
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

	executor := &NoOpBackupExecutor{
		DatabaseName: w.obj.GetName(),
		Namespace:    w.obj.Namespace,
	}

	if err := executor.Backup(ctx); err != nil {
		reconciler.updateDbBackupStatus(ctx, wandb, "Failed", err.Error())
		return err
	}

	reconciler.updateDbBackupStatus(ctx, wandb, "Completed", "Backup completed successfully")
	return nil
}
