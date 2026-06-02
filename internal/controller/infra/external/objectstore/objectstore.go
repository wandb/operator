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

func connectionSecretName(key string) string {
	if key == "" || key == apiv2.DefaultInstanceName {
		return ConnectionSecretName
	}
	return fmt.Sprintf("%s-%s", ConnectionSecretName, key)
}

func WriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec *apiv2.ObjectStoreConnection,
) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
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
		Path:   data["Bucket"],
	}

	if _, ok := data["Host"]; ok {
		if _, ok := data["Port"]; ok {
			bucketUrl.Host = fmt.Sprintf("%s:%s", data["Host"], data["Port"])
		} else {
			bucketUrl.Host = data["Host"]
		}
	}

	if _, ok := data["AccessKey"]; ok {
		bucketUrl.User = url.UserPassword(data["AccessKey"], data["SecretKey"])
	}

	data["url"] = bucketUrl.String()

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: connectionSecretName(key)}
	if conditions := external.WriteConnectionSecret(ctx, c, wandb, nsName, data); conditions != nil {
		return conditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	// ResolveFields only writes non-empty values, so any field that is
	// legitimately absent for some deployment must be optional: Host (plain
	// AWS S3 with no custom endpoint), AccessKey/SecretKey (IAM-role /
	// workload-identity auth), Region (MinIO or region supplied out-of-band).
	// url and Bucket are always written.
	return nil, &apiv2.ObjectStoreConnection{
		URL:       corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Endpoint:  corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(true)},
		Port:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(true)},
		AccessKey: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "AccessKey", Optional: ptr.To(true)},
		SecretKey: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SecretKey", Optional: ptr.To(true)},
		Bucket:    corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Bucket", Optional: ptr.To(false)},
		Region:    corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Region", Optional: ptr.To(true)},
	}
}

func ReadState(
	_ context.Context,
	_ client.Client,
	_ *apiv2.WeightsAndBiases,
	_ string,
	newConditions []metav1.Condition,
) []metav1.Condition {
	return newConditions
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      connectionSecretName(key),
	})
}
