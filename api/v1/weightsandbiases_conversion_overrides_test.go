/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

const testLegacyVersion = "0.83.0-test"

// disableConversionManifestFetch installs a resolver that always errors, so no
// unit test ever fetches a real manifest over the network. Tests opt in to
// per-app extraction via withConversionManifestApps.
func disableConversionManifestFetch() {
	SetConversionManifestGetter(func(_ context.Context, _, _ string) (serverManifest.Manifest, error) {
		return serverManifest.Manifest{}, errors.New("manifest fetch disabled in unit tests")
	})
}

func TestMain(m *testing.M) {
	disableConversionManifestFetch()
	os.Exit(m.Run())
}

// withConversionManifest installs a fake manifest resolver returning the given
// applications (cleaned up via t.Cleanup, which also clears the failure
// cooldowns). Returns a counter of resolver invocations.
func withConversionManifest(t *testing.T, apps map[string]serverManifest.Application) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	SetConversionManifestGetter(func(_ context.Context, _, _ string) (serverManifest.Manifest, error) {
		calls.Add(1)
		return serverManifest.Manifest{Applications: apps}, nil
	})
	t.Cleanup(disableConversionManifestFetch)
	return &calls
}

// withConversionManifestApps is withConversionManifest for plain names with no
// legacyKey.
func withConversionManifestApps(t *testing.T, names ...string) *atomic.Int32 {
	t.Helper()
	apps := make(map[string]serverManifest.Application, len(names))
	for _, name := range names {
		apps[name] = serverManifest.Application{Name: name}
	}
	return withConversionManifest(t, apps)
}

// withVersion adds the app.image.tag mapVersion reads, so per-app extraction
// has a version to resolve the manifest with.
func withVersion(values map[string]interface{}) map[string]interface{} {
	values["app"] = map[string]interface{}{
		"image": map[string]interface{}{"tag": testLegacyVersion},
	}
	return values
}

func TestConvertTo_LegacyOverridesAbsent(t *testing.T) {
	withConversionManifestApps(t, "api")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"global": map[string]interface{}{"host": "https://wandb.example.com"},
	}))
	require.NoError(t, src.ConvertTo(dst))
	require.Nil(t, dst.Spec.Wandb.LegacyOverrides)
}

func TestConvertTo_LegacyOverridesGlobalEnvPrecedence(t *testing.T) {
	// No version in values: global env must convert without any manifest fetch.
	SetConversionManifestGetter(func(_ context.Context, _, _ string) (serverManifest.Manifest, error) {
		t.Fatal("manifest must not be resolved when no version is derived")
		return serverManifest.Manifest{}, nil
	})
	t.Cleanup(disableConversionManifestFetch)

	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"env": map[string]interface{}{
				"BOTH":     "from-env",
				"ENV_ONLY": "env-value",
			},
			"extraEnv": map[string]interface{}{
				"BOTH":       "from-extra-env",
				"EXTRA_ONLY": "extra-value",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Contains(t, dst.Spec.Wandb.LegacyOverrides, appsv2.LegacyOverridesGlobalKey)
	global := dst.Spec.Wandb.LegacyOverrides[appsv2.LegacyOverridesGlobalKey]
	require.Equal(t, []corev1.EnvVar{
		{Name: "BOTH", Value: "from-env"},
		{Name: "ENV_ONLY", Value: "env-value"},
		{Name: "EXTRA_ONLY", Value: "extra-value"},
	}, global.Env)
	require.Nil(t, global.Resources)
}

func TestConvertTo_LegacyOverridesScalarCoercion(t *testing.T) {
	withConversionManifestApps(t, "parquet")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"parquet": map[string]interface{}{
			"env": map[string]interface{}{
				"BOOL":  true,
				"INT":   int64(8083),
				"FLOAT": 1.5,
				"STR":   "plain",
			},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, []corev1.EnvVar{
		{Name: "BOOL", Value: "true"},
		{Name: "FLOAT", Value: "1.5"},
		{Name: "INT", Value: "8083"},
		{Name: "STR", Value: "plain"},
	}, dst.Spec.Wandb.LegacyOverrides["parquet"].Env)
}

func TestConvertTo_LegacyOverridesValueFromBody(t *testing.T) {
	withConversionManifestApps(t, "api")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"api": map[string]interface{}{
			"env": map[string]interface{}{
				"API_KEY": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "observability",
							"key":  "api-key",
						},
					},
				},
			},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	env := dst.Spec.Wandb.LegacyOverrides["api"].Env
	require.Len(t, env, 1)
	require.Equal(t, "API_KEY", env[0].Name)
	require.NotNil(t, env[0].ValueFrom)
	require.NotNil(t, env[0].ValueFrom.SecretKeyRef)
	require.Equal(t, "observability", env[0].ValueFrom.SecretKeyRef.Name)
	require.Equal(t, "api-key", env[0].ValueFrom.SecretKeyRef.Key)
}

