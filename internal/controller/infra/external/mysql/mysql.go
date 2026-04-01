package mysql

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

const ConnectionSecretName = "wandb-mysql-connection"

func WriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.MySQL.ExternalMysql
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"url":      spec.URL,
		"Host":     spec.Host,
		"Port":     spec.Port,
		"Database": spec.Database,
		"Username": spec.Username,
		"Password": spec.Password,
		"Tls":      spec.Tls,
		"SslCa":    spec.SslCa,
		"SslCert":  spec.SslCert,
		"SslKey":   spec.SslKey,
	}

	data, err := external.ResolveFields(ctx, c, wandb.Namespace, fields)
	if err != nil {
		logger.Error(err, "failed to resolve external mysql fields")
		return []metav1.Condition{{
			Type:   "Reconciled",
			Status: metav1.ConditionFalse,
			Reason: "ApiError",
		}}
	}

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: ConnectionSecretName}
	return external.WriteConnectionSecret(ctx, c, wandb, nsName, data)
}

func ReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.MysqlConnection) {
	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: ConnectionSecretName}
	_, conditions, found := external.ReadConnectionSecret(ctx, c, nsName, newConditions)
	if !found {
		return conditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return conditions, &translator.MysqlConnection{
		URL:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Host:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		Database: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Database", Optional: ptr.To(false)},
		Username: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Username", Optional: ptr.To(false)},
		Password: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Password", Optional: ptr.To(false)},
	}
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      ConnectionSecretName,
	})
}
