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
	if actual.Annotations[DetachedAnnotation] == "true" {
		return []metav1.Condition{
			{
				Type:    common.ReconciledType,
				Status:  metav1.ConditionFalse,
				Reason:  common.DetachedSpecMismatch,
				Message: "detached MySQL CR must be renamed before it can be managed again",
			},
		}
	}
	if actual.Labels[WandbUIDLabel] == string(wandbUID) {
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

	if markDetached(actual, wandbOwner.GetUID()) {
		if err = cl.Update(ctx, actual); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			log.Error("error detaching MysqlCluster CR", logx.ErrAttr(err))
			return err
		}
		log.Info("detached MysqlCluster CR", "name", actual.Name)
	} else {
		log.Debug("MysqlCluster CR already detached")
	}

	configMap := &corev1.ConfigMap{}
	found, err = common.GetResource(
		ctx,
		cl,
		types.NamespacedName{Namespace: specNamespacedName.Namespace, Name: MyCnfConfigMapName(specNamespacedName.Name)},
		"ConfigMap",
		configMap,
	)
	if err != nil || !found {
		return err
	}
	if markDetached(configMap, wandbOwner.GetUID()) {
		if err := cl.Update(ctx, configMap); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func markDetached(obj client.Object, ownerUID types.UID) bool {
	changed := !common.IsDetached(obj, ownerUID)
	common.RemoveOwnerReference(obj, ownerUID)
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	if annotations[DetachedAnnotation] != "true" {
		annotations[DetachedAnnotation] = "true"
		obj.SetAnnotations(annotations)
		changed = true
	}
	return changed
}
