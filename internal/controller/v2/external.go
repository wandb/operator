package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mysqlConnectionName      = "wandb-mysql-connection"
	redisConnectionName      = "wandb-redis-connection"
	kafkaConnectionName      = "wandb-kafka-connection"
	minioConnectionName      = "wandb-minio-connection"
	clickHouseConnectionName = "wandb-clickhouse-connection"
)

// resolveSecretKey reads the value at sel from cluster secrets.
// Returns ("", nil) if sel.Name is empty (field not configured).
func resolveSecretKey(ctx context.Context, c client.Client, namespace string, sel corev1.SecretKeySelector) (string, error) {
	if sel.Name == "" {
		return "", nil
	}
	secret := &corev1.Secret{}
	found, err := common.GetResource(ctx, c, types.NamespacedName{Namespace: namespace, Name: sel.Name}, "Secret", secret)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("secret %s/%s not found", namespace, sel.Name)
	}
	val, ok := secret.Data[sel.Key]
	if !ok {
		if sel.Optional != nil && *sel.Optional {
			return "", nil
		}
		return "", fmt.Errorf("key %q not found in secret %s/%s", sel.Key, namespace, sel.Name)
	}
	return string(val), nil
}

func buildWandbOwnerRef(c client.Client, wandb *apiv2.WeightsAndBiases) (metav1.OwnerReference, error) {
	gvk, err := c.GroupVersionKindFor(wandb)
	if err != nil {
		return metav1.OwnerReference{}, fmt.Errorf("could not get GVK for wandb owner: %w", err)
	}
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               wandb.GetName(),
		UID:                wandb.GetUID(),
		Controller:         ptr.To(false),
		BlockOwnerDeletion: ptr.To(false),
	}, nil
}

func deleteWandbConnectionSecret(ctx context.Context, c client.Client, nsName types.NamespacedName) error {
	secret := &corev1.Secret{}
	found, err := common.GetResource(ctx, c, nsName, "Secret", secret)
	if err != nil {
		return err
	}
	if found {
		return c.Delete(ctx, secret)
	}
	return nil
}

func writeExternalConnectionSecret(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	nsName types.NamespacedName,
	data map[string]string,
) []metav1.Condition {
	actual := &corev1.Secret{}
	found, err := common.GetResource(ctx, c, nsName, "Secret", actual)
	if err != nil {
		return []metav1.Condition{{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		}}
	}
	if !found {
		actual = nil
	}

	ownerRef, err := buildWandbOwnerRef(c, wandb)
	if err != nil {
		return []metav1.Condition{{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ControllerErrorReason,
		}}
	}

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            nsName.Name,
			Namespace:       nsName.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: data,
	}

	if _, err = common.CrudResource(ctx, c, desired, actual); err != nil {
		return []metav1.Condition{{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		}}
	}
	return nil
}
