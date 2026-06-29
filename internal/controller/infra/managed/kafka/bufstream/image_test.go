package bufstream

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

// The global image registry is stubbed to "" inside the image resolvers until
// the CR field exists, so only the no-override cases are reachable here; the
// registry override behavior is covered by the ImageRef.GetImage table test.
func TestBufstreamImage(t *testing.T) {
	require.Equal(t, defaultBufstreamImage, BufstreamImage(manifest.ImageRef{}))
	require.Equal(t,
		"us-docker.pkg.dev/buf-images-1/buf/images/bufstream:0.5.0",
		BufstreamImage(manifest.ImageRef{
			Registry:   "us-docker.pkg.dev",
			Repository: "buf-images-1/buf/images/bufstream",
			Tag:        "0.5.0",
		}),
	)
}

func TestEtcdImage(t *testing.T) {
	require.Equal(t, defaultEtcdImage, EtcdImage(manifest.ImageRef{}))
	require.Equal(t,
		"quay.io/coreos/etcd:v3.5.31",
		EtcdImage(manifest.ImageRef{Registry: "quay.io", Repository: "coreos/etcd", Tag: "v3.5.31"}),
	)
}

func TestBucketEnsureImage(t *testing.T) {
	require.Equal(t, defaultBucketEnsureImage, BucketEnsureImage(manifest.ImageRef{}))
	require.Equal(t,
		"amazon/aws-cli:2.35.10",
		BucketEnsureImage(manifest.ImageRef{Repository: "amazon/aws-cli", Tag: "2.35.10"}),
	)
}
