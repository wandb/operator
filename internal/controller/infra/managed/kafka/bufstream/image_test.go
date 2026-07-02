package bufstream

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

func TestBufstreamImage(t *testing.T) {
	require.Equal(t, defaultBufstreamImage, BufstreamImage(manifest.ImageRef{}, ""))
	require.Equal(t,
		"us-docker.pkg.dev/buf-images-1/buf/images/bufstream:0.5.0",
		BufstreamImage(manifest.ImageRef{
			Registry:   "us-docker.pkg.dev",
			Repository: "buf-images-1/buf/images/bufstream",
			Tag:        "0.5.0",
		}, ""),
	)
	require.Equal(t,
		"myregistry.io/us-docker.pkg.dev/buf-images-1/buf/images/bufstream:0.5.0",
		BufstreamImage(manifest.ImageRef{
			Registry:   "us-docker.pkg.dev",
			Repository: "buf-images-1/buf/images/bufstream",
			Tag:        "0.5.0",
		}, "myregistry.io"),
	)
}

func TestEtcdImage(t *testing.T) {
	require.Equal(t, defaultEtcdImage, EtcdImage(manifest.ImageRef{}, ""))
	require.Equal(t,
		"quay.io/coreos/etcd:v3.5.31",
		EtcdImage(manifest.ImageRef{Registry: "quay.io", Repository: "coreos/etcd", Tag: "v3.5.31"}, ""),
	)
	require.Equal(t,
		"myregistry.io/quay.io/coreos/etcd:v3.5.31",
		EtcdImage(manifest.ImageRef{Registry: "quay.io", Repository: "coreos/etcd", Tag: "v3.5.31"}, "myregistry.io"),
	)
}

func TestBucketEnsureImage(t *testing.T) {
	require.Equal(t, defaultBucketEnsureImage, BucketEnsureImage(manifest.ImageRef{}, ""))
	require.Equal(t,
		"amazon/aws-cli:2.35.10",
		BucketEnsureImage(manifest.ImageRef{Repository: "amazon/aws-cli", Tag: "2.35.10"}, ""),
	)
	require.Equal(t,
		"myregistry.io/amazon/aws-cli:2.35.10",
		BucketEnsureImage(manifest.ImageRef{Repository: "amazon/aws-cli", Tag: "2.35.10"}, "myregistry.io"),
	)
}
