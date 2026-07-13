package mysql

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const mysqlSourceSecretName = "external-mysql"

func mysqlSel(key string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: mysqlSourceSecretName},
		Key:                  key,
	}
}

func TestWriteStateAddsCustomTLSParamsWhenCACertPresent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	source := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: mysqlSourceSecretName, Namespace: "default"},
		Data: map[string][]byte{
			"Host":     []byte("mysql.example.com"),
			"Port":     []byte("3306"),
			"Database": []byte("wandb"),
			"Username": []byte("wandb"),
			"Password": []byte("secret"),
			"SslCa":    []byte("---ca---"),
			"SslCert":  []byte("---cert---"),
			"SslKey":   []byte("---key---"),
		},
	}
	wandb := &apiv2.WeightsAndBiases{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			MySQL: map[string]apiv2.MySQLSpec{apiv2.DefaultInstanceName: {
				ExternalMysql: &apiv2.MysqlConnection{
					Host:     mysqlSel("Host"),
					Port:     mysqlSel("Port"),
					Database: mysqlSel("Database"),
					Username: mysqlSel("Username"),
					Password: mysqlSel("Password"),
					SslCa:    mysqlSel("SslCa"),
					SslCert:  mysqlSel("SslCert"),
					SslKey:   mysqlSel("SslKey"),
				},
			}},
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, source).Build()

	conditions := WriteState(context.Background(), client, wandb, apiv2.DefaultInstanceName, wandb.Spec.MySQL[apiv2.DefaultInstanceName].ExternalMysql)
	require.Nil(t, conditions)

	written := &corev1.Secret{}
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: ConnectionSecretName, Namespace: "default"}, written))
	data := mysqlConnectionData(written)
	parsed, err := url.Parse(data["url"])
	require.NoError(t, err)
	require.Equal(t, "mysql", parsed.Scheme)
	require.Equal(t, "mysql.example.com:3306", parsed.Host)
	require.Equal(t, "/wandb", parsed.Path)
	require.Equal(t, "custom", parsed.Query().Get("tls"))
	require.Equal(t, caCertPath, parsed.Query().Get("ssl-ca"))
	require.Equal(t, sslCertPath, parsed.Query().Get("ssl-cert"))
	require.Equal(t, sslKeyPath, parsed.Query().Get("ssl-key"))
}

func mysqlConnectionData(secret *corev1.Secret) map[string]string {
	out := map[string]string{}
	for k, v := range secret.Data {
		out[k] = string(v)
	}
	for k, v := range secret.StringData {
		out[k] = v
	}
	return out
}
