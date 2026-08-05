package bufstream

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/objectstore"
)

func TestRenderBufstreamConfigS3(t *testing.T) {
	storage := objectstore.ConnInfo{
		Provider:       apiv2.ObjectStoreProviderS3,
		URI:            "s3://wandb-bucket",
		Bucket:         "wandb-bucket",
		Endpoint:       "http://objstore.default.svc.cluster.local:8333",
		Region:         "us-east-1",
		AccessKey:      "ak",
		SecretKey:      "sk",
		ForcePathStyle: true,
	}

	etcdAddrs := []string{
		"my-kafka-etcd-0.my-kafka-etcd.default.svc.cluster.local:2379",
		"my-kafka-etcd-1.my-kafka-etcd.default.svc.cluster.local:2379",
		"my-kafka-etcd-2.my-kafka-etcd.default.svc.cluster.local:2379",
	}
	rendered, err := renderBufstreamConfig("my-kafka", "my-kafka.default.svc.cluster.local", etcdAddrs, storage)
	require.NoError(t, err)

	// Round-trips as valid YAML.
	var parsed bufstreamConfig
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &parsed))

	require.Equal(t, "v1beta1", parsed.Version)
	require.Equal(t, "my-kafka", parsed.Cluster)
	require.Len(t, parsed.Kafka.Listeners, 1)
	require.Equal(t, "0.0.0.0:9092", parsed.Kafka.Listeners[0].ListenAddress)
	require.Equal(t, "my-kafka.default.svc.cluster.local:9092", parsed.Kafka.Listeners[0].AdvertiseAddress)

	require.NotNil(t, parsed.Data.S3)
	// Bufstream data is isolated under a per-cluster key prefix.
	require.Equal(t, "s3://wandb-bucket/my-kafka", parsed.Data.S3.URI)
	require.Equal(t, "us-east-1", parsed.Data.S3.Region)
	require.Equal(t, "http://objstore.default.svc.cluster.local:8333", parsed.Data.S3.Endpoint)
	require.True(t, parsed.Data.S3.ForcePathStyle)
	// Credentials are referenced indirectly via env vars, never inlined.
	require.NotNil(t, parsed.Data.S3.AccessKeyID)
	require.Equal(t, EnvStorageAccessKeyID, parsed.Data.S3.AccessKeyID.EnvVar)
	require.Equal(t, EnvStorageSecretAccessKey, parsed.Data.S3.SecretAccessKey.EnvVar)
	require.NotContains(t, rendered, "sk")

	require.Equal(t, etcdAddrs, parsed.Metadata.Etcd.Addresses)
	require.True(t, strings.Contains(rendered, "0.0.0.0:9090"))
}

func TestRenderBufstreamConfigS3NoStaticCreds(t *testing.T) {
	storage := objectstore.ConnInfo{
		Provider: apiv2.ObjectStoreProviderS3,
		URI:      "s3://wandb-bucket",
		Bucket:   "wandb-bucket",
		Region:   "us-west-2",
	}
	rendered, err := renderBufstreamConfig("k", "k.ns.svc", []string{"e:2379"}, storage)
	require.NoError(t, err)

	var parsed bufstreamConfig
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &parsed))
	require.NotNil(t, parsed.Data.S3)
	// Ambient credentials: no env_var references are emitted.
	require.Nil(t, parsed.Data.S3.AccessKeyID)
	require.Nil(t, parsed.Data.S3.SecretAccessKey)
}

func TestRenderBufstreamConfigGCS(t *testing.T) {
	storage := objectstore.ConnInfo{
		Provider: apiv2.ObjectStoreProviderGCS,
		Path:     "prefix",
		Bucket:   "wandb-bucket",
	}
	rendered, err := renderBufstreamConfig("k", "k.ns.svc", []string{"e:2379"}, storage)
	require.NoError(t, err)

	var parsed bufstreamConfig
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &parsed))
	require.Nil(t, parsed.Data.S3)
	require.Equal(t, "gs://wandb-bucket/prefix/k", parsed.Data.GCS)
}

func TestRenderBufstreamConfigAzure(t *testing.T) {
	storage := objectstore.ConnInfo{
		Provider:  apiv2.ObjectStoreProviderAzure,
		Bucket:    "container",
		AccessKey: "wandbstorageacct",
		SecretKey: "azsupersecret",
	}
	rendered, err := renderBufstreamConfig("k", "k.ns.svc", []string{"e:2379"}, storage)
	require.NoError(t, err)

	var parsed bufstreamConfig
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &parsed))
	require.Nil(t, parsed.Data.S3)
	require.NotNil(t, parsed.Data.Azure)
	require.Equal(t, "https://wandbstorageacct.blob.core.windows.net/container/k", parsed.Data.Azure.URI,
		"the blob host must derive from the connection's storage account")
	require.NotNil(t, parsed.Data.Azure.AccessKeyID)
	require.Equal(t, EnvStorageAccessKeyID, parsed.Data.Azure.AccessKeyID.EnvVar)
	require.NotContains(t, rendered, "azsupersecret")
}
