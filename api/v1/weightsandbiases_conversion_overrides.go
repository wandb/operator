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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appsv2 "github.com/wandb/operator/api/v2"
)

// legacyAppSectionKeys are the wandb-base subchart aliases of the v1
// operator-wandb chart — the top-level values keys that configure a deployable
// app, job, or hook. The v1 chart is legacy, so this list is frozen; validity
// of the resulting map keys is judged against the server manifest at reconcile
// time, not here.
var legacyAppSectionKeys = []string{
	"anaconda2",
	"api",
	"app",
	"appRenamePostHook",
	"appRenamePreHook",
	"clickhouseMigrationJob",
	"console",
	"executor",
	"filemeta",
	"filestream",
	"flat-run-fields-updater",
	"frontend",
	"glue",
	"history-reader",
	"history-updater",
	"internalSignerPreHook",
	"lumen-agent",
	"mcp-server",
	"metric-observer",
	"nginx",
	"parquet",
	"parquet-metadata-cache",
	"settingsMigrationJob",
	"weave",
	"weave-evaluate-model-worker",
	"weave-trace",
	"weave-trace-agent-scoring-worker",
	"weave-trace-worker",
}

// legacyKeyRenames maps a helm alias to its v2 manifest application name for
// the unambiguous 1:1 renames. Deliberately no app -> api entry: the monolith's
// overrides were tuned for a different binary, so `app` stays under its own
// key and falls out at manifest validation.
var legacyKeyRenames = map[string]string{
	"nginx":                       "nginx-proxy",
	"weave-evaluate-model-worker": "weave-trace-evaluate-model-worker",
}

const legacyDefaultSize = "small"

// mapLegacyOverrides extracts global and per-application env/extraEnv and
// resource overrides from v1 values into spec.wandb.legacyOverrides. Every
// candidate section with content is copied; the reconciler validates the keys
// against the server manifest (conversion is stateless and cannot resolve it)
// and ignores those that don't map.
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

	for _, key := range legacyAppSectionKeys {
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

		name := key
		if renamed, ok := legacyKeyRenames[key]; ok {
			name = renamed
		}
		overrides[name] = appsv2.LegacyOverrides{Env: env, Resources: resources}
	}

	if len(overrides) > 0 {
		dst.Spec.Wandb.LegacyOverrides = overrides
	}
	return nil
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
