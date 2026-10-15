package reconciler

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
)

func TestFeatureBindings(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		w := &apiv2.WeightsAndBiases{}
		w.Spec.Wandb.EnableGlobalAdminAPIKey = enabled
		w.Spec.Wandb.Features = map[string]bool{"bound": !enabled, "legacy": true}
		m := serverManifest.Manifest{
			Features: map[string]bool{"legacy": false},
			FeatureBindings: map[string]serverManifest.FeatureBinding{
				"bound":  {Field: "spec.wandb.enableGlobalAdminAPIKey"},
				"absent": {Field: "spec.wandb.serviceAccount.create"},
			},
		}
		resolved, err := resolveManifestFeatures(w, m)
		require.NoError(t, err)
		require.Equal(t, enabled, resolved.Features["bound"])
		require.True(t, resolved.Features["legacy"])
		require.False(t, resolved.Features["absent"])
		require.False(t, m.Features["legacy"], "resolution must not mutate the source manifest")
		require.True(t, resolved.FeaturesEnabled([]string{"absent", "legacy"}), "preserve OR semantics")
	}
	for _, field := range []string{"", "status.ready", "spec..wandb", "spec.wandb.hostname"} {
		_, err := resolveManifestFeatures(&apiv2.WeightsAndBiases{}, serverManifest.Manifest{
			FeatureBindings: map[string]serverManifest.FeatureBinding{"bad": {Field: field}},
		})
		require.Error(t, err, field)
	}
	_, err := resolveManifestFeatures(&apiv2.WeightsAndBiases{}, serverManifest.Manifest{
		Features:        map[string]bool{"duplicate": false},
		FeatureBindings: map[string]serverManifest.FeatureBinding{"duplicate": {Field: "spec.wandb.bucketProxy"}},
	})
	require.ErrorContains(t, err, "both")
}

func TestGlobalAdminManifestWiring(t *testing.T) {
	dir, err := filepath.Abs("../../../hack/testing-manifests/server-manifest")
	require.NoError(t, err)
	m, err := serverManifest.LoadManifestFromFile(context.Background(), "file://"+dir, "0.85.0-dpanzella-server-manifest-test.1")
	require.NoError(t, err)
	require.Equal(t, "spec.wandb.enableGlobalAdminAPIKey", m.FeatureBindings["globalAdminAPIKey"].Field)
	require.Equal(t, "hex", m.GeneratedSecrets[0].CharacterType)
	require.Equal(t, 40, m.GeneratedSecrets[0].Length)
	for _, enabled := range []bool{false, true} {
		w := &apiv2.WeightsAndBiases{}
		w.Spec.Wandb.EnableGlobalAdminAPIKey = enabled
		w.Status.GeneratedSecrets = map[string]corev1.SecretKeySelector{
			"global-admin-api-key": {LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-global-admin-api-key"}, Key: "key"},
		}
		resolved, err := resolveManifestFeatures(w, m)
		require.NoError(t, err)
		for name, app := range m.Applications {
			envs, err := resolveEnvvars(context.Background(), customCATestClient(t).Build(), w, resolved, app.CommonEnvs, app.Env)
			require.NoError(t, err)
			require.Equal(t, enabled && name == "api", containsAdminEnv(envs), name)
		}
		for name, task := range m.Migrations {
			envs, err := resolveEnvvars(context.Background(), customCATestClient(t).Build(), w, resolved, task.CommonEnvs, task.Env)
			require.NoError(t, err)
			require.Equal(t, enabled && name == "gorilla", containsAdminEnv(envs), name)
		}
	}
}

func containsAdminEnv(envs []corev1.EnvVar) bool {
	for _, env := range envs {
		if env.Name == "GLOBAL_ADMIN_API_KEY" {
			return true
		}
	}
	return false
}

func TestGlobalAdminRejectsUnsupportedManifestAndOverrides(t *testing.T) {
	w := &apiv2.WeightsAndBiases{}
	w.Spec.Wandb.EnableGlobalAdminAPIKey = true
	require.ErrorContains(t, validateGlobalAdminConfiguration(w, serverManifest.Manifest{}), "requires")
	m := serverManifest.Manifest{FeatureBindings: map[string]serverManifest.FeatureBinding{"arbitraryName": {Field: "spec.wandb.enableGlobalAdminAPIKey"}}}
	require.NoError(t, validateGlobalAdminConfiguration(w, m))
	w.Spec.Wandb.EnableGlobalAdminAPIKey = false
	w.Spec.Wandb.LegacyOverrides = map[string]apiv2.LegacyOverrides{
		"global": {Env: []corev1.EnvVar{{Name: "GLOBAL_ADMIN_API_KEY", Value: "override"}}},
	}
	require.ErrorContains(t, validateGlobalAdminConfiguration(w, m), "conflicts")
}
