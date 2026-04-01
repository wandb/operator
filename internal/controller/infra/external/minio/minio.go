package minio

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external"
	"github.com/wandb/operator/internal/controller/translator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ConnectionSecretName = "wandb-minio-connection"

func WriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) ([]metav1.Condition, *translator.MinioConnection) {
	spec := wandb.Spec.Minio.ExternalMinio
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"url":       spec.URL,
		"Endpoint":  spec.Endpoint,
		"AccessKey": spec.AccessKey,
		"SecretKey": spec.SecretKey,
		"Bucket":    spec.Bucket,
		"Region":    spec.Region,
	}

	data, err := external.ResolveFields(ctx, c, wandb.Namespace, fields)
	if err != nil {
		logger.Error(err, "failed to resolve external minio fields")
		return []metav1.Condition{{
			Type:   "Reconciled",
			Status: metav1.ConditionFalse,
			Reason: "ApiError",
		}}, nil
	}

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: ConnectionSecretName}
	if conditions := external.WriteConnectionSecret(ctx, c, wandb, nsName, data); conditions != nil {
		return conditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return nil, &translator.MinioConnection{
		URL:       corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Endpoint:  corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Endpoint", Optional: ptr.To(false)},
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
