package tenant

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	miniov2 "github.com/wandb/operator/pkg/vendored/minio-operator/minio.min.io/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CheckDetached(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbUID types.UID,
	desiredReplicas int32,
) []metav1.Condition {
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	actual := &miniov2.Tenant{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.SpecNsName(), ResourceTypeName, actual)
	if err != nil || !found {
		return nil
	}
	if !common.IsDetached(actual, wandbUID) {
		return nil
	}

	if desiredReplicas > 0 && len(actual.Spec.Pools) > 0 {
		if int32(actual.Spec.Pools[0].Servers) != desiredReplicas {
			return []metav1.Condition{
				{
					Type:    common.ReconciledType,
					Status:  metav1.ConditionFalse,
					Reason:  common.DetachedSpecMismatch,
					Message: fmt.Sprintf("detached Minio CR spec mismatch: replicas want %d, have %d", desiredReplicas, actual.Spec.Pools[0].Servers),
				},
			}
		}
	}
	return nil
}

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, log := logx.WithSlog(ctx, logx.ObjectStore)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	var actual = &miniov2.Tenant{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.SpecNsName(), ResourceTypeName, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach finalizer: Tenant CR not found")
		return nil
	}

	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("Tenant CR already detached")
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching Tenant CR", logx.ErrAttr(err))
		return err
	}
	log.Info("detached Tenant CR", "name", actual.Name)
	return nil
}
