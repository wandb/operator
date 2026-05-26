package objectstore

import (
	"context"
	"fmt"
	"net/url"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ConnectionSecretName = "wandb-objectstore-connection"

func WriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
	spec := wandb.Spec.ObjectStore.ExternalObjectStore
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"Host":      spec.Endpoint,
		"Port":      spec.Port,
		"AccessKey": spec.AccessKey,
		"SecretKey": spec.SecretKey,
		"Bucket":    spec.Bucket,
		"Region":    spec.Region,
	}

	data, err := external.ResolveFields(ctx, c, wandb.Namespace, fields)
	if err != nil {
		logger.Error(err, "failed to resolve external object store fields")
		return []metav1.Condition{{
			Type:   "Reconciled",
			Status: metav1.ConditionFalse,
			Reason: "ApiError",
		}}, nil
	}

	bucketUrl := url.URL{
		Scheme: "s3",
		Host:   fmt.Sprintf("%s:%s", data["Host"], data["Port"]),
		Path:   data["Bucket"],
	}

	if _, ok := data["AccessKey"]; ok {
		bucketUrl.User = url.UserPassword(data["AccessKey"], data["SecretKey"])
	}

	data["url"] = bucketUrl.String()

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: ConnectionSecretName}
	if conditions := external.WriteConnectionSecret(ctx, c, wandb, nsName, data); conditions != nil {
		return conditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return nil, &apiv2.ObjectStoreConnection{
		URL:       corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Endpoint:  corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(true)},
		AccessKey: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "AccessKey", Optional: ptr.To(false)},
		SecretKey: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SecretKey", Optional: ptr.To(false)},
		Bucket:    corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Bucket", Optional: ptr.To(false)},
		Region:    corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Region", Optional: ptr.To(false)},
	}
}

func ReadState(
	_ context.Context,
	_ client.Client,
	_ *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) []metav1.Condition {
	return newConditions
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      ConnectionSecretName,
	})
}
