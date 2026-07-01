package bufstream

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStorageConnectionS3Compatible(t *testing.T) {
	// Managed SeaweedFS shape: s3://ak:sk@host:port/bucket?tls=false plus a
	// discrete Region key.
	data := map[string][]byte{
		"url":    []byte("s3://ak:sk@seaweedfs.wandb.svc.cluster.local:8333/wandb-bucket?tls=false"),
		"Region": []byte("us-east-1"),
	}

	info, err := parseStorageConnection(data)
	require.NoError(t, err)
	require.Equal(t, providerS3, info.Provider)
	require.Equal(t, "wandb-bucket", info.Bucket)
	require.Equal(t, "s3://wandb-bucket", info.URI)
	require.Equal(t, "http://seaweedfs.wandb.svc.cluster.local:8333", info.Endpoint)
	require.Equal(t, "us-east-1", info.Region)
	require.Equal(t, "ak", info.AccessKey)
	require.Equal(t, "sk", info.SecretKey)
	require.True(t, info.ForcePathStyle)
}

func TestParseStorageConnectionS3TLS(t *testing.T) {
	data := map[string][]byte{
		"url": []byte("s3://ak:sk@minio.example.com:9000/bucket?tls=true"),
	}
	info, err := parseStorageConnection(data)
	require.NoError(t, err)
	require.Equal(t, "https://minio.example.com:9000", info.Endpoint)
}

func TestParseStorageConnectionAWS(t *testing.T) {
	// AWS S3 with IAM role: no host, no credentials.
	data := map[string][]byte{
		"url":    []byte("s3://my-bucket"),
		"Region": []byte("us-west-2"),
	}
	info, err := parseStorageConnection(data)
	require.NoError(t, err)
	require.Equal(t, providerS3, info.Provider)
	require.Equal(t, "my-bucket", info.Bucket)
	require.Empty(t, info.Endpoint)
	require.False(t, info.ForcePathStyle)
	require.False(t, info.hasStaticCredentials())
}

func TestParseStorageConnectionDiscreteCredFallback(t *testing.T) {
	data := map[string][]byte{
		"url":       []byte("s3://host:9000/bucket"),
		"AccessKey": []byte("ak"),
		"SecretKey": []byte("sk"),
	}
	info, err := parseStorageConnection(data)
	require.NoError(t, err)
	require.Equal(t, "ak", info.AccessKey)
	require.Equal(t, "sk", info.SecretKey)
}

func TestParseStorageConnectionGCS(t *testing.T) {
	data := map[string][]byte{
		"url": []byte("gs://wandb-bucket/some/prefix"),
	}
	info, err := parseStorageConnection(data)
	require.NoError(t, err)
	require.Equal(t, providerGCS, info.Provider)
	require.Equal(t, "wandb-bucket", info.Bucket)
	require.Equal(t, "gs://wandb-bucket/some/prefix", info.URI)
}

func TestParseStorageConnectionAzureHTTPS(t *testing.T) {
	data := map[string][]byte{
		"url": []byte("https://acct.blob.core.windows.net/container/prefix"),
	}
	info, err := parseStorageConnection(data)
	require.NoError(t, err)
	require.Equal(t, providerAzure, info.Provider)
	require.Equal(t, "container", info.Bucket)
	require.Equal(t, "https://acct.blob.core.windows.net/container/prefix", info.URI)
}

func TestParseStorageConnectionAzureScheme(t *testing.T) {
	data := map[string][]byte{
		"url": []byte("az://acct:key@acct/container/prefix"),
	}
	info, err := parseStorageConnection(data)
	require.NoError(t, err)
	require.Equal(t, providerAzure, info.Provider)
	require.Equal(t, "container", info.Bucket)
	require.Equal(t, "https://acct.blob.core.windows.net/container/prefix", info.URI)
	require.Equal(t, "acct", info.AccessKey)
	require.Equal(t, "key", info.SecretKey)
}

func TestParseStorageConnectionErrors(t *testing.T) {
	_, err := parseStorageConnection(map[string][]byte{})
	require.Error(t, err)

	_, err = parseStorageConnection(map[string][]byte{"url": []byte("ftp://nope/bucket")})
	require.Error(t, err)
}
