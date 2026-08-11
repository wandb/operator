package mysqlconnection_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysqlconnection"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNormalize(t *testing.T) {
	caCert, clientCert, clientKey := testCertificates(t)
	base := mysqlconnection.Material{
		Host:     "mysql.example.com",
		Port:     "3306",
		Database: "wandb",
		Username: "wandb",
		Password: "secret",
	}

	tests := []struct {
		name      string
		configure func(*mysqlconnection.Material)
		wantTLS   string
		wantError string
	}{
		{name: "plaintext", wantTLS: ""},
		{
			name: "true uses driver TLS",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "true"
			},
			wantTLS: "true",
		},
		{
			name: "skip verify",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "skip-verify"
			},
			wantTLS: "skip-verify",
		},
		{
			name: "preferred",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "preferred"
			},
			wantTLS: "preferred",
		},
		{
			name: "false disables TLS",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "false"
			},
			wantTLS: "false",
		},
		{
			name: "CA enables custom TLS",
			configure: func(material *mysqlconnection.Material) {
				material.CACert = caCert
			},
			wantTLS: "custom",
		},
		{
			name: "true with CA becomes custom TLS",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "true"
				material.CACert = caCert
			},
			wantTLS: "custom",
		},
		{
			name: "valid client certificate pair",
			configure: func(material *mysqlconnection.Material) {
				material.CACert = caCert
				material.ClientCert = clientCert
				material.ClientKey = clientKey
			},
			wantTLS: "custom",
		},
		{
			name: "custom requires CA",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "custom"
			},
			wantError: "sslCa is required",
		},
		{
			name: "client certificate requires key",
			configure: func(material *mysqlconnection.Material) {
				material.CACert = caCert
				material.ClientCert = clientCert
			},
			wantError: "configured together",
		},
		{
			name: "preferred rejects CA",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "preferred"
				material.CACert = caCert
			},
			wantError: "cannot be configured",
		},
		{
			name: "false rejects CA",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "false"
				material.CACert = caCert
			},
			wantError: "cannot be configured",
		},
		{
			name: "invalid CA",
			configure: func(material *mysqlconnection.Material) {
				material.CACert = []byte("not PEM")
			},
			wantError: "valid PEM",
		},
		{
			name: "invalid port",
			configure: func(material *mysqlconnection.Material) {
				material.Port = "70000"
			},
			wantError: "between 1 and 65535",
		},
		{
			name: "unsupported future TLS mode",
			configure: func(material *mysqlconnection.Material) {
				material.TLS = "verify-identity"
				material.CACert = caCert
			},
			wantError: "unsupported tls mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			material := base
			if tt.configure != nil {
				tt.configure(&material)
			}

			got, err := mysqlconnection.Normalize(material)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTLS, got.TLS)
		})
	}
}

func TestURL(t *testing.T) {
	caCert, _, _ := testCertificates(t)
	connectionURL, err := mysqlconnection.URL(mysqlconnection.Material{
		Host:     "2001:db8::1",
		Port:     "3306",
		Database: "wandb/db",
		Username: "user@example.com",
		Password: "p@ss:/?#word",
		CACert:   caCert,
	}, "analytics")
	require.NoError(t, err)

	parsed, err := url.Parse(connectionURL)
	require.NoError(t, err)
	assert.Equal(t, "[2001:db8::1]:3306", parsed.Host)
	assert.Equal(t, "user@example.com", parsed.User.Username())
	password, ok := parsed.User.Password()
	require.True(t, ok)
	assert.Equal(t, "p@ss:/?#word", password)
	assert.Equal(t, "/wandb/db", parsed.Path)
	assert.Contains(t, connectionURL, "/wandb%2Fdb")
	assert.Equal(t, "custom", parsed.Query().Get("tls"))
	assert.Equal(
		t,
		mysqlconnection.MountPath("analytics")+"/"+mysqlconnection.CACertFile,
		parsed.Query().Get("ssl-ca"),
	)
}

func TestWrite(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "test",
			UID:       types.UID("wandb-uid"),
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb).Build()

	connection, err := mysqlconnection.Write(t.Context(), client, wandb, "analytics", mysqlconnection.Material{
		Host:     "mysql.example.com",
		Port:     "3306",
		Database: "wandb",
		Username: "wandb",
		Password: "secret",
	})
	require.NoError(t, err)
	assert.Equal(t, mysqlconnection.SecretName(wandb.Name, "analytics"), connection.URL.Name)

	secret := &corev1.Secret{}
	require.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      connection.URL.Name,
	}, secret))
	assert.Equal(t, mysqlconnection.BundleVersion, secret.Annotations[mysqlconnection.BundleVersionAnnotation])
	assert.Equal(t, "secret", string(secret.Data[mysqlconnection.PasswordKey]))
	for _, key := range []string{
		mysqlconnection.URLKey,
		mysqlconnection.HostKey,
		mysqlconnection.PortKey,
		mysqlconnection.DatabaseKey,
		mysqlconnection.UsernameKey,
		mysqlconnection.PasswordKey,
		mysqlconnection.TLSKey,
		mysqlconnection.SSLCAKey,
		mysqlconnection.SSLCertKey,
		mysqlconnection.SSLKeyKey,
	} {
		assert.Contains(t, secret.Data, key)
	}
	assert.NotEqual(
		t,
		mysqlconnection.SecretName(wandb.Name, "analytics"),
		mysqlconnection.SecretName(wandb.Name, "default"),
	)
}

func TestDeleteStale(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "wandb", Namespace: "test", UID: types.UID("current-uid"),
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb).Build()
	material := mysqlconnection.Material{
		Host: "mysql.example.com", Port: "3306", Database: "wandb", Username: "wandb", Password: "secret",
	}
	_, err := mysqlconnection.Write(t.Context(), client, wandb, "default", material)
	require.NoError(t, err)
	_, err = mysqlconnection.Write(t.Context(), client, wandb, "analytics", material)
	require.NoError(t, err)

	foreign := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: "wandb-foreign-mysql-connection", Namespace: wandb.Namespace,
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "wandb-operator",
			"app.kubernetes.io/instance":   wandb.Name,
			"app.kubernetes.io/component":  "mysql-connection",
		},
		OwnerReferences: []metav1.OwnerReference{{UID: types.UID("old-uid")}},
	}}
	require.NoError(t, client.Create(t.Context(), foreign))

	require.NoError(t, mysqlconnection.DeleteStale(
		t.Context(), client, wandb, map[string]struct{}{"default": {}},
	))

	assert.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Namespace: wandb.Namespace, Name: mysqlconnection.SecretName(wandb.Name, "default"),
	}, &corev1.Secret{}))
	assert.Error(t, client.Get(t.Context(), types.NamespacedName{
		Namespace: wandb.Namespace, Name: mysqlconnection.SecretName(wandb.Name, "analytics"),
	}, &corev1.Secret{}))
	assert.NoError(t, client.Get(t.Context(), types.NamespacedName{
		Namespace: wandb.Namespace, Name: foreign.Name,
	}, &corev1.Secret{}))
}

func testCertificates(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	clientDER, err := x509.CreateCertificate(
		rand.Reader,
		clientTemplate,
		caTemplate,
		&clientKey.PublicKey,
		caKey,
	)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})
}
