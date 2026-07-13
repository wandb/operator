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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appsv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

const (
	legacyDefaultSize = "small"

	manifestFetchTimeout    = 15 * time.Second
	manifestFailureCooldown = time.Minute
)

// conversionManifestGetter resolves the server manifest; a package-level seam
// so tests (and alternate wiring) can avoid the real OCI/file fetch.
var conversionManifestGetter = serverManifest.GetServerManifest

// Successful fetches need no caching here — GetServerManifest is local-first
// (the ORAS store under /tmp/server-manifest, populated on first download).
// Failures are the gap: the store retries the remote on every call, and in a
// conversion webhook that would stall every v1 write for the full fetch
// timeout while the registry is unreachable, so failures are remembered
// briefly per (repository, version).
var (
	manifestFailuresMu sync.Mutex
	manifestFailures   = map[string]manifestFailure{}
)

type manifestFailure struct {
	err   error
	until time.Time
}

// SetConversionManifestGetter swaps the manifest resolver used by conversion
// and clears the failure cooldowns. Intended for tests; call with nil to
// restore the default resolver.
func SetConversionManifestGetter(getter func(ctx context.Context, repository, version string) (serverManifest.Manifest, error)) {
	manifestFailuresMu.Lock()
	defer manifestFailuresMu.Unlock()
	if getter == nil {
		getter = serverManifest.GetServerManifest
	}
	conversionManifestGetter = getter
	manifestFailures = map[string]manifestFailure{}
}

// legacyManifestApps maps each application name in the server manifest for
// version to the v1 values key holding its section: the application's
// legacyKey when the manifest sets one (renamed helm aliases like nginx ->
// nginx-proxy), else the name itself. The manifest is resolved from the
// default manifest repository — v1 values have no repository field, so
// conversion uses the same default the v2 defaulting webhook applies.
func legacyManifestApps(version string) (map[string]string, error) {
	repository := appsv2.DefaultManifestRepository
	key := repository + "|" + version

	manifestFailuresMu.Lock()
	getter := conversionManifestGetter
	if failure, ok := manifestFailures[key]; ok && time.Now().Before(failure.until) {
		manifestFailuresMu.Unlock()
		return nil, failure.err
	}
	manifestFailuresMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), manifestFetchTimeout)
	defer cancel()

	m, err := getter(ctx, repository, version)
	if err != nil {
		manifestFailuresMu.Lock()
		manifestFailures[key] = manifestFailure{err: err, until: time.Now().Add(manifestFailureCooldown)}
		manifestFailuresMu.Unlock()
		return nil, err
	}

	apps := make(map[string]string, len(m.Applications))
	for name, app := range m.Applications {
		valuesKey := app.LegacyKey
		if valuesKey == "" {
			valuesKey = name
		}
		apps[name] = valuesKey
	}
	return apps, nil
}

// mapLegacyOverrides extracts global and per-application env/extraEnv and
// resource overrides from v1 values into spec.wandb.legacyOverrides. The
// server manifest for the converted version (set by mapVersion, which runs
// first) is the authority on which sections are applications; override-shaped
// sections that match no manifest application are logged and skipped.
//
// Manifest resolution is best-effort: conversion stays a pure function of
// (values, version), and a fetch failure — offline cluster, unpublished
// version — must never make v1 objects unservable, so on error only the
// per-application extraction is skipped (with a log); global env still
// converts, and the raw values remain in the v1-values annotation.
func mapLegacyOverrides(values map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	overrides := map[string]appsv2.LegacyOverrides{}

	globalMap, _, err := unstructured.NestedMap(values, "global")
	if err != nil {
		return fmt.Errorf("spec.values.global: %w", err)
	}
	if len(globalMap) > 0 {
		env, err := legacyEnvFromSection(globalMap, "global")
		if err != nil {
			return err
		}
		if len(env) > 0 {
			overrides[appsv2.LegacyOverridesGlobalKey] = appsv2.LegacyOverrides{Env: env}
		}
	}

	globalSize, _, err := unstructured.NestedString(globalMap, "size")
	if err != nil {
		return fmt.Errorf("spec.values.global.size: %w", err)
	}

	if err := mapPerAppLegacyOverrides(values, dst.Spec.Wandb.Version, globalSize, overrides); err != nil {
		return err
	}

	if len(overrides) > 0 {
		dst.Spec.Wandb.LegacyOverrides = overrides
	}
	return nil
}