func TestConvertTo_LegacyOverridesMalformedBodyFails(t *testing.T) {
	withConversionManifestApps(t, "api")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"api": map[string]interface{}{
			"env": map[string]interface{}{
				"BROKEN": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyReff": map[string]interface{}{"name": "x", "key": "y"},
					},
				},
			},
		},
	}))
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "spec.values.api env BROKEN")
}

func TestConvertTo_LegacyOverridesTemplateValuesDropped(t *testing.T) {
	withConversionManifestApps(t, "executor")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"global": map[string]interface{}{
			"extraEnv": map[string]interface{}{
				"TEMPLATED": "{{ .Release.Name }}-suffix",
				"KEPT":      "plain",
				"INTERP":    "$(OTHER_VAR)/path",
			},
		},
		"executor": map[string]interface{}{
			"env": map[string]interface{}{
				"ONLY_TEMPLATED": `{{ include "wandb.executor.taskQueue" . }}`,
			},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	global := dst.Spec.Wandb.LegacyOverrides[appsv2.LegacyOverridesGlobalKey]
	require.Equal(t, []corev1.EnvVar{
		{Name: "INTERP", Value: "$(OTHER_VAR)/path"},
		{Name: "KEPT", Value: "plain"},
	}, global.Env)

	// executor's only entry was templated, so the whole section is absent.
	require.NotContains(t, dst.Spec.Wandb.LegacyOverrides, "executor")
}

func TestConvertTo_LegacyOverridesManifestLegacyKey(t *testing.T) {
	// Renamed apps declare their v1 values key via the manifest's legacyKey
	// field; the operator carries no rename table.
	withConversionManifest(t, map[string]serverManifest.Application{
		"nginx-proxy": {Name: "nginx-proxy", LegacyKey: "nginx"},
		"weave-trace-evaluate-model-worker": {
			Name:      "weave-trace-evaluate-model-worker",
			LegacyKey: "weave-evaluate-model-worker",
		},
		"parquet": {Name: "parquet"},
	})
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"nginx": map[string]interface{}{
			"env": map[string]interface{}{"NGINX_VAR": "1"},
		},
		"weave-evaluate-model-worker": map[string]interface{}{
			"env": map[string]interface{}{"WORKER_VAR": "2"},
		},
		"parquet": map[string]interface{}{
			"env": map[string]interface{}{"PARQUET_VAR": "3"},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	overrides := dst.Spec.Wandb.LegacyOverrides
	require.Contains(t, overrides, "nginx-proxy")
	require.NotContains(t, overrides, "nginx")
	require.Contains(t, overrides, "weave-trace-evaluate-model-worker")
	require.NotContains(t, overrides, "weave-evaluate-model-worker")
	require.Contains(t, overrides, "parquet")
}

func TestConvertTo_LegacyOverridesWithoutLegacyKeyRenamedSectionSkipped(t *testing.T) {
	// A manifest that predates the legacyKey field: the nginx section has no
	// reader, so it is logged as unmapped and skipped rather than guessed.
	withConversionManifestApps(t, "nginx-proxy")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"nginx": map[string]interface{}{
			"env": map[string]interface{}{"NGINX_VAR": "1"},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Spec.Wandb.LegacyOverrides, "nginx-proxy")
	require.NotContains(t, dst.Spec.Wandb.LegacyOverrides, "nginx")
}

func TestConvertTo_LegacyOverridesUnmappedSectionsSkipped(t *testing.T) {
	withConversionManifestApps(t, "api")
	dst := &appsv2.WeightsAndBiases{}
	values := withVersion(map[string]interface{}{
		"console": map[string]interface{}{
			"env": map[string]interface{}{"CONSOLE_VAR": "2"},
		},
		"api": map[string]interface{}{
			"env": map[string]interface{}{"API_VAR": "1"},
		},
	})
	// The monolith section carries env alongside the image tag withVersion set.
	values["app"].(map[string]interface{})["env"] = map[string]interface{}{"MONOLITH_VAR": "1"}
	src := newV1(values)
	require.NoError(t, src.ConvertTo(dst))

	// Only manifest applications are converted; app/console are logged and
	// skipped (still recoverable from the v1-values annotation).
	overrides := dst.Spec.Wandb.LegacyOverrides
	require.Equal(t, []corev1.EnvVar{{Name: "API_VAR", Value: "1"}}, overrides["api"].Env)
	require.NotContains(t, overrides, "app")
	require.NotContains(t, overrides, "console")
}

func TestConvertTo_LegacyOverridesResourcesSizingMerge(t *testing.T) {
	withConversionManifestApps(t, "api")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"global": map[string]interface{}{"size": "medium"},
		"api": map[string]interface{}{
			"sizing": map[string]interface{}{
				"default": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "100m", "memory": "128Mi"},
					},
				},
				"medium": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "2"},
						"limits":   map[string]interface{}{"memory": "4Gi"},
					},
				},
				"small": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "should-not-apply"},
					},
				},
			},
			"resources": map[string]interface{}{
				"limits": map[string]interface{}{"cpu": "3"},
			},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	resources := dst.Spec.Wandb.LegacyOverrides["api"].Resources
	require.NotNil(t, resources)
	// medium requests.cpu beat default; default memory request survives;
	// flat resources.limits merged over sizing limits.
	require.Equal(t, resource.MustParse("2"), resources.Requests[corev1.ResourceCPU])
	require.Equal(t, resource.MustParse("128Mi"), resources.Requests[corev1.ResourceMemory])
	require.Equal(t, resource.MustParse("3"), resources.Limits[corev1.ResourceCPU])
	require.Equal(t, resource.MustParse("4Gi"), resources.Limits[corev1.ResourceMemory])
}

