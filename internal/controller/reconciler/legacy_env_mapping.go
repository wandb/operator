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
	"fmt"
	"reflect"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
)

// Legacy env vars promoted into typed v2 fields at reconcile. v1 users set these
// as raw pod env; conversion carries them verbatim into legacyOverrides, and this
// step maps the ones we model into typed fields, then drops them.
const (
	envClickHouseReplicated        = "WF_CLICKHOUSE_REPLICATED"
	envClickHouseReplicatedCluster = "WF_CLICKHOUSE_REPLICATED_CLUSTER"

	// Data keys inside the operator-owned <cr>-clickhouse-converted Secret.
	convertedClickHouseReplicatedKey = "replicated"
	convertedClickHouseClusterKey    = "replicatedCluster"
)

type mappingScope int

const (
	// scopeDatastore: one value for the whole datastore. Collected across every
	// legacyOverrides section; per-app beats global; disagreement is an error.
	scopeDatastore mappingScope = iota
)

type conflictPolicy int

const (
	keepCR                    conflictPolicy = iota // never overwrite a field already set
	overrideConversionDerived                       // overwrite when unset or operator-owned
)

// applyFn owns the managed-vs-external guard, Secret materialization / selector
// repointing, and setting the field from the resolved EnvVar. remove=true means
// strip the env from legacyOverrides (mapped, or intentionally dropped);
// remove=false leaves it as a raw-env passthrough (a valueFrom shape the
// Secret-only target can't represent).
type applyFn func(ctx context.Context, c ctrlClient.Client, w *apiv2.WeightsAndBiases, env corev1.EnvVar) (remove bool, err error)

type legacyEnvMapping struct {
	env   string
	scope mappingScope
	apply applyFn
}

// legacyEnvMappings is the hardcoded registry of v1 env vars the reconciler
// promotes into typed v2 fields. Bound to the CRD; grows as we model more knobs.
// Only add an env var here when a manifest source re-injects its value from the
// target field — otherwise removal silently drops it from the pod.
var legacyEnvMappings = []legacyEnvMapping{
	{
		env:   envClickHouseReplicated,
		scope: scopeDatastore,
		apply: externalClickHouseSelector(convertedClickHouseReplicatedKey,
			func(c *apiv2.ClickHouseConnection) *corev1.SecretKeySelector { return &c.Replicated },
			overrideConversionDerived),
	},
	{
		env:   envClickHouseReplicatedCluster,
		scope: scopeDatastore,
		apply: externalClickHouseSelector(convertedClickHouseClusterKey,
			func(c *apiv2.ClickHouseConnection) *corev1.SecretKeySelector { return &c.ClusterName },
			keepCR),
	},
}

// mapLegacyEnvToCR promotes registered legacy env vars into typed spec fields,
// then removes them from legacyOverrides (the field is now authoritative).
// Mirrors migrateLegacyAnnotations: mutate spec -> Update -> requeue, and the
// caller short-circuits the rest of reconcile on a non-zero RequeueAfter.
func mapLegacyEnvToCR(ctx context.Context, c ctrlClient.Client, w *apiv2.WeightsAndBiases) (ctrl.Result, error) {
	if len(w.Spec.Wandb.LegacyOverrides) == 0 {
		return ctrl.Result{}, nil
	}

	changed := false
	for _, m := range legacyEnvMappings {
		env, found, err := resolveLegacyEnv(m, w.Spec.Wandb.LegacyOverrides)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !found {
			continue
		}
		remove, err := m.apply(ctx, c, w, env)
		if err != nil {
			return ctrl.Result{}, err
		}
		if remove && removeLegacyEnv(w, m.env) {
			changed = true
		}
	}
	if !changed {
		return ctrl.Result{}, nil
	}
	if err := c.Update(ctx, w); err != nil {
		return ctrl.Result{}, fmt.Errorf("update CR after legacy env mapping: %w", err)
	}
	return ctrl.Result{RequeueAfter: time.Second}, nil
}

func resolveLegacyEnv(m legacyEnvMapping, overrides map[string]apiv2.LegacyOverrides) (corev1.EnvVar, bool, error) {
	switch m.scope {
	case scopeDatastore:
		return resolveDatastoreEnv(overrides, m.env)
	default:
		return corev1.EnvVar{}, false, fmt.Errorf("legacy env %q: unsupported mapping scope %d", m.env, m.scope)
	}
}

