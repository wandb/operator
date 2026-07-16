package objectstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv2 "github.com/wandb/operator/api/v2"
)

const connSecretName = "wandb-objectstore-connection"

// resolveFixture builds a fake client holding a single connection secret with
// the given keys and an ObjectStoreConnection whose selectors point at them.
func resolveFixture(t *testing.T, data map[string]string) (*apiv2.ObjectStoreConnection, ConnInfo, error) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	raw := map[string][]byte{}
	for k, v := range data {
		raw[k] = []byte(v)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: connSecretName, Namespace: "default"},
		Data:       raw,
	}

	connSel := func(key string) corev1.SecretKeySelector {
		return corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: connSecretName},
			Key:                  key,
		}
	}
	conn := &apiv2.ObjectStoreConnection{
		Provider:       connSel("Provider"),
		Endpoint:       connSel("Host"),
		Port:           connSel("Port"),
		AccessKey:      connSel("AccessKey"),
		SecretKey:      connSel("SecretKey"),
		Bucket:         connSel("Bucket"),
		Path:           connSel("Path"),
		Region:         connSel("Region"),
		TlsEnabled:     connSel("TlsEnabled"),
		ForcePathStyle: connSel("ForcePathStyle"),
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	info, err := Resolve(context.Background(), c, "default", conn)
	return conn, info, err
}

func TestResolve_NilConnection(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	_, err := Resolve(context.Background(), c, "default", nil)
	require.Error(t, err)
}

func TestResolve_ExternalS3WithStaticCredentials(t *testing.T) {
	conn, info, err := resolveFixture(t, map[string]string{
		"Provider":       "s3",
		"Host":           "minio.local",
		"Port":           "9000",
		"AccessKey":      "minio",
		"SecretKey":      "minio123",
		"Bucket":         "my-bucket",
		"Path":           "team/prefix",
		"Region":         "us-east-1",
		"TlsEnabled":     "true",
		"ForcePathStyle": "true",
	})
	require.NoError(t, err)

	require.Equal(t, apiv2.ObjectStoreProviderS3, info.Provider)
	require.Equal(t, "minio.local", info.Endpoint)
	require.Equal(t, "9000", info.Port)
	require.Equal(t, "minio", info.AccessKey)
	require.Equal(t, "minio123", info.SecretKey)
	require.Equal(t, "my-bucket", info.Bucket)
	require.Equal(t, "team/prefix", info.Path)
	require.Equal(t, "us-east-1", info.Region)
	require.True(t, info.TlsEnabled)
	require.True(t, info.ForcePathStyle)
	require.True(t, info.HasStaticCredentials())

	// The credential selectors are preserved for consumers that inject by ref.
	require.Equal(t, conn.AccessKey, info.AccessKeyRef)
	require.Equal(t, conn.SecretKey, info.SecretKeyRef)
}

func TestResolve_AmbientCredentials(t *testing.T) {
	_, info, err := resolveFixture(t, map[string]string{
		"Provider": "s3",
		"Bucket":   "my-bucket",
		"Region":   "us-west-2",
	})
	require.NoError(t, err)

	require.Empty(t, info.AccessKey)
	require.Empty(t, info.SecretKey)
	require.False(t, info.HasStaticCredentials())
	require.Equal(t, "us-west-2", info.Region)
}

func TestResolve_ForcePathStyleFallbackFromEndpoint(t *testing.T) {
	// A custom endpoint with no ForcePathStyle key falls back to RequiresPathStyle.
	_, info, err := resolveFixture(t, map[string]string{
		"Host":   "minio.local",
		"Bucket": "my-bucket",
	})
	require.NoError(t, err)
	require.True(t, info.ForcePathStyle)

	// No endpoint (native AWS) means virtual-hosted addressing.
	_, info, err = resolveFixture(t, map[string]string{
		"Bucket": "my-bucket",
	})
	require.NoError(t, err)
	require.False(t, info.ForcePathStyle)
}

func TestResolve_MissingTlsDefaultsFalse(t *testing.T) {
	_, info, err := resolveFixture(t, map[string]string{
		"Host":   "minio.local",
		"Bucket": "my-bucket",
	})
	require.NoError(t, err)
	require.False(t, info.TlsEnabled)
}
