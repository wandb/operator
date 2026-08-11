package mysql

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external"
	"github.com/wandb/operator/internal/controller/infra/mysqlconnection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func WriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec *apiv2.MysqlConnection,
) []metav1.Condition {
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
		return []metav1.Condition{
			{
				Type:    mysqlconnection.ProviderReadyType,
				Status:  metav1.ConditionFalse,
				Reason:  "SourceSecretsUnavailable",
				Message: err.Error(),
			},
			{
				Type:    mysqlconnection.ConnectionResolvedType,
				Status:  metav1.ConditionFalse,
				Reason:  "SourceSecretsUnavailable",
				Message: err.Error(),
			},
			{
				Type:   mysqlconnection.BundleReadyType,
				Status: metav1.ConditionFalse,
				Reason: "ConnectionNotResolved",
			},
			{
				Type:   "Reconciled",
				Status: metav1.ConditionFalse,
				Reason: "ApiError",
			},
		}
	}

	material := mysqlconnection.Material{
		Host:       data["Host"],
		Port:       data["Port"],
		Database:   data["Database"],
		Username:   data["Username"],
		Password:   data["Password"],
		TLS:        data["Tls"],
		CACert:     []byte(data["SslCa"]),
		ClientCert: []byte(data["SslCert"]),
		ClientKey:  []byte(data["SslKey"]),
	}
	if _, err := mysqlconnection.Write(ctx, c, wandb, key, material); err != nil {
		logger.Error(err, "failed to write external mysql connection bundle")
		return []metav1.Condition{
			{
				Type:   mysqlconnection.ProviderReadyType,
				Status: metav1.ConditionTrue,
				Reason: "SourceSecretsResolved",
			},
			{
				Type:    mysqlconnection.ConnectionResolvedType,
				Status:  metav1.ConditionFalse,
				Reason:  "InvalidConnection",
				Message: err.Error(),
			},
			{
				Type:   mysqlconnection.BundleReadyType,
				Status: metav1.ConditionFalse,
				Reason: "InvalidConnection",
			},
			{
				Type:   "Reconciled",
				Status: metav1.ConditionFalse,
				Reason: "InvalidConnection",
			},
		}
	}

	return []metav1.Condition{
		{
			Type:   mysqlconnection.ProviderReadyType,
			Status: metav1.ConditionTrue,
			Reason: "SourceSecretsResolved",
		},
		{
			Type:   mysqlconnection.ConnectionResolvedType,
			Status: metav1.ConditionTrue,
			Reason: "SourceSecretsResolved",
		},
		{
			Type:   mysqlconnection.BundleReadyType,
			Status: metav1.ConditionTrue,
			Reason: "BundleWritten",
		},
	}
}

func ReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.MysqlConnection) {
	connection, err := mysqlconnection.Read(ctx, c, wandb, key)
	if err != nil {
		return append(newConditions, metav1.Condition{
			Type:   "Reconciled",
			Status: metav1.ConditionFalse,
			Reason: "ApiError",
		}), nil
	}
	return newConditions, connection
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string) error {
	return mysqlconnection.Delete(ctx, c, wandb, key)
}