// resolveDatastoreEnv finds one env var across legacyOverrides sections and
// returns the winning EnvVar (Value or ValueFrom): per-app beats "global"; apps
// that disagree are an error. Agreement compares the whole body, so a literal and
// a secretKeyRef for the same var are a genuine conflict.
func resolveDatastoreEnv(overrides map[string]apiv2.LegacyOverrides, name string) (corev1.EnvVar, bool, error) {
	var globalEnv corev1.EnvVar
	var globalSet bool
	var appEnv corev1.EnvVar
	var appSet bool
	var appSrc string
	for _, key := range sortedLegacyKeys(overrides) {
		for _, e := range overrides[key].Env {
			if e.Name != name || (e.Value == "" && e.ValueFrom == nil) {
				continue
			}
			if key == apiv2.LegacyOverridesGlobalKey {
				if !globalSet {
					globalEnv, globalSet = e, true
				}
				continue
			}
			if appSet && !sameEnvBody(e, appEnv) {
				return corev1.EnvVar{}, false, fmt.Errorf(
					"legacyOverrides: %s at %q conflicts with %q; ClickHouse replication is a "+
						"property of the datastore, so every application must agree",
					name, appSrc, key)
			}
			appEnv, appSet, appSrc = e, true, key
		}
	}
	switch {
	case appSet:
		return appEnv, true, nil
	case globalSet:
		return globalEnv, true, nil
	default:
		return corev1.EnvVar{}, false, nil
	}
}

// sameEnvBody compares the payload (not the name) of two EnvVars.
func sameEnvBody(a, b corev1.EnvVar) bool {
	return a.Value == b.Value && reflect.DeepEqual(a.ValueFrom, b.ValueFrom)
}

// externalClickHouseSelector maps a resolved env var into an external
// ClickHouseConnection SecretKeySelector field. A literal value is written into
// the operator-owned converted Secret; a secretKeyRef source repoints the field
// at the user's Secret; any other valueFrom shape passes through untouched.
func externalClickHouseSelector(
	dataKey string,
	pick func(*apiv2.ClickHouseConnection) *corev1.SecretKeySelector,
	policy conflictPolicy,
) applyFn {
	return func(ctx context.Context, c ctrlClient.Client, w *apiv2.WeightsAndBiases, env corev1.EnvVar) (bool, error) {
		spec, ok := w.Spec.ClickHouse[apiv2.DefaultInstanceName]
		if !ok || spec.ExternalClickHouse == nil {
			// Managed or unconfigured: managed derives its own topology, so drop.
			return true, nil
		}
		target := pick(spec.ExternalClickHouse)

		isLiteral := env.ValueFrom == nil && env.Value != ""
		hasSecretRef := env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil
		if !isLiteral && !hasSecretRef {
			logx.GetSlog(ctx).Warn("legacy env has an unrepresentable valueFrom; leaving it as a raw override",
				"env", env.Name)
			return false, nil // passthrough: leave the env as raw pod env
		}

		secretName := clickHouseConvertedSecretName(w)
		userOwned := target.Name != "" && target.Name != secretName
		if (policy == keepCR && target.Name != "") || (policy == overrideConversionDerived && userOwned) {
			return true, nil // field already authoritative; just drop the env
		}

		if hasSecretRef {
			*target = *env.ValueFrom.SecretKeyRef // point directly at the user's Secret
			return true, nil
		}
		if err := upsertConvertedSecretKeys(ctx, c, w, secretName, map[string][]byte{dataKey: []byte(env.Value)}); err != nil {
			return false, err
		}
		*target = secretSelector(secretName, dataKey)
		return true, nil
	}
}

// removeLegacyEnv strips name from every legacyOverrides section, pruning a
// section whose Env empties and carries no Resources, and the map when it
// empties. Returns whether anything was removed.
func removeLegacyEnv(w *apiv2.WeightsAndBiases, name string) bool {
	overrides := w.Spec.Wandb.LegacyOverrides
	removed := false
	for key, ov := range overrides {
		var kept []corev1.EnvVar
		for _, e := range ov.Env {
			if e.Name == name {
				removed = true
				continue
			}
			kept = append(kept, e)
		}
		if len(kept) == len(ov.Env) {
			continue
		}
		ov.Env = kept
		if len(ov.Env) == 0 && ov.Resources == nil {
			delete(overrides, key)
			continue
		}
		overrides[key] = ov
	}
	if len(overrides) == 0 {
		w.Spec.Wandb.LegacyOverrides = nil
	}
	return removed
}

func sortedLegacyKeys(m map[string]apiv2.LegacyOverrides) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
