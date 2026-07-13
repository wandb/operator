package reconciler

import (
	"context"
	"sort"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
)

// validateLegacyOverrides logs every spec.wandb.legacyOverrides key that is
// neither the reserved global entry nor an application in the resolved server
// manifest. The spec is deliberately left untouched: unknown keys stay in
// place and are simply never applied, because the apply path only looks up
// "global" and manifest application names.
func validateLegacyOverrides(ctx context.Context, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) {
	if len(wandb.Spec.Wandb.LegacyOverrides) == 0 {
		return
	}
	logger := logx.GetSlog(ctx)

	keys := make([]string, 0, len(wandb.Spec.Wandb.LegacyOverrides))
	for key := range wandb.Spec.Wandb.LegacyOverrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if key == apiv2.LegacyOverridesGlobalKey {
			continue
		}
		if _, ok := manifest.Applications[key]; ok {
			continue
		}
		logger.Warn("legacy override section does not map to any application in the server manifest; ignoring",
			"section", key, "version", wandb.Spec.Wandb.Version)
	}
}

// applyLegacyOverrideEnv layers the v1-derived env overrides onto a fully
// built env list — global first, then per-app so per-app wins. Applying after
// all other env construction means overrides beat manifest and
// operator-injected vars, matching the v1 chart where user env displaced
// chart-computed env.
func applyLegacyOverrideEnv(ctx context.Context, wandb *apiv2.WeightsAndBiases, appName string, envVars []corev1.EnvVar) []corev1.EnvVar {
	overrides := wandb.Spec.Wandb.LegacyOverrides
	if len(overrides) == 0 {
		return envVars
	}
	envVars = overrideEnvVars(ctx, envVars, overrides[apiv2.LegacyOverridesGlobalKey].Env)
	envVars = overrideEnvVars(ctx, envVars, overrides[appName].Env)
	return envVars
}

// overrideEnvVars applies each override by name: it replaces a same-named var
// in place and appends otherwise, so overrides win regardless of how the base
// list was built (the inverse of appendMissingEnvVars). Hand-authored input is
// handled defensively: empty names are skipped with a log and a later
// duplicate within overrides wins.
func overrideEnvVars(ctx context.Context, base []corev1.EnvVar, overrides []corev1.EnvVar) []corev1.EnvVar {
	if len(overrides) == 0 {
		return base
	}
	index := make(map[string]int, len(base))
	for i, envVar := range base {
		index[envVar.Name] = i
	}
	for _, envVar := range overrides {
		if envVar.Name == "" {
			logx.GetSlog(ctx).Warn("skipping legacy override env var with empty name")
			continue
		}
		if i, ok := index[envVar.Name]; ok {
			base[i] = envVar
			continue
		}
		index[envVar.Name] = len(base)
		base = append(base, envVar)
	}
	return base
}
