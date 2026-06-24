package bufstream

import (
	"context"
	"log/slog"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckDetached blocks spec changes when the broker Application has been detached
// from the wandb CR but its replica count no longer matches the desired spec.
func CheckDetached(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbUID types.UID,
	desiredReplicas int32,
) []metav1.Condition {
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	actual := &apiv2.Application{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.BufstreamNsName(), ApplicationResourceType, actual)
	if err != nil || !found {
		return nil
	}
	if !common.IsDetached(actual, wandbUID) {
		return nil
	}
	actualReplicas := int32(0)
	if actual.Spec.Replicas != nil {
		actualReplicas = *actual.Spec.Replicas
	}
	// Compare against the effective (HA-floored) broker count, the same value
	// ToBufstreamApplication builds the Deployment with. Otherwise a detached
	// cluster sized below the floor (e.g. dev sizing requests 1, floor is 2)
	// would always look mismatched and never get re-adopted on re-apply.
	if actualReplicas != effectiveBufstreamReplicas(desiredReplicas) {
		return []metav1.Condition{{
			Type:    common.ReconciledType,
			Status:  metav1.ConditionFalse,
			Reason:  common.DetachedSpecMismatch,
			Message: "detached Bufstream Application spec mismatch",
		}}
	}
	return nil
}

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	apps := []types.NamespacedName{
		nsnBuilder.EtcdNsName(),
		nsnBuilder.BufstreamNsName(),
	}
	for _, nn := range apps {
		if err := detachObject(ctx, cl, log, nn, ApplicationResourceType, &apiv2.Application{}, wandbOwner); err != nil {
			return err
		}
	}

	if err := detachObject(ctx, cl, log, nsnBuilder.ConfigMapNsName(), ConfigMapResourceType, &corev1.ConfigMap{}, wandbOwner); err != nil {
		return err
	}
	if err := detachObject(ctx, cl, log, nsnBuilder.CredentialsNsName(), SecretResourceType, &corev1.Secret{}, wandbOwner); err != nil {
		return err
	}
	return detachObject(ctx, cl, log, nsnBuilder.ConnectionNsName(), SecretResourceType, &corev1.Secret{}, wandbOwner)
}

func detachObject(
	ctx context.Context,
	cl client.Client,
	log *slog.Logger,
	nsName types.NamespacedName,
	resourceType string,
	obj client.Object,
	wandbOwner client.Object,
) error {
	found, err := common.GetResource(ctx, cl, nsName, resourceType, obj)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if common.IsDetached(obj, wandbOwner.GetUID()) {
		return nil
	}
	common.RemoveOwnerReference(obj, wandbOwner.GetUID())
	if err := cl.Update(ctx, obj); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching resource", logx.ErrAttr(err), "name", nsName.Name)
		return err
	}
	log.Info("detached resource", "type", resourceType, "name", nsName.Name)
	return nil
}
