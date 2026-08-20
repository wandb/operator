package clickhouse

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	fields := map[string]apiv2.ValueOrSecret{
		"url":      spec.URL,
		"Host":     spec.Host,
		"HTTPPort": spec.HTTPPort,
		"TCPPort":  spec.TCPPort,
		"User":     spec.Username,
		"Password": spec.Password,
		"Database": spec.Database,
	}

	data, err := external.ResolveValueFields(ctx, c, wandb.Namespace, fields)
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

	return conditions, &apiv2.ClickHouseConnection{
		URL:      apiv2.ValueFromSecret(nsName.Name, "url", false),
		Host:     apiv2.ValueFromSecret(nsName.Name, "Host", false),
		HTTPPort: apiv2.ValueFromSecret(nsName.Name, "HTTPPort", false),
		TCPPort:  apiv2.ValueFromSecret(nsName.Name, "TCPPort", false),
		Username: apiv2.ValueFromSecret(nsName.Name, "User", false),
		Password: apiv2.ValueFromSecret(nsName.Name, "Password", false),
		Database: apiv2.ValueFromSecret(nsName.Name, "Database", false),
	}
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      connectionSecretName(key),
	})
}
