package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
)

func envNamesValues(envVars []corev1.EnvVar) map[string]string {
	out := make(map[string]string, len(envVars))
	for _, envVar := range envVars {
		out[envVar.Name] = envVar.Value
	}
	return out
}

func TestOverrideEnvVarsReplacesInPlaceAndAppends(t *testing.T) {
	base := []corev1.EnvVar{
		{Name: "A", Value: "base-a"},
		{Name: "B", Value: "base-b"},
	}
	result := overrideEnvVars(context.Background(), base, []corev1.EnvVar{
		{Name: "B", Value: "override-b"},
		{Name: "C", Value: "override-c"},
	})

	if len(result) != 3 {
		t.Fatalf("expected 3 env vars, got %d: %v", len(result), result)
	}
	// Replaced in place: order preserved for existing names.
	if result[1].Name != "B" || result[1].Value != "override-b" {
		t.Errorf("expected B replaced in place, got %v", result[1])
	}
	if result[2].Name != "C" || result[2].Value != "override-c" {
		t.Errorf("expected C appended, got %v", result[2])
	}
	if result[0].Value != "base-a" {
		t.Errorf("expected A untouched, got %v", result[0])
	}
}

func TestOverrideEnvVarsReplacesValueFrom(t *testing.T) {
	base := []corev1.EnvVar{
		{Name: "SECRET", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "old"},
				Key:                  "k",
			},
		}},
	}
	result := overrideEnvVars(context.Background(), base, []corev1.EnvVar{
		{Name: "SECRET", Value: "literal-now"},
	})

	if result[0].ValueFrom != nil || result[0].Value != "literal-now" {
		t.Errorf("expected override to fully replace the var, got %+v", result[0])
	}
}

func TestOverrideEnvVarsHandAuthoredEdgeCases(t *testing.T) {
	base := []corev1.EnvVar{{Name: "A", Value: "base-a"}}
	result := overrideEnvVars(context.Background(), base, []corev1.EnvVar{
		{Name: "", Value: "skipped"},
		{Name: "DUP", Value: "first"},
		{Name: "DUP", Value: "last-wins"},
	})

	values := envNamesValues(result)
	if _, ok := values[""]; ok {
		t.Error("empty-name entry should be skipped")
	}
	if values["DUP"] != "last-wins" {
		t.Errorf("expected later duplicate to win, got %q", values["DUP"])
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 env vars, got %d: %v", len(result), result)
	}
}

func TestOverrideEnvVarsNoOverridesNoOp(t *testing.T) {
	base := []corev1.EnvVar{{Name: "A", Value: "base-a"}}
	result := overrideEnvVars(context.Background(), base, nil)
	if len(result) != 1 || result[0].Value != "base-a" {
		t.Errorf("expected base unchanged, got %v", result)
	}
}

func TestApplyLegacyOverrideEnvPrecedence(t *testing.T) {
	wandb := testWeightsAndBiases()
	wandb.Spec.Wandb.LegacyOverrides = map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{
			{Name: "GLOBAL_ONLY", Value: "global"},
			{Name: "BOTH", Value: "from-global"},
			{Name: "MANIFEST_VAR", Value: "global-override"},
		}},
		"api": {Env: []corev1.EnvVar{
			{Name: "BOTH", Value: "from-app"},
			{Name: "APP_ONLY", Value: "app"},
		}},
	}

	base := []corev1.EnvVar{{Name: "MANIFEST_VAR", Value: "manifest"}}

	result := applyLegacyOverrideEnv(context.Background(), wandb, "api", base)
	values := envNamesValues(result)

	if values["MANIFEST_VAR"] != "global-override" {
		t.Errorf("expected override to beat manifest env, got %q", values["MANIFEST_VAR"])
	}
	if values["BOTH"] != "from-app" {
		t.Errorf("expected per-app to beat global, got %q", values["BOTH"])
	}
	if values["GLOBAL_ONLY"] != "global" || values["APP_ONLY"] != "app" {
		t.Errorf("expected both layers present, got %v", values)
	}
}

func TestApplyLegacyOverrideEnvAppWithoutEntryGetsGlobal(t *testing.T) {
	wandb := testWeightsAndBiases()
	wandb.Spec.Wandb.LegacyOverrides = map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{{Name: "HTTP_PROXY", Value: "http://proxy"}}},
		"parquet":                      {Env: []corev1.EnvVar{{Name: "PARQUET_VAR", Value: "x"}}},
	}

	result := applyLegacyOverrideEnv(context.Background(), wandb, "weave", nil)
	values := envNamesValues(result)

	if values["HTTP_PROXY"] != "http://proxy" {
		t.Errorf("expected global env applied, got %v", values)
	}
	if _, ok := values["PARQUET_VAR"]; ok {
		t.Error("another app's overrides must not apply")
	}
}

func TestApplyLegacyOverrideEnvNoOverrides(t *testing.T) {
	base := []corev1.EnvVar{{Name: "A", Value: "a"}}
	result := applyLegacyOverrideEnv(context.Background(), testWeightsAndBiases(), "api", base)
	if len(result) != 1 || result[0].Value != "a" {
		t.Errorf("expected base unchanged, got %v", result)
	}
}

func TestValidateLegacyOverridesDoesNotMutateSpec(t *testing.T) {
	wandb := testWeightsAndBiases()
	wandb.Spec.Wandb.LegacyOverrides = map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{{Name: "A", Value: "1"}}},
		"api":                          {Env: []corev1.EnvVar{{Name: "B", Value: "2"}}},
		"console":                      {Env: []corev1.EnvVar{{Name: "C", Value: "3"}}},
		"app":                          {Env: []corev1.EnvVar{{Name: "D", Value: "4"}}},
	}
	manifest := serverManifest.Manifest{
		Applications: map[string]serverManifest.Application{
			"api": {Name: "api"},
		},
	}

	validateLegacyOverrides(context.Background(), wandb, manifest)

	// Unmapped keys (console, app) are logged but must remain in the spec.
	if len(wandb.Spec.Wandb.LegacyOverrides) != 4 {
		t.Fatalf("expected spec untouched, got %v", wandb.Spec.Wandb.LegacyOverrides)
	}
	for _, key := range []string{apiv2.LegacyOverridesGlobalKey, "api", "console", "app"} {
		if _, ok := wandb.Spec.Wandb.LegacyOverrides[key]; !ok {
			t.Errorf("expected key %q to remain in spec", key)
		}
	}
}

func TestValidateLegacyOverridesEmptyInputs(t *testing.T) {
	// Must not panic with nil overrides or an empty manifest.
	validateLegacyOverrides(context.Background(), testWeightsAndBiases(), serverManifest.Manifest{})

	wandb := testWeightsAndBiases()
	wandb.Spec.Wandb.LegacyOverrides = map[string]apiv2.LegacyOverrides{"api": {}}
	validateLegacyOverrides(context.Background(), wandb, serverManifest.Manifest{})
}
