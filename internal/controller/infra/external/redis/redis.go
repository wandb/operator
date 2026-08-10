package redis

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ConnectionSecretName = "wandb-redis-connection"
const caCertPath = "/etc/ssl/certs/redis_ca.pem"

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
	spec *apiv2.RedisConnection,
) []metav1.Condition {
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"Host":     spec.Host,
		"Port":     spec.Port,
		"Password": spec.Password,
		"Tls":      spec.Tls,
		"SslCa":    spec.SslCa,
	}

	data, err := external.ResolveFields(ctx, c, wandb.Namespace, fields)
	if err != nil {
		logger.Error(err, "failed to resolve external redis fields")
		return []metav1.Condition{{
			Type:    common.ReconciledType,
			Status:  metav1.ConditionFalse,
			Reason:  common.ApiErrorReason,
			Message: err.Error(),
		}}
	}

	if err := validateConnectionData(data); err != nil {
		logger.Error(err, "invalid external redis connection")
		return []metav1.Condition{{
			Type:    common.ReconciledType,
			Status:  metav1.ConditionFalse,
			Reason:  common.ResourceErrorReason,
			Message: err.Error(),
		}}
	}

	redisUrl := url.URL{
		Scheme: "redis",
		Host:   fmt.Sprintf("%s:%s", data["Host"], data["Port"]),
	}

	if _, ok := data["Password"]; ok {
		redisUrl.User = url.UserPassword(data["Password"], "")
	}

	if _, ok := data["Tls"]; ok {
		values := redisUrl.Query()
		values.Add("tls", data["Tls"])
		redisUrl.RawQuery = values.Encode()
	}
	if _, ok := data["SslCa"]; ok {
		values := redisUrl.Query()
		if values.Get("tls") == "" {
			values.Set("tls", "true")
		}
		values.Set("caCertPath", caCertPath)
		redisUrl.RawQuery = values.Encode()
	}

	data["url"] = redisUrl.String()

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: connectionSecretName(key)}
	return external.WriteConnectionSecret(ctx, c, wandb, nsName, data)
}

func validateConnectionData(data map[string]string) error {
	host := strings.TrimSpace(data["Host"])
	if host == "" {
		return fmt.Errorf("external Redis host is empty")
	}

	portValue := strings.TrimSpace(data["Port"])
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("external Redis port %q must be an integer between 1 and 65535", portValue)
	}

	data["Host"] = host
	data["Port"] = strconv.Itoa(port)
	return nil
}

func ReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.RedisConnection) {
	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: connectionSecretName(key)}
	_, conditions, found := external.ReadConnectionSecret(ctx, c, nsName, newConditions)
	if !found {
		return conditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return conditions, &apiv2.RedisConnection{
		URL:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Host:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		Password: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Password", Optional: ptr.To(true)},
		Tls:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Tls", Optional: ptr.To(true)},
		SslCa:    corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SslCa", Optional: ptr.To(true)},
	}
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      connectionSecretName(key),
	})
}
