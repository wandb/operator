package mysql

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysqlconnection"
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
			"SslCa":    testCACertificate(t),
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
				},
			}},
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, source).Build()

	conditions := WriteState(context.Background(), client, wandb, apiv2.DefaultInstanceName, wandb.Spec.MySQL[apiv2.DefaultInstanceName].ExternalMysql)
	require.Len(t, conditions, 3)
	for _, conditionType := range []string{
		mysqlconnection.ProviderReadyType,
		mysqlconnection.ConnectionResolvedType,
		mysqlconnection.BundleReadyType,
	} {
		require.Condition(t, func() bool {
			for _, condition := range conditions {
				if condition.Type == conditionType {
					return condition.Status == metav1.ConditionTrue
				}
			}
			return false
		})
	}

	written := &corev1.Secret{}
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{
		Name:      mysqlconnection.SecretName(wandb.Name, apiv2.DefaultInstanceName),
		Namespace: "default",
	}, written))
	data := mysqlConnectionData(written)
	parsed, err := url.Parse(data["url"])
	require.NoError(t, err)
	require.Equal(t, "mysql", parsed.Scheme)
	require.Equal(t, "mysql.example.com:3306", parsed.Host)
	require.Equal(t, "/wandb", parsed.Path)
	require.Equal(t, "custom", parsed.Query().Get("tls"))
	require.Equal(
		t,
		mysqlconnection.MountPath(apiv2.DefaultInstanceName)+"/"+mysqlconnection.CACertFile,
		parsed.Query().Get("ssl-ca"),
	)
	require.Empty(t, parsed.Query().Get("ssl-cert"))
	require.Equal(t, mysqlconnection.BundleVersion, written.Annotations[mysqlconnection.BundleVersionAnnotation])
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

func testCACertificate(t *testing.T) []byte {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
}