func mapPerAppLegacyOverrides(values map[string]interface{}, version, globalSize string, overrides map[string]appsv2.LegacyOverrides) error {
	if version == "" {
		logger.Info("no version derived from v1 values; skipping per-application legacy overrides")
		return nil
	}
	apps, err := legacyManifestApps(version)
	if err != nil {
		logger.Error(err, "failed to resolve server manifest; skipping per-application legacy overrides",
			"version", version)
		return nil
	}

	appNames := make([]string, 0, len(apps))
	for name := range apps {
		appNames = append(appNames, name)
	}
	sort.Strings(appNames)

	for _, name := range appNames {
		key := apps[name]
		section, found, err := unstructured.NestedMap(values, key)
		if err != nil {
			return fmt.Errorf("spec.values.%s: %w", key, err)
		}
		if !found {
			continue
		}

		env, err := legacyEnvFromSection(section, key)
		if err != nil {
			return err
		}
		resources, err := legacyResourcesFromSection(section, key, globalSize)
		if err != nil {
			return err
		}
		if len(env) == 0 && resources == nil {
			continue
		}
		overrides[name] = appsv2.LegacyOverrides{Env: env, Resources: resources}
	}

	logUnmappedLegacySections(values, apps, version)
	return nil
}

// logUnmappedLegacySections logs every top-level values section that carries
// the override shape we extract (env/extraEnv/sizing) but is read by no
// manifest application (by name or legacyKey) — e.g. the v1 monolith `app` or
// `console`. Their values are not converted and remain only in the v1-values
// annotation.
func logUnmappedLegacySections(values map[string]interface{}, apps map[string]string, version string) {
	mappedKeys := make(map[string]struct{}, len(apps))
	for _, valuesKey := range apps {
		mappedKeys[valuesKey] = struct{}{}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if key == "global" {
			continue
		}
		if _, ok := mappedKeys[key]; ok {
			continue
		}
		section, ok := values[key].(map[string]interface{})
		if !ok || !hasLegacyOverrideShape(section) {
			continue
		}
		logger.Info("legacy values section does not map to any application in the server manifest; its env/resources are not converted",
			"section", key, "version", version)
	}
}

// hasLegacyOverrideShape reports whether a values section carries any of the
// keys legacy override extraction reads. The flat `resources` key is
// deliberately excluded: infra subchart sections (mysql, redis, …) legitimately
// carry it and would be false positives.
func hasLegacyOverrideShape(section map[string]interface{}) bool {
	for _, key := range []string{"env", "extraEnv", "sizing"} {
		if _, ok := section[key]; ok {
			return true
		}
	}
	return false
}

