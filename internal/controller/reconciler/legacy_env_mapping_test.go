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

package reconciler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	apiv2 "github.com/wandb/operator/api/v2"
)

// newEnvMapFixture builds a WeightsAndBiases with the given legacyOverrides and
// (optionally) an external ClickHouse connection, plus a fake client seeded with
// it and any extra objects.
func newEnvMapFixture(
	t *testing.T,
	overrides map[string]apiv2.LegacyOverrides,
	chConn *apiv2.ClickHouseConnection,
	seed ...ctrlClient.Object,
) (ctrlClient.Client, *apiv2.WeightsAndBiases) {
	t.Helper()
	return newMigrationFixture(t, nil, func(w *apiv2.WeightsAndBiases) {
		w.Spec.Wandb.LegacyOverrides = overrides
		if chConn != nil {
			w.Spec.ClickHouse = map[string]apiv2.ClickHouseSpec{
				apiv2.DefaultInstanceName: {ExternalClickHouse: chConn},
			}
		}
	}, seed...)
}

func litEnv(name, value string) corev1.EnvVar {
	return corev1.EnvVar{Name: name, Value: value}
}

func externalCH(w *apiv2.WeightsAndBiases) *apiv2.ClickHouseConnection {
	return w.Spec.ClickHouse[apiv2.DefaultInstanceName].ExternalClickHouse
}

func TestMapLegacyEnvToCR_LiteralReplicatedFromGlobal(t *testing.T) {
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}},
	}, &apiv2.ClickHouseConnection{})

	res, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	secret, err := getClickHouseConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("true"), secret.Data[convertedClickHouseReplicatedKey])

	conn := externalCH(wandb)
	require.Equal(t, "wandb-clickhouse-converted", conn.Replicated.SecretKeyRef().Name)
	require.Equal(t, convertedClickHouseReplicatedKey, conn.Replicated.SecretKeyRef().Key)

	// Env stripped from legacyOverrides (map pruned to nil since it emptied).
	require.Nil(t, wandb.Spec.Wandb.LegacyOverrides)
}

func TestMapLegacyEnvToCR_ClusterFromApp(t *testing.T) {
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		"parquet": {Env: []corev1.EnvVar{litEnv(envClickHouseReplicatedCluster, "weavecluster")}},
	}, &apiv2.ClickHouseConnection{})

	_, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getClickHouseConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("weavecluster"), secret.Data[convertedClickHouseClusterKey])

	conn := externalCH(wandb)
	require.Equal(t, "wandb-clickhouse-converted", conn.ClusterName.SecretKeyRef().Name)
	require.Equal(t, convertedClickHouseClusterKey, conn.ClusterName.SecretKeyRef().Key)
	require.Nil(t, wandb.Spec.Wandb.LegacyOverrides)
}

func TestMapLegacyEnvToCR_PerAppBeatsGlobal(t *testing.T) {
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}},
		"parquet":                      {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "false")}},
	}, &apiv2.ClickHouseConnection{})

	_, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getClickHouseConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("false"), secret.Data[convertedClickHouseReplicatedKey])
}

func TestMapLegacyEnvToCR_ConflictingAppsError(t *testing.T) {
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		"parquet": {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}},
		"weave":   {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "false")}},
	}, &apiv2.ClickHouseConnection{})

	_, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.Error(t, err)
	require.Contains(t, err.Error(), "every application must agree")
}

func TestMapLegacyEnvToCR_AgreeingAppsOK(t *testing.T) {
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		"parquet": {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}},
		"weave":   {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}},
	}, &apiv2.ClickHouseConnection{})

	_, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getClickHouseConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("true"), secret.Data[convertedClickHouseReplicatedKey])
}

// A secretKeyRef source points the connection field directly at the user's
// Secret; no converted Secret is written.
func TestMapLegacyEnvToCR_SecretKeyRefRepointsField(t *testing.T) {
	userRef := &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "user-ch"},
		Key:                  "replicated",
	}
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{{
			Name:      envClickHouseReplicated,
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: userRef},
		}}},
	}, &apiv2.ClickHouseConnection{})

	_, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)

	conn := externalCH(wandb)
	require.Equal(t, "user-ch", conn.Replicated.SecretKeyRef().Name)
	require.Equal(t, "replicated", conn.Replicated.SecretKeyRef().Key)

	_, err = getClickHouseConvertedSecret(t, client)
	require.Error(t, err, "no converted Secret should be written for a secretKeyRef source")
	require.Nil(t, wandb.Spec.Wandb.LegacyOverrides)
}

// An unrepresentable valueFrom (configMapKeyRef) cannot fill a Secret-only
// selector, so the env is left in place as a raw pod-env override.
func TestMapLegacyEnvToCR_UnrepresentableValueFromPassthrough(t *testing.T) {
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{{
			Name: envClickHouseReplicated,
			ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "cm"},
				Key:                  "replicated",
			}},
		}}},
	}, &apiv2.ClickHouseConnection{})

	res, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter, "passthrough persists nothing")

	require.Nil(t, externalCH(wandb).Replicated.SecretKeyRef())
	require.Len(t, wandb.Spec.Wandb.LegacyOverrides[apiv2.LegacyOverridesGlobalKey].Env, 1)
}

