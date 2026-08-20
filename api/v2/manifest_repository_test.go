package v2_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	appsv2 "github.com/wandb/operator/api/v2"
)

func TestManifestRepositoryFor(t *testing.T) {
	// Empty image registry yields the public default.
	require.Equal(t, appsv2.DefaultManifestRepository, appsv2.ManifestRepositoryFor(""))

	// A private registry places the manifest next to the images, as wsm mirrors it.
	require.Equal(t,
		"oci://myreg.example.com/wandb/server-manifest",
		appsv2.ManifestRepositoryFor("myreg.example.com"))

	// The default must equal the derivation from the default registry so the two
	// constants can never drift.
	require.Equal(t,
		appsv2.DefaultManifestRepository,
		appsv2.ManifestRepositoryFor(appsv2.DefaultImageRegistry))
}
