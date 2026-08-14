package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	appsv2 "github.com/wandb/operator/api/v2"
)

// externalClickHouseValues is a minimal v1 external ClickHouse block, so
// conversion has a connection to attach the replication settings to.
func externalClickHouseValues(extra map[string]interface{}) map[string]interface{} {
	global := map[string]interface{}{
		"clickhouse": map[string]interface{}{
			"install":  false,
			"host":     "clickhouse-wandb.clickhouse.svc.cluster.local",
			"port":     int64(8123),
			"database": "weave",
			"user":     "weave",
		},
	}
	values := map[string]interface{}{"global": global}
	for k, v := range extra {
		if k != "global" {
			values[k] = v
			continue
		}
		// One level deeper, so adding global.clickhouse.replicated doesn't wipe out
		// the connection fields the harvest needs something to attach to.
		for gk, gv := range v.(map[string]interface{}) {
			existing, isMap := global[gk].(map[string]interface{})
			incoming, alsoMap := gv.(map[string]interface{})
			if isMap && alsoMap {
				for ik, iv := range incoming {
					existing[ik] = iv
				}
				continue
			}
			global[gk] = gv
		}
	}
	return values
}

func convertedClickHouse(t *testing.T, values map[string]interface{}) *appsv2.ClickHouseConnection {
	t.Helper()
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, newV1(values).ConvertTo(dst))
	return dst.Spec.ClickHouse[appsv2.DefaultInstanceName].ExternalClickHouse
}

// convertedClickHousePending decodes the clickhouse-pending annotation. Conversion
// leaves replication there, with the other literals it can't express as
// selectors, for the reconciler to drain into the connection Secret.
func convertedClickHousePending(t *testing.T, values map[string]interface{}) map[string]interface{} {
	t.Helper()
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, newV1(values).ConvertTo(dst))

	raw, found := dst.Annotations[ClickHousePendingAnnotation]
	if !found {
		return map[string]interface{}{}
	}
	var pending map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &pending))
	return pending
}

func legacyOverrideEnvNames(t *testing.T, values map[string]interface{}) map[string][]string {
	t.Helper()
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, newV1(values).ConvertTo(dst))

	out := map[string][]string{}
	for key, override := range dst.Spec.Wandb.LegacyOverrides {
		for _, env := range override.Env {
			out[key] = append(out[key], env.Name)
		}
	}
	return out
}

// The real cutover shape: both env vars under weave-trace.extraEnv.
func TestConvertTo_ClickHouseReplicationFromAppExtraEnv(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"extraEnv": map[string]interface{}{
				"DISABLE_TELEMETRY":                "true",
				"WF_CLICKHOUSE_REPLICATED":         "true",
				"WF_CLICKHOUSE_REPLICATED_CLUSTER": "weavecluster",
			},
		},
	}))

	require.Equal(t, "true", pending["replicated"])
	require.Equal(t, "weavecluster", pending["replicatedCluster"])
}

func TestConvertTo_ClickHouseReplicationFromAppEnv(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"env": map[string]interface{}{
				"WF_CLICKHOUSE_REPLICATED":         "true",
				"WF_CLICKHOUSE_REPLICATED_CLUSTER": "from-env",
			},
		},
	}))

	require.Equal(t, "true", pending["replicated"])
	require.Equal(t, "from-env", pending["replicatedCluster"])
}

func TestConvertTo_ClickHouseReplicationFromGlobalEnv(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"env": map[string]interface{}{
				"WF_CLICKHOUSE_REPLICATED":         "true",
				"WF_CLICKHOUSE_REPLICATED_CLUSTER": "global-cluster",
			},
		},
	}))

	require.Equal(t, "true", pending["replicated"])
	require.Equal(t, "global-cluster", pending["replicatedCluster"])
}

func TestConvertTo_ClickHouseReplicationFromGlobalExtraEnv(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"extraEnv": map[string]interface{}{
				"WF_CLICKHOUSE_REPLICATED": "true",
			},
		},
	}))

	require.Equal(t, "true", pending["replicated"])
}

// The structured v1 flag, with no env var anywhere.
func TestConvertTo_ClickHouseReplicationFromGlobalClickhouseFlag(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"clickhouse": map[string]interface{}{"replicated": true},
		},
	}))

	require.Equal(t, "true", pending["replicated"])
	require.NotContains(t, pending, "replicatedCluster", "the v1 flag carries no cluster name")
}

// Per-application env is more specific than global, mirroring v1 at runtime.
func TestConvertTo_ClickHouseReplicationAppWinsOverGlobal(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"env": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED_CLUSTER": "global-cluster"},
		},
		"weave-trace": map[string]interface{}{
			"env": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED_CLUSTER": "app-cluster"},
		},
	}))

	require.Equal(t, "app-cluster", pending["replicatedCluster"])
}

// An explicit env var is more specific than the structured flag.
func TestConvertTo_ClickHouseReplicationEnvWinsOverFlag(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"clickhouse": map[string]interface{}{"replicated": true},
			"env":        map[string]interface{}{"WF_CLICKHOUSE_REPLICATED": "false"},
		},
	}))

	require.Equal(t, "false", pending["replicated"])
}