func TestConvertTo_LegacyOverridesResourcesPerAppSizeWins(t *testing.T) {
	withConversionManifestApps(t, "parquet")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"global": map[string]interface{}{"size": "small"},
		"parquet": map[string]interface{}{
			"size": "xlarge",
			"sizing": map[string]interface{}{
				"small": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "1"},
					},
				},
				"xlarge": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "8"},
					},
				},
			},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	resources := dst.Spec.Wandb.LegacyOverrides["parquet"].Resources
	require.NotNil(t, resources)
	require.Equal(t, resource.MustParse("8"), resources.Requests[corev1.ResourceCPU])
}

func TestConvertTo_LegacyOverridesResourcesDefaultSizeIsSmall(t *testing.T) {
	withConversionManifestApps(t, "weave")
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"weave": map[string]interface{}{
			"sizing": map[string]interface{}{
				"small": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"memory": "1Gi"},
					},
				},
				"large": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"memory": "16Gi"},
					},
				},
			},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	resources := dst.Spec.Wandb.LegacyOverrides["weave"].Resources
	require.NotNil(t, resources)
	require.Equal(t, resource.MustParse("1Gi"), resources.Requests[corev1.ResourceMemory])
}

func TestConvertTo_LegacyOverridesManifestUnavailable(t *testing.T) {
	SetConversionManifestGetter(func(_ context.Context, _, _ string) (serverManifest.Manifest, error) {
		return serverManifest.Manifest{}, errors.New("registry unreachable")
	})
	t.Cleanup(disableConversionManifestFetch)

	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"global": map[string]interface{}{
			"env": map[string]interface{}{"HTTP_PROXY": "http://proxy"},
		},
		"api": map[string]interface{}{
			"env": map[string]interface{}{"API_VAR": "1"},
		},
	}))
	// A manifest fetch failure must never fail conversion: global env still
	// converts, per-app extraction is skipped.
	require.NoError(t, src.ConvertTo(dst))

	overrides := dst.Spec.Wandb.LegacyOverrides
	require.Contains(t, overrides, appsv2.LegacyOverridesGlobalKey)
	require.NotContains(t, overrides, "api")
}

