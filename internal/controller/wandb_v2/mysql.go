package wandb_v2

import (
	"context"

	ndbv1 "github.com/mysql/ndb-operator/pkg/apis/ndbcontroller/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *WeightsAndBiasesV2Reconciler) handleMysql(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (
	ctrl.Result, error,
) {
	mysqlNamespacedName := types.NamespacedName{
		Name:      "wandb-mysql",
		Namespace: req.Namespace,
	}

	var expectMysql bool
	var hasMysql bool

	expectMysql = wandb.Spec.Database.Enabled

	existingMysql := &ndbv1.NdbCluster{}
	err := r.Get(ctx, mysqlNamespacedName, existingMysql)
	if err != nil {
		if errors.IsNotFound(err) {
			hasMysql = false
		} else {
			return ctrl.Result{}, err
		}
	} else {
		hasMysql = true
	}

	if expectMysql && !hasMysql {
		newMysql := &ndbv1.NdbCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mysqlNamespacedName.Name,
				Namespace: mysqlNamespacedName.Namespace,
			},
			Spec: ndbv1.NdbClusterSpec{
				DataNode: &ndbv1.NdbDataNodeSpec{
					NodeCount: 2,
				},
			},
		}

		gvks, _, err := r.Scheme.ObjectKinds(wandb)
		if err != nil || len(gvks) == 0 {
			return ctrl.Result{}, err
		}
		gvk := gvks[0]

		blockOwnerDeletion := true
		isController := true
		newMysql.ObjectMeta.OwnerReferences = append(
			newMysql.ObjectMeta.OwnerReferences,
			metav1.OwnerReference{
				APIVersion:         gvk.GroupVersion().String(),
				Kind:               gvk.Kind,
				Name:               wandb.GetName(),
				UID:                wandb.GetUID(),
				BlockOwnerDeletion: &blockOwnerDeletion,
				Controller:         &isController,
			},
		)

		if err := r.Create(ctx, newMysql); err != nil {
			return ctrl.Result{}, err
		}
	} else if !expectMysql && hasMysql {
		// Check if this NdbCluster is owned by this WeightsAndBiases CR
		// Only delete if it's owned by us to avoid deleting manually created clusters
		isOwnedByUs := false
		for _, owner := range existingMysql.ObjectMeta.OwnerReferences {
			if owner.UID == wandb.GetUID() {
				isOwnedByUs = true
				break
			}
		}

		if isOwnedByUs {
			if err := r.Delete(ctx, existingMysql); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}
