package objectstore

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	apiv2 "github.com/wandb/operator/api/v2"
)

func TestParseSecretDataS3Compatible(t *testing.T) {
	// Managed SeaweedFS shape: s3://ak:sk@host:port/bucket?tls=false plus a
	// discrete Region key.
	data := map[string][]byte{
		"url":    []byte("s3://ak:sk@seaweedfs.wandb.svc.cluster.local:8333/wandb-bucket?tls=false"),
		"Region": []byte("us-east-1"),
	}

	info, err := ParseSecretData(data)
	require.NoError(t, err)
	require.Equal(t, apiv2.ObjectStoreProviderS3, info.Provider)
	require.Equal(t, "wandb-bucket", info.Bucket)
	require.Equal(t, "s3://wandb-bucket", info.URI)
	require.Equal(t, "http://seaweedfs.wandb.svc.cluster.local:8333", info.Endpoint)
	require.Equal(t, "us-east-1", info.Region)
	require.Equal(t, "ak", info.AccessKey)
	require.Equal(t, "sk", info.SecretKey)
	require.True(t, info.ForcePathStyle)
}

func TestParseSecretDataS3TLS(t *testing.T) {
	data := map[string][]byte{
		"url": []byte("s3://ak:sk@minio.example.com:9000/bucket?tls=true"),
	}
	info, err := ParseSecretData(data)
	require.NoError(t, err)
	require.Equal(t, "https://minio.example.com:9000", info.Endpoint)
}

func TestParseSecretDataAWS(t *testing.T) {
	// AWS S3 with IAM role: no host, no credentials.
	data := map[string][]byte{
		"url":    []byte("s3://my-bucket"),
		"Region": []byte("us-west-2"),
	}
	info, err := ParseSecretData(data)
	require.NoError(t, err)
	require.Equal(t, apiv2.ObjectStoreProviderS3, info.Provider)
	require.Equal(t, "my-bucket", info.Bucket)
	require.Empty(t, info.Endpoint)
	require.False(t, info.ForcePathStyle)
	require.False(t, info.HasStaticCredentials())
}

func TestParseSecretDataDiscreteCredFallback(t *testing.T) {
	data := map[string][]byte{
		"url":       []byte("s3://host:9000/bucket"),
		"AccessKey": []byte("ak"),
		"SecretKey": []byte("sk"),
	}
	info, err := ParseSecretData(data)
	require.NoError(t, err)
	require.Equal(t, "ak", info.AccessKey)
	require.Equal(t, "sk", info.SecretKey)
}

func TestParseSecretDataGCS(t *testing.T) {
	data := map[string][]byte{
		"url": []byte("gs://wandb-bucket/some/prefix"),
	}
	info, err := ParseSecretData(data)
	require.NoError(t, err)
	require.Equal(t, apiv2.ObjectStoreProviderGCS, info.Provider)
	require.Equal(t, "wandb-bucket", info.Bucket)
	require.Equal(t, "gs://wandb-bucket/some/prefix", info.URI)
}

func TestParseSecretDataAzureHTTPS(t *testing.T) {
	data := map[string][]byte{
		"url": []byte("https://acct.blob.core.windows.net/container/prefix"),
	}
	info, err := ParseSecretData(data)
	require.NoError(t, err)
	require.Equal(t, apiv2.ObjectStoreProviderAzure, info.Provider)
	require.Equal(t, "container", info.Bucket)
	require.Equal(t, "https://acct.blob.core.windows.net/container/prefix", info.URI)
}

func TestParseSecretDataAzureScheme(t *testing.T) {
	data := map[string][]byte{
		"url": []byte("az://acct:key@acct/container/prefix"),
	}
	info, err := ParseSecretData(data)
	require.NoError(t, err)
	require.Equal(t, apiv2.ObjectStoreProviderAzure, info.Provider)
	require.Equal(t, "container", info.Bucket)
	require.Equal(t, "https://acct.blob.core.windows.net/container/prefix", info.URI)
	require.Equal(t, "acct", info.AccessKey)
	require.Equal(t, "key", info.SecretKey)
}

func TestParseSecretDataErrors(t *testing.T) {
	_, err := ParseSecretData(map[string][]byte{})
	require.Error(t, err)

	_, err = ParseSecretData(map[string][]byte{"url": []byte("ftp://nope/bucket")})
	require.Error(t, err)
}

