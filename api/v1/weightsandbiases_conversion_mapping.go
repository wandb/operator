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
	"encoding/json"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appsv2 "github.com/wandb/operator/api/v2"
)

// OIDCPendingAnnotation holds a JSON-encoded snapshot of v1's
// spec.values.global.auth.oidc subtree when a v1 object is converted to v2.
// v2 expects OIDC fields as SecretKeySelectors, which a stateless conversion
// webhook cannot create, so the raw v1 strings + secret ref are stashed here
// for a downstream reconciler step to materialize.
const OIDCPendingAnnotation = "legacy.operator.wandb.com/oidc-pending"

var validSizes = map[string]appsv2.Size{
	string(appsv2.SizeDev):     appsv2.SizeDev,
	string(appsv2.SizeMicro):   appsv2.SizeMicro,
	string(appsv2.SizeSmall):   appsv2.SizeSmall,
	string(appsv2.SizeMedium):  appsv2.SizeMedium,
	string(appsv2.SizeLarge):   appsv2.SizeLarge,
	string(appsv2.SizeXLarge):  appsv2.SizeXLarge,
	string(appsv2.SizeXXLarge): appsv2.SizeXXLarge,
}

// applyGlobalMappings extracts the supported v1 spec.values.global.* fields
// and writes them onto dst's typed v2 spec (and annotations, for fields that
// need downstream Secret materialization).
func applyGlobalMappings(src *WeightsAndBiases, dst *appsv2.WeightsAndBiases) error {
	values := src.Spec.Values.Object
	if values == nil {
		return nil
	}

	globalMap, found, err := unstructured.NestedMap(values, "global")
	if err != nil {
		return fmt.Errorf("spec.values.global: %w", err)
	}
	if !found {
		return nil
	}

	if err := mapHostnameLicense(globalMap, dst); err != nil {
		return err
	}
	if err := mapSize(globalMap, dst); err != nil {
		return err
	}
	if err := stashOIDCAnnotation(globalMap, dst); err != nil {
		return err
	}

	return nil
}

func mapHostnameLicense(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	host, _, err := unstructured.NestedString(globalMap, "host")
	if err != nil {
		return fmt.Errorf("spec.values.global.host: %w", err)
	}
	if host != "" {
		dst.Spec.Wandb.Hostname = host
	}

	license, _, err := unstructured.NestedString(globalMap, "license")
	if err != nil {
		return fmt.Errorf("spec.values.global.license: %w", err)
	}
	if license != "" {
		dst.Spec.Wandb.License = license
	}

	return nil
}

func mapSize(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	size, found, err := unstructured.NestedString(globalMap, "size")
	if err != nil {
		return fmt.Errorf("spec.values.global.size: %w", err)
	}
	if !found || size == "" {
		return nil
	}

	mapped, ok := validSizes[size]
	if !ok {
		return fmt.Errorf("spec.values.global.size: %q is not a valid v2 size (valid: %s)", size, sortedSizeList())
	}
	dst.Spec.Size = mapped
	return nil
}

func stashOIDCAnnotation(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	oidc, found, err := unstructured.NestedMap(globalMap, "auth", "oidc")
	if err != nil {
		return fmt.Errorf("spec.values.global.auth.oidc: %w", err)
	}
	if !found || len(oidc) == 0 {
		return nil
	}

	payload, err := json.Marshal(oidc)
	if err != nil {
		return fmt.Errorf("marshal global.auth.oidc: %w", err)
	}

	if dst.Annotations == nil {
		dst.Annotations = make(map[string]string)
	}
	dst.Annotations[OIDCPendingAnnotation] = string(payload)
	return nil
}

func sortedSizeList() string {
	names := make([]string, 0, len(validSizes))
	for k := range validSizes {
		names = append(names, k)
	}
	sort.Strings(names)
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}