// legacyEnvFromSection merges <section>.env over <section>.extraEnv (the
// chart's layer precedence) and renders the result as a name-sorted EnvVar
// list so conversion output is deterministic across round-trips.
func legacyEnvFromSection(section map[string]interface{}, sectionName string) ([]corev1.EnvVar, error) {
	merged := map[string]interface{}{}
	for _, sub := range []string{"extraEnv", "env"} {
		m, found, err := unstructured.NestedMap(section, sub)
		if err != nil {
			return nil, fmt.Errorf("spec.values.%s.%s: %w", sectionName, sub, err)
		}
		if !found {
			continue
		}
		for k, v := range m {
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		return nil, nil
	}

	vars := make([]corev1.EnvVar, 0, len(merged))
	for name, raw := range merged {
		envVar, ok, err := legacyEnvVar(name, raw, sectionName)
		if err != nil {
			return nil, err
		}
		if ok {
			vars = append(vars, envVar)
		}
	}
	if len(vars) == 0 {
		return nil, nil
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
	return vars, nil
}

// legacyEnvVar renders one helm env map entry as an EnvVar. Map values are
// full EnvVar bodies (valueFrom etc.) and decode strictly — a malformed body
// fails conversion like other mappers. Scalars are string-coerced the way
// helm's toString rendered them. Helm template expressions can't be evaluated
// outside helm, so those entries are dropped with a log, never an error.
func legacyEnvVar(name string, raw interface{}, sectionName string) (corev1.EnvVar, bool, error) {
	if body, isMap := raw.(map[string]interface{}); isMap {
		payload, err := json.Marshal(body)
		if err != nil {
			return corev1.EnvVar{}, false, fmt.Errorf("spec.values.%s env %s: %w", sectionName, name, err)
		}
		dec := json.NewDecoder(bytes.NewReader(payload))
		dec.DisallowUnknownFields()
		var envVar corev1.EnvVar
		if err := dec.Decode(&envVar); err != nil {
			return corev1.EnvVar{}, false, fmt.Errorf("spec.values.%s env %s: %w", sectionName, name, err)
		}
		envVar.Name = name
		if strings.Contains(envVar.Value, "{{") {
			logger.Info("dropping legacy env var with helm template value",
				"section", sectionName, "name", name)
			return corev1.EnvVar{}, false, nil
		}
		return envVar, true, nil
	}

	s, ok := scalarToString(raw)
	if !ok {
		logger.Info("dropping legacy env var with non-scalar value",
			"section", sectionName, "name", name)
		return corev1.EnvVar{}, false, nil
	}
	if strings.Contains(s, "{{") {
		logger.Info("dropping legacy env var with helm template value",
			"section", sectionName, "name", name)
		return corev1.EnvVar{}, false, nil
	}
	return corev1.EnvVar{Name: name, Value: s}, true, nil
}

// legacyResourcesFromSection deep-merges the resource fragments a section
// sets — sizing.default.resources, then sizing.<effective size>.resources,
// then the legacy flat resources key — mirroring the chart's merge order.
// Only fragments present in the values contribute, so a section that never
// touched resources yields nil and v2 manifest sizing applies untouched.
func legacyResourcesFromSection(section map[string]interface{}, sectionName, globalSize string) (*corev1.ResourceRequirements, error) {
	size, _, err := unstructured.NestedString(section, "size")
	if err != nil {
		return nil, fmt.Errorf("spec.values.%s.size: %w", sectionName, err)
	}
	if size == "" {
		size = globalSize
	}
	if size == "" {
		size = legacyDefaultSize
	}

	merged := map[string]interface{}{}
	for _, path := range [][]string{
		{"sizing", "default", "resources"},
		{"sizing", size, "resources"},
		{"resources"},
	} {
		m, found, err := unstructured.NestedMap(section, path...)
		if err != nil {
			return nil, fmt.Errorf("spec.values.%s.%s: %w", sectionName, strings.Join(path, "."), err)
		}
		if found {
			mergeLegacyValueMaps(merged, m)
		}
	}
	if len(merged) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("spec.values.%s resources: %w", sectionName, err)
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	var resources corev1.ResourceRequirements
	if err := dec.Decode(&resources); err != nil {
		return nil, fmt.Errorf("spec.values.%s resources: %w", sectionName, err)
	}
	return &resources, nil
}

// mergeLegacyValueMaps deep-merges src into dst, overwriting non-map values —
// the same shape of merge helm applies to values maps.
func mergeLegacyValueMaps(dst, src map[string]interface{}) {
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				mergeLegacyValueMaps(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
}