// Managed (no external connection) drops the value but still removes the env, so
// managed ClickHouse's own topology is authoritative.
func TestMapLegacyEnvToCR_ManagedDropsAndRemoves(t *testing.T) {
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}},
	}, nil)

	res, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	_, err = getClickHouseConvertedSecret(t, client)
	require.Error(t, err)
	require.Nil(t, wandb.Spec.Wandb.LegacyOverrides)
}

// overrideConversionDerived overwrites a value the conversion flag drain wrote
// into the operator-owned converted Secret (env beats the structured flag).
func TestMapLegacyEnvToCR_OverridesConversionDerivedValue(t *testing.T) {
	conn := &apiv2.ClickHouseConnection{
		Replicated: apiv2.ValueFromSecret("wandb-clickhouse-converted", convertedClickHouseReplicatedKey, false),
	}
	seed := &corev1.Secret{}
	seed.Name = "wandb-clickhouse-converted"
	seed.Namespace = "default"
	seed.Data = map[string][]byte{convertedClickHouseReplicatedKey: []byte("true")}

	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "false")}},
	}, conn, seed)

	_, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getClickHouseConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("false"), secret.Data[convertedClickHouseReplicatedKey], "env overrides the flag value")
}

// A user-owned selector (points at the user's own Secret) is left untouched, but
// the env is still removed — the typed field wins.
func TestMapLegacyEnvToCR_UserOwnedSelectorRespected(t *testing.T) {
	conn := &apiv2.ClickHouseConnection{
		Replicated: apiv2.ValueFromSecret("user-ch", "flag", false),
	}
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}},
	}, conn)

	_, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)

	require.Equal(t, "user-ch", externalCH(wandb).Replicated.SecretKeyRef().Name, "user field untouched")
	require.Equal(t, "flag", externalCH(wandb).Replicated.SecretKeyRef().Key)
	require.Nil(t, wandb.Spec.Wandb.LegacyOverrides, "env still removed")
}

// keepCR (cluster) does not overwrite a field the user already set.
func TestMapLegacyEnvToCR_ClusterKeepCRWhenSet(t *testing.T) {
	conn := &apiv2.ClickHouseConnection{
		ClusterName: apiv2.ValueFromSecret("user-ch", "cluster", false),
	}
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{litEnv(envClickHouseReplicatedCluster, "override")}},
	}, conn)

	_, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)

	require.Equal(t, "user-ch", externalCH(wandb).ClusterName.SecretKeyRef().Name)
	require.Equal(t, "cluster", externalCH(wandb).ClusterName.SecretKeyRef().Key)
	require.Nil(t, wandb.Spec.Wandb.LegacyOverrides)
}

func TestMapLegacyEnvToCR_NoOverridesNoOp(t *testing.T) {
	client, wandb := newEnvMapFixture(t, nil, &apiv2.ClickHouseConnection{})

	res, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
}

// Unrelated env vars are left untouched (only registered names are promoted).
func TestMapLegacyEnvToCR_UnrelatedEnvUntouched(t *testing.T) {
	client, wandb := newEnvMapFixture(t, map[string]apiv2.LegacyOverrides{
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{litEnv("SOME_OTHER_VAR", "x")}},
	}, &apiv2.ClickHouseConnection{})

	res, err := mapLegacyEnvToCR(context.Background(), client, wandb)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Len(t, wandb.Spec.Wandb.LegacyOverrides[apiv2.LegacyOverridesGlobalKey].Env, 1)
}

func TestRemoveLegacyEnv_PrunesEmptyPreservesResourcesAndOthers(t *testing.T) {
	quantity := corev1.ResourceRequirements{}
	wandb := &apiv2.WeightsAndBiases{}
	wandb.Spec.Wandb.LegacyOverrides = map[string]apiv2.LegacyOverrides{
		// Emptied section with no resources -> pruned.
		apiv2.LegacyOverridesGlobalKey: {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}},
		// Emptied Env but has Resources -> section kept, Env cleared.
		"weave": {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true")}, Resources: &quantity},
		// Other env survives.
		"parquet": {Env: []corev1.EnvVar{litEnv(envClickHouseReplicated, "true"), litEnv("KEEP", "1")}},
	}

	removed := removeLegacyEnv(wandb, envClickHouseReplicated)
	require.True(t, removed)

	overrides := wandb.Spec.Wandb.LegacyOverrides
	require.NotContains(t, overrides, apiv2.LegacyOverridesGlobalKey)
	require.Contains(t, overrides, "weave")
	require.Empty(t, overrides["weave"].Env)
	require.NotNil(t, overrides["weave"].Resources)
	require.Len(t, overrides["parquet"].Env, 1)
	require.Equal(t, "KEEP", overrides["parquet"].Env[0].Name)
}
