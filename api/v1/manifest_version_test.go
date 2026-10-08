package v1

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	appsv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
)

func TestConvertToManifestContractVersions(t *testing.T) {
	for _, tc := range []struct{ name, metadata string }{
		{"legacy", ""},
		{"explicit", "manifestVersion: 1\n"},
		{"unsupported", "manifestVersion: 2\n"},
		{"invalid", "manifestVersion: null\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, testLegacyVersion+".yaml"), []byte(tc.metadata+"applications:\n  api:\n    name: api\n"), 0600))
			SetConversionManifestGetter(func(ctx context.Context, _, version string, _ *manifest.RegistryAuth) (manifest.Manifest, error) {
				return manifest.LoadManifestFromFile(ctx, "file://"+dir, version)
			})
			t.Cleanup(defaultConversionManifest)
			src := newV1(withVersion(map[string]interface{}{
				"api": map[string]interface{}{"env": map[string]interface{}{"CUSTOM": "preserved"}},
			}))
			dst := &appsv2.WeightsAndBiases{}
			err := src.ConvertTo(dst)
			switch tc.name {
			case "unsupported":
				var unsupported *manifest.UnsupportedManifestVersionError
				require.ErrorAs(t, err, &unsupported)
				require.Equal(t, 2, unsupported.Version)
			case "invalid":
				var invalid *manifest.InvalidManifestVersionError
				require.ErrorAs(t, err, &invalid)
			default:
				require.NoError(t, err)
				require.Len(t, dst.Spec.Wandb.LegacyOverrides["api"].Env, 1)
				require.Equal(t, "CUSTOM", dst.Spec.Wandb.LegacyOverrides["api"].Env[0].Name)
				require.Equal(t, "preserved", dst.Spec.Wandb.LegacyOverrides["api"].Env[0].Value)
				// Reading the stored v2 object's legacy source does not need a
				// manifest, even if its selected artifact later becomes unavailable.
				withFailingConversionManifest(t, &manifest.UnsupportedManifestVersionError{Version: 2, Supported: []int{1}})
				roundTrip := &WeightsAndBiases{}
				require.NoError(t, roundTrip.ConvertFrom(dst))
				require.Equal(t, src.Spec.Values, roundTrip.Spec.Values)
			}
		})
	}
}
