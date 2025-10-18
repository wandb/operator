package wandb_v2

import (
	"context"
	"errors"

	ndbv1 "github.com/mysql/ndb-operator/pkg/apis/ndbcontroller/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

type wandbNdkMysqlWrapper struct {
	installed bool
	obj       *ndbv1.NdbCluster
}

type wandbNdkMysqlDoReconcile interface {
	Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) (ctrl.Result, error)
}

func mysqlNamespacedName(req ctrl.Request) types.NamespacedName {
	return types.NamespacedName{
		Name:      "wandb-mysql",
		Namespace: req.Namespace,
	}
}

func (r *WeightsAndBiasesV2Reconciler) handleMysql(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (
	ctrl.Result, error,
) {
	var err error
	var desiredMysql wandbNdkMysqlWrapper
	var actualMysql wandbNdkMysqlWrapper
	var reconciliation wandbNdkMysqlDoReconcile

	if actualMysql, err = r.actualNdkMysql(ctx, req); err != nil {
		return ctrl.Result{}, err
	}
	if desiredMysql, err = r.desiredNdkMysql(ctx, req, wandb); err != nil {
		return ctrl.Result{}, err

	}
	if reconciliation, err = computeReconcileDrift(ctx, desiredMysql, actualMysql); err != nil {
		return ctrl.Result{}, err
	}
	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}
	return ctrl.Result{}, nil
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

func (r *WeightsAndBiasesV2Reconciler) actualNdkMysql(
	ctx context.Context, req ctrl.Request,
) (
	wandbNdkMysqlWrapper, error,
) {
	result := wandbNdkMysqlWrapper{
		installed: false,
		obj:       nil,
	}
	obj := &ndbv1.NdbCluster{}
	err := r.Get(ctx, mysqlNamespacedName(req), obj)
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
	return result, nil
}

func (r *WeightsAndBiasesV2Reconciler) desiredNdkMysql(
	ctx context.Context, req ctrl.Request, wandb *apiv2.WeightsAndBiases,
) (
	wandbNdkMysqlWrapper, error,
) {
	result := wandbNdkMysqlWrapper{
		installed: false,
		obj:       nil,
	}
	if wandb.Spec.Database.Enabled {
		result.installed = true
		mysqlNamespacedName := mysqlNamespacedName(req)
		gvk := wandb.GroupVersionKind()
		if gvk.GroupVersion().Group == "" || gvk.GroupVersion().Version == "" || gvk.Kind == "" {
			return result, errors.New("no GroupKindVersion for WeightsAndBiases CR")
		}
		blockOwnerDeletion := true
		isController := true
		result.obj = &ndbv1.NdbCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mysqlNamespacedName.Name,
				Namespace: mysqlNamespacedName.Namespace,
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

func (c *wandbNdkMysqlCreate) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Installing NDK Mysql")
	if err := r.Create(ctx, c.desired.obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

type wandbNdkMysqlDelete struct {
	actual wandbNdkMysqlWrapper
}

func (d *wandbNdkMysqlDelete) Execute(ctx context.Context, r *WeightsAndBiasesV2Reconciler) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Uninstalling NDK Mysql")
	if err := r.Delete(ctx, d.actual.obj); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