func TestConvertTo_LegacyOverridesManifestFailureCooldown(t *testing.T) {
	// Successful fetches are cached on disk by the ORAS store, but failures
	// retry the remote every call — the cooldown keeps a burst of conversions
	// from stalling on an unreachable registry for every v1 write.
	var calls atomic.Int32
	SetConversionManifestGetter(func(_ context.Context, _, _ string) (serverManifest.Manifest, error) {
		calls.Add(1)
		return serverManifest.Manifest{}, errors.New("registry unreachable")
	})
	t.Cleanup(disableConversionManifestFetch)

	for i := 0; i < 3; i++ {
		dst := &appsv2.WeightsAndBiases{}
		src := newV1(withVersion(map[string]interface{}{
			"api": map[string]interface{}{
				"env": map[string]interface{}{"API_VAR": "1"},
			},
		}))
		require.NoError(t, src.ConvertTo(dst))
		require.NotContains(t, dst.Spec.Wandb.LegacyOverrides, "api")
	}

	require.Equal(t, int32(1), calls.Load(), "repeat conversions within the cooldown must not retry the fetch")
}

func TestConvertTo_LegacyOverridesPrefersActiveSpecValues(t *testing.T) {
	withConversionManifestApps(t, "api")
	withConversionReader(t, activeSpecSecret(t, "default", "wandb", withVersion(map[string]interface{}{
		"api": map[string]interface{}{
			"env": map[string]interface{}{"FROM_ACTIVE": "yes"},
		},
	})))

	dst := &appsv2.WeightsAndBiases{}
	src := newV1(withVersion(map[string]interface{}{
		"api": map[string]interface{}{
			"env": map[string]interface{}{"FROM_CR": "yes"},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, []corev1.EnvVar{{Name: "FROM_ACTIVE", Value: "yes"}},
		dst.Spec.Wandb.LegacyOverrides["api"].Env)
}

func TestConvertRoundTrip_LegacyOverridesIdempotent(t *testing.T) {
	withConversionManifestApps(t, "api")
	values := withVersion(map[string]interface{}{
		"global": map[string]interface{}{
			"size": "medium",
			"env":  map[string]interface{}{"B": "2", "A": "1"},
			"extraEnv": map[string]interface{}{
				"C": true,
			},
		},
		"api": map[string]interface{}{
			"env": map[string]interface{}{
				"KEY": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{"name": "s", "key": "k"},
					},
				},
			},
			"sizing": map[string]interface{}{
				"medium": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{"cpu": "2"},
					},
				},
			},
		},
	})

	first := &appsv2.WeightsAndBiases{}
	require.NoError(t, newV1(values).ConvertTo(first))

	bounced := &WeightsAndBiases{}
	require.NoError(t, bounced.ConvertFrom(first))

	second := &appsv2.WeightsAndBiases{}
	require.NoError(t, bounced.ConvertTo(second))

	require.Equal(t, first.Spec.Wandb.LegacyOverrides, second.Spec.Wandb.LegacyOverrides)
	require.Contains(t, first.Spec.Wandb.LegacyOverrides, "api")
}