// env beats extraEnv within one section, as the chart did.
func TestConvertTo_ClickHouseReplicationEnvBeatsExtraEnvInSection(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"extraEnv": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED_CLUSTER": "from-extra"},
			"env":      map[string]interface{}{"WF_CLICKHOUSE_REPLICATED_CLUSTER": "from-env"},
		},
	}))

	require.Equal(t, "from-env", pending["replicatedCluster"])
}

// Applications that disagree describe one datastore two ways; v2 can't hold both.
func TestConvertTo_ClickHouseReplicationConflictingAppsFails(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"env": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED_CLUSTER": "cluster-a"},
		},
		"weave-trace-worker": map[string]interface{}{
			"env": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED_CLUSTER": "cluster-b"},
		},
	}))

	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cluster-a")
	require.Contains(t, err.Error(), "cluster-b")
	require.Contains(t, err.Error(), "every application must agree")
}

// Identical values across applications are not a conflict.
func TestConvertTo_ClickHouseReplicationAgreeingAppsOK(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"env": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED_CLUSTER": "shared"},
		},
		"weave-trace-worker": map[string]interface{}{
			"env": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED_CLUSTER": "shared"},
		},
	}))

	require.Equal(t, "shared", pending["replicatedCluster"])
}

// The whole point: they must not also survive as per-application overrides, where
// a stale value would outrank the datastore's own topology.
func TestConvertTo_ClickHouseReplicationExcludedFromLegacyOverrides(t *testing.T) {
	names := legacyOverrideEnvNames(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"extraEnv": map[string]interface{}{
				"ENABLE_REGISTRY_UI":       "true",
				"WF_CLICKHOUSE_REPLICATED": "true",
			},
		},
		"weave-trace": map[string]interface{}{
			"extraEnv": map[string]interface{}{
				"DISABLE_TELEMETRY":                "true",
				"WF_CLICKHOUSE_REPLICATED":         "true",
				"WF_CLICKHOUSE_REPLICATED_CLUSTER": "weavecluster",
			},
		},
	}))

	for key, envNames := range names {
		require.NotContains(t, envNames, "WF_CLICKHOUSE_REPLICATED",
			"replication env leaked into legacyOverrides[%q]", key)
		require.NotContains(t, envNames, "WF_CLICKHOUSE_REPLICATED_CLUSTER",
			"replication env leaked into legacyOverrides[%q]", key)
	}
	// Unrelated env in the same blocks must still convert.
	require.Contains(t, names[appsv2.LegacyOverridesGlobalKey], "ENABLE_REGISTRY_UI")
}

// Managed ClickHouse derives its own topology, so harvested values are dropped
// rather than written to a connection that doesn't exist.
func TestConvertTo_ClickHouseReplicationDroppedWhenManaged(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"clickhouse": map[string]interface{}{"install": true},
		},
		"weave-trace": map[string]interface{}{
			"extraEnv": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED": "true"},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Empty(t, dst.Spec.ClickHouse,
		"install=true must leave ClickHouse to the defaulter, which makes it managed")
}

func TestConvertTo_ClickHouseReplicationNonBooleanFails(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"env": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED": "yes-please"},
		},
	}))

	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not a boolean")
}

// Helm templates never resolve in the operator, so they must not be harvested.
func TestConvertTo_ClickHouseReplicationTemplatedValueIgnored(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"env": map[string]interface{}{
				"WF_CLICKHOUSE_REPLICATED_CLUSTER": "{{ .Release.Name }}-cluster",
			},
		},
	}))

	require.NotContains(t, pending, "replicatedCluster")
}

// Absent everywhere: nothing declared, so the manifest's defaultValue applies.
func TestConvertTo_ClickHouseReplicationAbsent(t *testing.T) {
	conn := convertedClickHouse(t, externalClickHouseValues(nil))
	require.NotNil(t, conn)

	pending := convertedClickHousePending(t, externalClickHouseValues(nil))
	require.NotContains(t, pending, "replicated")
	require.NotContains(t, pending, "replicatedCluster")
}

// Replication is written into an annotation mapClickHouse has already populated,
// so it has to merge: clobbering it would drop the connection literals and leave
// the converted Secret with nothing but a topology.
func TestConvertTo_ClickHouseReplicationMergesIntoPendingLiterals(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"env": map[string]interface{}{
				"WF_CLICKHOUSE_REPLICATED":         "true",
				"WF_CLICKHOUSE_REPLICATED_CLUSTER": "weavecluster",
			},
		},
	}))

	require.Equal(t, "clickhouse-wandb.clickhouse.svc.cluster.local", pending["host"])
	require.Equal(t, "weave", pending["database"])
	require.Equal(t, "weave", pending["user"])
	require.Equal(t, "8123", pending["port"], "a numeric v1 port must survive the merge as written")
	require.Equal(t, "true", pending["replicated"])
	require.Equal(t, "weavecluster", pending["replicatedCluster"])
}

// The connection must still round-trip through the raw v1 annotation untouched.
func TestConvertTo_ClickHouseReplicationPreservesV1Annotation(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(externalClickHouseValues(map[string]interface{}{
		"weave-trace": map[string]interface{}{
			"extraEnv": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED": "true"},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(dst.Annotations[v1ValuesAnnotation]), &decoded))
	weave := decoded["weave-trace"].(map[string]interface{})
	extra := weave["extraEnv"].(map[string]interface{})
	require.Equal(t, "true", extra["WF_CLICKHOUSE_REPLICATED"],
		"the v1-values annotation must keep the original env for round-tripping")
}
