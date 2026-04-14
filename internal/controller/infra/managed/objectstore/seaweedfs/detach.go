package seaweedfs

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	corev1 "k8s.io/api/core/v1"
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
	actual := &seaweedv1.Seaweed{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.SpecNsName(), ResourceTypeName, actual)
	if err != nil || !found {
		return nil
	}
	if !common.IsDetached(actual, wandbUID) {
		return nil
	}

	if desiredReplicas > 0 && (actual.Spec.Volume != nil || actual.Spec.Volume.Replicas != desiredReplicas ) {
		return []metav1.Condition{
			{
				Type:    common.ReconciledType,
				Status:  metav1.ConditionFalse,
				Reason:  common.DetachedSpecMismatch,
				Message: fmt.Sprintf("detached Seaweed CR spec mismatch: volume replicas want %d, have %d", desiredReplicas, actual.Spec.Volume.Replicas),
			},
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

	var actual = &seaweedv1.Seaweed{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.SpecNsName(), ResourceTypeName, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach finalizer: Seaweed CR not found")
		return nil
	}

	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("Seaweed CR already detached")
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching Seaweed CR", logx.ErrAttr(err))
		return err
	}
	log.Info("detached Seaweed CR", "name", actual.Name)

	secret := &corev1.Secret{}
	found, err = common.GetResource(ctx, cl, nsnBuilder.ConnectionNsName(), "Secret", secret)
	if err != nil || !found {
		return err
	}
	common.RemoveOwnerReference(secret, wandbOwner.GetUID())
	if err = cl.Update(ctx, secret); err != nil && !errors.IsNotFound(err) {
		log.Error("error detaching connection secret", logx.ErrAttr(err))
		return err
	}
	log.Info("detached connection secret", "name", nsnBuilder.ConnectionNsName().Name)
	return nil
}
