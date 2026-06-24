package bufstream

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WriteState reconciles the managed Bufstream resources: an etcd StatefulSet, a
// credentials secret and config map, and the Bufstream broker Deployment. It
// gates on object store readiness because Bufstream needs an S3 bucket.
func WriteState(
	ctx context.Context,
	cl client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)

	spec := wandb.Spec.Kafka.ManagedKafka
	nsnBuilder := CreateNsNameBuilder(types.NamespacedName{Namespace: spec.Namespace, Name: spec.Name})

	storage, ready, err := resolveStorage(ctx, cl, wandb)
	if err != nil {
		log.Error("failed to resolve object store connection for bufstream", logx.ErrAttr(err))
		return []metav1.Condition{
			{Type: common.ReconciledType, Status: metav1.ConditionFalse, Reason: common.ApiErrorReason},
			{Type: ObjectStoreReadyType, Status: metav1.ConditionUnknown, Reason: common.ApiErrorReason},
		}
	}
	if !ready {
		log.Info("object store not ready yet, deferring bufstream provisioning")
		return []metav1.Condition{
			{Type: ObjectStoreReadyType, Status: metav1.ConditionFalse, Reason: common.PendingCreateReason},
		}
	}

	credsSecret, err := ToCredentialsSecret(wandb, nsnBuilder, storage, cl.Scheme())
	if err != nil {
		return translateError(err)
	}
	configMap, err := ToConfigMap(wandb, nsnBuilder, storage, cl.Scheme())
	if err != nil {
		return translateError(err)
	}
	etcdApp, err := ToEtcdApplication(wandb, nsnBuilder, cl.Scheme())
	if err != nil {
		return translateError(err)
	}
	bufstreamApp, err := ToBufstreamApplication(wandb, nsnBuilder, storage, cl.Scheme())
	if err != nil {
		return translateError(err)
	}

	results := []metav1.Condition{
		{Type: ObjectStoreReadyType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason},
	}
	results = append(results, writeResource(ctx, cl, common.ReconciledType, SecretResourceType, credsSecret, &corev1.Secret{})...)
	results = append(results, writeResource(ctx, cl, common.ReconciledType, ConfigMapResourceType, configMap, &corev1.ConfigMap{})...)
	results = append(results, writeResource(ctx, cl, EtcdApplicationType, ApplicationResourceType, etcdApp, &apiv2.Application{})...)
	results = append(results, writeResource(ctx, cl, BufstreamApplicationType, ApplicationResourceType, bufstreamApp, &apiv2.Application{})...)

	return results
}

func translateError(err error) []metav1.Condition {
	return []metav1.Condition{
		{Type: common.ReconciledType, Status: metav1.ConditionFalse, Reason: common.ControllerErrorReason, Message: err.Error()},
	}
}

// writeResource gets the current object, runs the generic CRUD, and maps the
// resulting action onto a status condition of the given type.
func writeResource[T client.Object](
	ctx context.Context,
	cl client.Client,
	conditionType string,
	resourceType string,
	desired T,
	actual T,
) []metav1.Condition {
	found, err := common.GetResource(ctx, cl, client.ObjectKeyFromObject(desired), resourceType, actual)
	if err != nil {
		return []metav1.Condition{
			{Type: common.ReconciledType, Status: metav1.ConditionFalse, Reason: common.ApiErrorReason},
			{Type: conditionType, Status: metav1.ConditionUnknown, Reason: common.ApiErrorReason},
		}
	}
	if !found {
		var zero T
		actual = zero
	}

	action, err := common.CrudResource(ctx, cl, desired, actual)
	if err != nil {
		return []metav1.Condition{
			{Type: common.ReconciledType, Status: metav1.ConditionFalse, Reason: common.ApiErrorReason},
		}
	}

	if conditionType == common.ReconciledType {
		// Supporting resources only report reconcile success/failure.
		return nil
	}

	return []metav1.Condition{actionToCondition(conditionType, action)}
}

func actionToCondition(conditionType string, action common.CrudAction) metav1.Condition {
	switch action {
	case common.CreateAction:
		return metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse, Reason: common.PendingCreateReason}
	case common.DeleteAction:
		return metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse, Reason: common.PendingDeleteReason}
	case common.UpdateAction:
		return metav1.Condition{Type: conditionType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason}
	default:
		return metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse, Reason: common.NoResourceReason}
	}
}

// resolveStorage reads the object store connection secret and parses its
// connection string into the provider-specific values needed to configure
// Bufstream. Returns ready=false when the object store is not yet available.
func resolveStorage(
	ctx context.Context,
	cl client.Client,
	wandb *apiv2.WeightsAndBiases,
) (storageConnInfo, bool, error) {
	if !wandb.Status.ObjectStoreStatus.Ready {
		return storageConnInfo{}, false, nil
	}

	secretName := wandb.Status.ObjectStoreStatus.Connection.URL.Name
	if secretName == "" {
		return storageConnInfo{}, false, nil
	}

	secret := &corev1.Secret{}
	found, err := common.GetResource(
		ctx, cl,
		types.NamespacedName{Namespace: wandb.Namespace, Name: secretName},
		"Secret", secret,
	)
	if err != nil {
		return storageConnInfo{}, false, err
	}
	if !found {
		return storageConnInfo{}, false, nil
	}

	info, err := parseStorageConnection(secret.Data)
	if err != nil {
		return storageConnInfo{}, false, err
	}
	return info, true, nil
}
