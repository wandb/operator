package clickhouse

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ConnectionSecretName = "wandb-clickhouse-connection"

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
	spec *apiv2.ClickHouseConnection,
) []metav1.Condition {
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"url":      spec.URL,
		"Host":     spec.Host,
		"Port":     spec.Port,
		"User":     spec.Username,
		"Password": spec.Password,
		"Database": spec.Database,
	}

	data, err := external.ResolveFields(ctx, c, wandb.Namespace, fields)
	if err != nil {
		logger.Error(err, "failed to resolve external clickhouse fields")
		return []metav1.Condition{{
			Type:   "Reconciled",
			Status: metav1.ConditionFalse,
			Reason: "ApiError",
		}}
	}

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: connectionSecretName(key)}
	return external.WriteConnectionSecret(ctx, c, wandb, nsName, data)
}

func ReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.ClickHouseConnection) {
	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: connectionSecretName(key)}
	_, conditions, found := external.ReadConnectionSecret(ctx, c, nsName, newConditions)
	if !found {
		return conditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return conditions, &apiv2.ClickHouseConnection{
		URL:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Host:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		Username: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "User", Optional: ptr.To(false)},
		Password: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Password", Optional: ptr.To(false)},
		Database: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Database", Optional: ptr.To(false)},
	}
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      connectionSecretName(key),
	})
}