func TestToSecretData_ManagedSeaweedShape(t *testing.T) {
	// Mirrors the values the managed SeaweedFS path populates; the resulting
	// secret must contain exactly the historical key set (no Path).
	ci := ConnInfo{
		Provider:       apiv2.ObjectStoreProviderS3,
		URL:            "s3://ak:sk@object-store-s3.wandb.svc.cluster.local:8333/bucket?tls=false",
		Endpoint:       "object-store-s3.wandb.svc.cluster.local",
		Port:           "8333",
		AccessKey:      "ak",
		SecretKey:      "sk",
		Region:         "us-east-1",
		Bucket:         "bucket",
		Scheme:         "http",
		TlsEnabled:     false,
		ForcePathStyle: true,
	}

	require.Equal(t, map[string]string{
		"url":            "s3://ak:sk@object-store-s3.wandb.svc.cluster.local:8333/bucket?tls=false",
		"Host":           "object-store-s3.wandb.svc.cluster.local",
		"Port":           "8333",
		"AccessKey":      "ak",
		"SecretKey":      "sk",
		"Region":         "us-east-1",
		"Bucket":         "bucket",
		"Scheme":         "http",
		"TlsEnabled":     "false",
		"Provider":       "s3",
		"ForcePathStyle": "true",
	}, ci.ToSecretData())
}

func TestToSecretData_OmitsEmptyDiscreteKeys(t *testing.T) {
	ci := ConnInfo{
		Provider: apiv2.ObjectStoreProviderS3,
		URL:      "s3://my-bucket",
		Bucket:   "my-bucket",
	}
	data := ci.ToSecretData()
	require.Equal(t, "s3://my-bucket", data["url"])
	require.NotContains(t, data, "Host")
	require.NotContains(t, data, "AccessKey")
	require.NotContains(t, data, "SecretKey")
	require.NotContains(t, data, "Region")
	require.NotContains(t, data, "Path")
	require.NotContains(t, data, "Scheme")
	// TlsEnabled is always written; ForcePathStyle is written for S3.
	require.Equal(t, "false", data["TlsEnabled"])
	require.Equal(t, "false", data["ForcePathStyle"])
}

func TestToSecretData_ForcePathStyleIsS3Only(t *testing.T) {
	for _, provider := range []apiv2.ObjectStoreProvider{apiv2.ObjectStoreProviderGCS, apiv2.ObjectStoreProviderAzure} {
		ci := ConnInfo{Provider: provider, URL: "gs://b", Bucket: "b"}
		require.NotContains(t, ci.ToSecretData(), "ForcePathStyle", "path-style is an S3-only concept")
	}
}

func TestToObjectStoreConnection_RequireAll(t *testing.T) {
	ci := ConnInfo{
		Provider:       apiv2.ObjectStoreProviderS3,
		URL:            "s3://ak:sk@host:8333/bucket?tls=false",
		Endpoint:       "host",
		Port:           "8333",
		AccessKey:      "ak",
		SecretKey:      "sk",
		Region:         "us-east-1",
		Bucket:         "bucket",
		Scheme:         "http",
		ForcePathStyle: true,
	}
	conn := ci.ToObjectStoreConnection("conn-secret", true)

	// Every emitted selector points at conn-secret and is required.
	for _, s := range []corev1.SecretKeySelector{
		conn.URL, conn.Provider, conn.Endpoint, conn.Port, conn.AccessKey,
		conn.SecretKey, conn.Region, conn.Bucket, conn.TlsEnabled, conn.ForcePathStyle,
	} {
		require.Equal(t, "conn-secret", s.Name)
		require.NotNil(t, s.Optional)
		require.False(t, *s.Optional)
	}
	require.Equal(t, "Host", conn.Endpoint.Key)
	require.Equal(t, "url", conn.URL.Key)
	// Path is not written for the managed shape, so its selector stays empty.
	require.Empty(t, conn.Path.Name)
}

func TestToObjectStoreConnection_ExternalOptionality(t *testing.T) {
	ci := ConnInfo{
		Provider:  apiv2.ObjectStoreProviderS3,
		URL:       "s3://ak:sk@host:9000/bucket",
		Endpoint:  "host",
		Port:      "9000",
		AccessKey: "ak",
		SecretKey: "sk",
		Region:    "us-west-2",
		Bucket:    "bucket",
	}
	conn := ci.ToObjectStoreConnection("conn-secret", false)

	// url/Provider/Bucket are required...
	for _, s := range []corev1.SecretKeySelector{conn.URL, conn.Provider, conn.Bucket} {
		require.NotNil(t, s.Optional)
		require.False(t, *s.Optional)
	}
	// ...everything else is optional.
	for _, s := range []corev1.SecretKeySelector{conn.Endpoint, conn.Port, conn.AccessKey, conn.SecretKey, conn.Region} {
		require.NotNil(t, s.Optional)
		require.True(t, *s.Optional)
	}
}
