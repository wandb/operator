package bufstream

import (
	"context"
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external/objectstore"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	mfst manifest.Manifest,
) []metav1.Condition {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)

	spec := wandb.Spec.Kafka.ManagedKafka
	nsnBuilder := CreateNsNameBuilder(types.NamespacedName{Namespace: spec.Namespace, Name: spec.Name})

	storage, ready, err := resolveStorage(ctx, cl, wandb, spec)
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
	//objectStoreSpec, _ := apiv2.ResolveInstance(wandb.Spec.ObjectStore, bufstreamObjectStoreInstance)
	ensureBucket := false

	credsSecret, err := ToCredentialsSecret(wandb, nsnBuilder, storage, cl.Scheme())
	if err != nil {
		return translateError(err)
	}
	configMap, err := ToConfigMap(wandb, nsnBuilder, storage, cl.Scheme())
	if err != nil {
		return translateError(err)
	}
	serviceAccount, err := ToServiceAccount(wandb, nsnBuilder, cl.Scheme())
	if err != nil {
		return translateError(err)
	}
	// On OpenShift, bind the SA to nonroot-v2 so broker runs as its fixed UID.
	var sccRoleBinding *rbacv1.RoleBinding
	if utils.IsOpenShift() {
		sccRoleBinding, err = ToSccRoleBinding(wandb, nsnBuilder, cl.Scheme())
		if err != nil {
			return translateError(err)
		}
	}
	etcdApp, err := ToEtcdApplication(wandb, nsnBuilder, cl.Scheme(), mfst)
	if err != nil {
		return translateError(err)
	}
	bufstreamApp, err := ToBufstreamApplication(wandb, nsnBuilder, storage, ensureBucket, cl.Scheme(), mfst)
	if err != nil {
		return translateError(err)
	}

	results := []metav1.Condition{
		{Type: ObjectStoreReadyType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason},
	}
	results = append(results, writeResource(ctx, cl, common.ReconciledType, SecretResourceType, credsSecret, &corev1.Secret{})...)
	results = append(results, writeResource(ctx, cl, common.ReconciledType, ConfigMapResourceType, configMap, &corev1.ConfigMap{})...)
	results = append(results, writeResource(ctx, cl, common.ReconciledType, ServiceAccountResourceType, serviceAccount, &corev1.ServiceAccount{})...)
	if sccRoleBinding != nil {
		results = append(results, writeResource(ctx, cl, common.ReconciledType, RoleBindingResourceType, sccRoleBinding, &rbacv1.RoleBinding{})...)
	}
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
	case common.UpdateAction, common.UnchangedAction:
		return metav1.Condition{Type: conditionType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason}
	default:
		return metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse, Reason: common.NoResourceReason}
	}
}

// bufstreamObjectStoreInstance is the object-store instance name Bufstream
// prefers for message storage; ResolveInstance falls back to the default
// instance when it is not provisioned.
const bufstreamObjectStoreInstance = "bufstream"

// resolveStorage reads the object store connection secret and parses its
// connection string into the provider-specific values needed to configure
// Bufstream. Returns ready=false when the object store is not yet available.
func resolveStorage(
	ctx context.Context,
	cl client.Client,
	wandb *apiv2.WeightsAndBiases,
	spec *apiv2.ManagedKafkaSpec,
) (objectstore.ConnInfo, bool, error) {
	status, ok := apiv2.ResolveInstance(wandb.Status.ObjectStoreStatus, bufstreamObjectStoreInstance)
	if !ok || !status.Ready {
		return objectstore.ConnInfo{}, false, nil
	}

	resolver := &utils.ConnSecretResolver{Client: cl, Namespace: spec.Namespace, Cache: map[string]*corev1.Secret{}}

	connInfo := objectstore.ConnInfo{}

	provider, err := resolver.Value(ctx, status.Connection.Provider)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}
	connInfo.Provider = apiv2.ObjectStoreProvider(provider)

	connInfo.Bucket, err = resolver.Value(ctx, status.Connection.Bucket)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}

	connInfo.Endpoint, err = resolver.Value(ctx, status.Connection.Endpoint)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}

	connInfo.Port, err = resolver.Value(ctx, status.Connection.Port)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}

	connInfo.Region, err = resolver.Value(ctx, status.Connection.Region)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}

	connInfo.AccessKey, err = resolver.Value(ctx, status.Connection.AccessKey)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}

	connInfo.SecretKey, err = resolver.Value(ctx, status.Connection.SecretKey)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}

	forcePathStyleString, err := resolver.Value(ctx, status.Connection.ForcePathStyle)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}
	connInfo.ForcePathStyle, err = strconv.ParseBool(forcePathStyleString)
	if err != nil {
		connInfo.ForcePathStyle = false
	}

	tlsEnabledString, err := resolver.Value(ctx, status.Connection.TlsEnabled)
	if err != nil {
		return objectstore.ConnInfo{}, true, err
	}
	connInfo.TlsEnabled, err = strconv.ParseBool(tlsEnabledString)
	if err != nil {
		connInfo.TlsEnabled = false
	}

	return connInfo, true, nil
}
