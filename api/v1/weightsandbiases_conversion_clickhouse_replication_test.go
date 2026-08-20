package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	appsv2 "github.com/wandb/operator/api/v2"
)

// externalClickHouseValues is a minimal v1 external ClickHouse block, so
// conversion has a connection to attach the replication flag to.
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
		// the connection fields the flag needs something to attach to.
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
// leaves the connection literals it can't express as selectors there, plus the
// structured global.clickhouse.replicated flag, for the reconciler to drain into
// the connection Secret.
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

// The structured global.clickhouse.replicated flag is written into the
// clickhouse-pending annotation for the reconciler to materialize. Env-var
// replication (WF_CLICKHOUSE_REPLICATED[_CLUSTER]) is handled separately at
// reconcile from legacyOverrides.
func TestConvertTo_ClickHouseReplicatedFlagToPending(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"clickhouse": map[string]interface{}{"replicated": true},
		},
	}))

	require.Equal(t, "true", pending["replicated"])
	require.NotContains(t, pending, "replicatedCluster", "the structured flag carries no cluster name")
}

// The flag merges with the connection literals mapClickHouse already wrote,
// rather than clobbering them.
func TestConvertTo_ClickHouseReplicatedFlagMergesWithLiterals(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"clickhouse": map[string]interface{}{"replicated": true},
		},
	}))

	require.Equal(t, "clickhouse-wandb.clickhouse.svc.cluster.local", pending["host"])
	require.Equal(t, "weave", pending["database"])
	require.Equal(t, "weave", pending["user"])
	require.Equal(t, "8123", pending["port"], "a numeric v1 port must survive the merge as written")
	require.Equal(t, "true", pending["replicated"])
}

// No flag: the connection converts but publishes no replication key.
func TestConvertTo_ClickHouseReplicatedFlagAbsent(t *testing.T) {
	conn := convertedClickHouse(t, externalClickHouseValues(nil))
	require.NotNil(t, conn)

	pending := convertedClickHousePending(t, externalClickHouseValues(nil))
	require.NotContains(t, pending, "replicated")
}

// A non-boolean flag is treated as absent (nestedBoolLenient) rather than failing
// conversion; the value can't make a v1 object unservable.
func TestConvertTo_ClickHouseReplicatedFlagNonBooleanIgnored(t *testing.T) {
	pending := convertedClickHousePending(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"clickhouse": map[string]interface{}{"replicated": "yes-please"},
		},
	}))

	require.NotContains(t, pending, "replicated")
}

// Managed ClickHouse (install=true) creates no external connection, so there is
// nothing to attach the replication flag to.
func TestConvertTo_ClickHouseInstallTrueLeavesNoExternal(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"clickhouse": map[string]interface{}{"install": true, "replicated": true},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Empty(t, dst.Spec.ClickHouse,
		"install=true must leave ClickHouse to the defaulter, which makes it managed")
}

// The replication env vars now flow through into legacyOverrides verbatim; the
// reconciler maps and removes them. Previously conversion stripped them here.
func TestConvertTo_ClickHouseReplicationEnvPassesThrough(t *testing.T) {
	names := legacyOverrideEnvNames(t, externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"extraEnv": map[string]interface{}{
				"ENABLE_REGISTRY_UI":               "true",
				"WF_CLICKHOUSE_REPLICATED":         "true",
				"WF_CLICKHOUSE_REPLICATED_CLUSTER": "weavecluster",
			},
		},
	}))

	global := names[appsv2.LegacyOverridesGlobalKey]
	require.Contains(t, global, "WF_CLICKHOUSE_REPLICATED")
	require.Contains(t, global, "WF_CLICKHOUSE_REPLICATED_CLUSTER")
	require.Contains(t, global, "ENABLE_REGISTRY_UI")
}

// The connection must still round-trip through the raw v1 annotation untouched,
// so the original env survives even though the reconciler will map it.
func TestConvertTo_ClickHouseReplicationPreservesV1Annotation(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(externalClickHouseValues(map[string]interface{}{
		"global": map[string]interface{}{
			"extraEnv": map[string]interface{}{"WF_CLICKHOUSE_REPLICATED": "true"},
		},
	}))
	require.NoError(t, src.ConvertTo(dst))

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(dst.Annotations[v1ValuesAnnotation]), &decoded))
	global := decoded["global"].(map[string]interface{})
	extra := global["extraEnv"].(map[string]interface{})
	require.Equal(t, "true", extra["WF_CLICKHOUSE_REPLICATED"],
		"the v1-values annotation must keep the original env for round-tripping")
}
