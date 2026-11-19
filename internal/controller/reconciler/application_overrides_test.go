package reconciler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

func TestUnknownApplicationOverrideKeys(t *testing.T) {
	wandb := testWeightsAndBiases()
	wandb.Spec.Wandb.Applications = map[string]apiv2.WandbApplicationOverride{
		"api":      {},
		"weave":    {},
		"consloe":  {},
		"frontend": {},
	}
	manifest := serverManifest.Manifest{
		Applications: map[string]serverManifest.Application{
			"api":     {Name: "api"},
			"weave":   {},
			"web-key": {Name: "frontend"},
		},
	}

	require.Equal(t, []string{"consloe"}, unknownApplicationOverrideKeys(wandb, manifest))
	require.Len(t, wandb.Spec.Wandb.Applications, 4, "spec must be left untouched")
}

func TestUnknownApplicationOverrideKeysEmptyInputs(t *testing.T) {
	require.Empty(t, unknownApplicationOverrideKeys(testWeightsAndBiases(), serverManifest.Manifest{}))

	wandb := testWeightsAndBiases()
	wandb.Spec.Wandb.Applications = map[string]apiv2.WandbApplicationOverride{"api": {}}
	require.Equal(t, []string{"api"}, unknownApplicationOverrideKeys(wandb, serverManifest.Manifest{}))
	validateApplicationOverrides(context.Background(), wandb, serverManifest.Manifest{})
}
