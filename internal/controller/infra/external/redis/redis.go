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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	fields := map[string]apiv2.ValueOrSecret{
		"Host":     spec.Host,
		"Port":     spec.Port,
		"Password": spec.Password,
		"Tls":      spec.Tls,
		"SslCa":    spec.SslCa,
	}

	data, err := external.ResolveValueFields(ctx, c, wandb.Namespace, fields)
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

	return conditions, &apiv2.RedisConnection{
		URL:      apiv2.ValueFromSecret(nsName.Name, "url", false),
		Host:     apiv2.ValueFromSecret(nsName.Name, "Host", false),
		Port:     apiv2.ValueFromSecret(nsName.Name, "Port", false),
		Password: apiv2.ValueFromSecret(nsName.Name, "Password", true),
		Tls:      apiv2.ValueFromSecret(nsName.Name, "Tls", true),
		SslCa:    apiv2.ValueFromSecret(nsName.Name, "SslCa", true),
	}
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      connectionSecretName(key),
	})
}
