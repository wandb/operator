package moco

import (
	"context"
	"fmt"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
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
	actual := &mocov1beta2.MySQLCluster{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.ClusterNsName(), ResourceTypeName, actual)
	if err != nil || !found {
		return nil
	}
	if !common.IsDetached(actual, wandbUID) {
		return nil
	}

	if desiredReplicas > 0 && actual.Spec.Replicas != desiredReplicas {
		return []metav1.Condition{
			{
				Type:    common.ReconciledType,
				Status:  metav1.ConditionFalse,
				Reason:  common.DetachedSpecMismatch,
				Message: fmt.Sprintf("detached MySQL CR spec mismatch: replicas want %d, have %d", desiredReplicas, actual.Spec.Replicas),
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

	var actual = &mocov1beta2.MySQLCluster{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.ClusterNsName(), ResourceTypeName, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach finalizer: MysqlCluster CR not found")
		return nil
	}

	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("MysqlCluster CR already detached")
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching MysqlCluster CR", logx.ErrAttr(err))
		return err
	}
	log.Info("detached MysqlCluster CR", "name", actual.Name)

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
