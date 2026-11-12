package mysql

import (
	"context"
	"fmt"
	"net/url"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ConnectionSecretName = "wandb-mysql-connection"
const caCertPath = "/etc/ssl/certs/mysql_ca.pem"
const sslCertPath = "/etc/ssl/certs/mysql_ssl_cert.pem"
const sslKeyPath = "/etc/ssl/certs/mysql_ssl_key.pem"

// connectionSecretName returns the connection secret name for an instance. The
// reserved default instance keeps the historical name for backward
// compatibility; other instances are suffixed with their key.
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
	spec *apiv2.MysqlConnection,
) []metav1.Condition {
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]apiv2.ValueOrSecret{
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

	data, err := external.ResolveValueFields(ctx, c, wandb.Namespace, fields)
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
	values := dbUrl.Query()
	if tls, ok := data["Tls"]; ok {
		values.Set("tls", tls)
	}
	if _, ok := data["SslCa"]; ok {
		if values.Get("tls") == "" {
			values.Set("tls", "custom")
		}
		values.Set("ssl-ca", caCertPath)
	}
	if _, ok := data["SslCert"]; ok {
		values.Set("ssl-cert", sslCertPath)
	}
	if _, ok := data["SslKey"]; ok {
		values.Set("ssl-key", sslKeyPath)
	}
	dbUrl.RawQuery = values.Encode()

	data["url"] = dbUrl.String()

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: connectionSecretName(key)}
	return external.WriteConnectionSecret(ctx, c, wandb, nsName, data)
}

func ReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.MysqlConnection) {
	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: connectionSecretName(key)}
	_, conditions, found := external.ReadConnectionSecret(ctx, c, nsName, newConditions)
	if !found {
		return conditions, nil
	}

	return conditions, &apiv2.MysqlConnection{
		URL:      apiv2.ValueFromSecret(nsName.Name, "url", false),
		Host:     apiv2.ValueFromSecret(nsName.Name, "Host", false),
		Port:     apiv2.ValueFromSecret(nsName.Name, "Port", false),
		Database: apiv2.ValueFromSecret(nsName.Name, "Database", false),
		Username: apiv2.ValueFromSecret(nsName.Name, "Username", false),
		Password: apiv2.ValueFromSecret(nsName.Name, "Password", false),
		Tls:      apiv2.ValueFromSecret(nsName.Name, "Tls", true),
		SslCa:    apiv2.ValueFromSecret(nsName.Name, "SslCa", true),
		SslCert:  apiv2.ValueFromSecret(nsName.Name, "SslCert", true),
		SslKey:   apiv2.ValueFromSecret(nsName.Name, "SslKey", true),
	}
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      connectionSecretName(key),
	})
}
