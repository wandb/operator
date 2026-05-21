package mysql

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

const ConnectionSecretName = "wandb-mysql-connection"

func WriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.MySQL.ExternalMysql
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
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

	dbUrl := url.URL{
		Scheme: "mysql",
		Host:   fmt.Sprintf("%s:%s", data["Host"], data["Port"]),
		User:   url.UserPassword(data["Username"], data["Password"]),
		Path:   data["Database"],
	}

	data["url"] = dbUrl.String()

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: ConnectionSecretName}
	return external.WriteConnectionSecret(ctx, c, wandb, nsName, data)
}

func ReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.MysqlConnection) {
	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: ConnectionSecretName}
	_, conditions, found := external.ReadConnectionSecret(ctx, c, nsName, newConditions)
	if !found {
		return conditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return conditions, &apiv2.MysqlConnection{
		URL:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Host:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		Database: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Database", Optional: ptr.To(false)},
		Username: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Username", Optional: ptr.To(false)},
		Password: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Password", Optional: ptr.To(false)},
		Tls:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Tls", Optional: ptr.To(true)},
		SslCa:    corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SslCa", Optional: ptr.To(true)},
		SslCert:  corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SslCert", Optional: ptr.To(true)},
		SslKey:   corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SslKey", Optional: ptr.To(true)},
	}
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      ConnectionSecretName,
	})
}
