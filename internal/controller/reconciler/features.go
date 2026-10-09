package reconciler

import (
	"fmt"
	"maps"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

func resolveManifestFeatures(wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (serverManifest.Manifest, error) {
	features := maps.Clone(manifest.Features)
	if features == nil {
		features = map[string]bool{}
	}
	for name, enabled := range wandb.Spec.Wandb.Features {
		if _, bound := manifest.FeatureBindings[name]; !bound {
			features[name] = enabled
		}
	}
	for name, binding := range manifest.FeatureBindings {
		if _, duplicate := manifest.Features[name]; duplicate {
			return manifest, fmt.Errorf("feature %q is declared in both features and featureBindings", name)
		}
		if name == "" || !strings.HasPrefix(binding.Field, "spec.") || strings.HasSuffix(binding.Field, ".") || strings.Contains(binding.Field, "..") {
			return manifest, fmt.Errorf("feature binding %q requires a dotted spec field", name)
		}
		value, found := resolveCRField(wandb, binding.Field)
		enabled := false
		if found && value != nil {
			var ok bool
			enabled, ok = value.(bool)
			if !ok {
				return manifest, fmt.Errorf("feature binding %q field %q must be boolean", name, binding.Field)
			}
		}
		features[name] = enabled
	}
	manifest.Features = features
	return manifest, nil
}

func validateGlobalAdminConfiguration(wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) error {
	supported := false
	for _, binding := range manifest.FeatureBindings {
		if binding.Field == "spec.wandb.enableGlobalAdminAPIKey" {
			supported = true
		}
	}
	if wandb.Spec.Wandb.EnableGlobalAdminAPIKey && !supported {
		return fmt.Errorf("enableGlobalAdminAPIKey requires a server manifest with a binding to spec.wandb.enableGlobalAdminAPIKey")
	}
	if supported {
		// Reject overrides even while disabled: otherwise they would bypass revocation.
		for name, override := range wandb.Spec.Wandb.LegacyOverrides {
			for _, env := range override.Env {
				if env.Name == "GLOBAL_ADMIN_API_KEY" {
					return fmt.Errorf("legacyOverrides.%s.env conflicts with manifest-managed GLOBAL_ADMIN_API_KEY; remove the override and use enableGlobalAdminAPIKey", name)
				}
			}
		}
	}
	return nil
}
