package reconciler

import (
	"context"
	"maps"
	"slices"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

// validateApplicationOverrides logs spec.wandb.applications keys that match no
// manifest application. Such keys are never applied; the spec is left untouched.
func validateApplicationOverrides(ctx context.Context, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) {
	unknown := unknownApplicationOverrideKeys(wandb, manifest)
	if len(unknown) == 0 {
		return
	}
	logx.GetSlog(ctx).Warn("spec.wandb.applications keys do not match any application in the server manifest; ignoring",
		"keys", unknown, "version", wandb.Spec.Wandb.Version)
}

func unknownApplicationOverrideKeys(wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) []string {
	if len(wandb.Spec.Wandb.Applications) == 0 {
		return nil
	}
	known := make(map[string]struct{}, len(manifest.Applications))
	for _, app := range sortedManifestApplications(manifest) {
		known[app.Name] = struct{}{}
	}
	var unknown []string
	for _, key := range slices.Sorted(maps.Keys(wandb.Spec.Wandb.Applications)) {
		if _, ok := known[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	return unknown
}
