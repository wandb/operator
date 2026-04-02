package mysql

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	v2 "github.com/wandb/operator/pkg/vendored/mysql-operator/v2"
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
	actual := &v2.InnoDBCluster{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.ClusterNsName(), ResourceTypeName, actual)
	if err != nil || !found {
		return nil
	}
	if !common.IsDetached(actual, wandbUID) {
		return nil
	}

	if desiredReplicas > 0 && actual.Spec.Instances != desiredReplicas {
		return []metav1.Condition{
			{
				Type:    common.ReconciledType,
				Status:  metav1.ConditionFalse,
				Reason:  common.DetachedSpecMismatch,
				Message: fmt.Sprintf("detached MySQL CR spec mismatch: replicas want %d, have %d", desiredReplicas, actual.Spec.Instances),
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
	ctx, log := logx.WithSlog(ctx, logx.Mysql)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	var actual = &v2.InnoDBCluster{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.ClusterNsName(), ResourceTypeName, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach finalizer: InnoDBCluster CR not found")
		return nil
	}

	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("InnoDBCluster CR already detached")
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching InnoDBCluster CR", logx.ErrAttr(err))
		return err
	}
	log.Info("detached InnoDBCluster CR", "name", actual.Name)

	if err = detachConnectionSecret(ctx, cl, nsnBuilder, wandbOwner); err != nil {
		log.Error("error detaching connection secret", logx.ErrAttr(err))
		return err
	}
	return nil
}

func detachConnectionSecret(ctx context.Context, cl client.Client, nsnBuilder *NsNameBuilder, wandbOwner client.Object) error {
	secret := &corev1.Secret{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.ConnectionNsName(), "Secret", secret)
	if err != nil || !found {
		return err
	}
	common.RemoveOwnerReference(secret, wandbOwner.GetUID())
	if err = cl.Update(ctx, secret); err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}
