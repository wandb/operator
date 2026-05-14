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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appsv2 "github.com/wandb/operator/api/v2"
)

// Annotations that stash v1 sub-trees needing downstream Secret materialization
// by the v2 reconciler. A stateless conversion webhook can't create Secrets, so
// it hands raw v1 values off here and the reconciler turns them into
// SecretKeySelectors on the spec.
const (
	OIDCPendingAnnotation   = "legacy.operator.wandb.com/oidc-pending"
	MySQLPendingAnnotation  = "legacy.operator.wandb.com/mysql-pending"
	RedisPendingAnnotation  = "legacy.operator.wandb.com/redis-pending"
	BucketPendingAnnotation = "legacy.operator.wandb.com/bucket-pending"
)

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
	if err := stashSubtree(globalMap, dst, OIDCPendingAnnotation, "auth", "oidc"); err != nil {
		return err
	}
	if err := stashSubtree(globalMap, dst, MySQLPendingAnnotation, "mysql"); err != nil {
		return err
	}
	if err := stashSubtree(globalMap, dst, RedisPendingAnnotation, "redis"); err != nil {
		return err
	}
	if err := stashBucketAnnotation(globalMap, dst); err != nil {
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

// stashSubtree marshals the nested map at the given path within globalMap
// and writes it to dst's annotations under key. No-op if the path is missing
// or the map is empty.
func stashSubtree(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases, key string, path ...string) error {
	sub, found, err := unstructured.NestedMap(globalMap, path...)
	if err != nil {
		return fmt.Errorf("spec.values.global.%s: %w", strings.Join(path, "."), err)
	}
	if !found || len(sub) == 0 {
		return nil
	}
	return writeAnnotation(dst, key, sub)
}

// stashBucketAnnotation merges v1's global.bucket and global.defaultBucket
// sub-trees into a single annotation so the reconciler has both the
// credentials shape (bucket) and the location shape (defaultBucket) in one
// place. v1 historically populated either or both depending on chart version.
func stashBucketAnnotation(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	bucket, _, err := unstructured.NestedMap(globalMap, "bucket")
	if err != nil {
		return fmt.Errorf("spec.values.global.bucket: %w", err)
	}
	defaultBucket, _, err := unstructured.NestedMap(globalMap, "defaultBucket")
	if err != nil {
		return fmt.Errorf("spec.values.global.defaultBucket: %w", err)
	}
	if len(bucket) == 0 && len(defaultBucket) == 0 {
		return nil
	}

	combined := map[string]interface{}{}
	if len(bucket) > 0 {
		combined["bucket"] = bucket
	}
	if len(defaultBucket) > 0 {
		combined["defaultBucket"] = defaultBucket
	}
	return writeAnnotation(dst, BucketPendingAnnotation, combined)
}

func writeAnnotation(dst *appsv2.WeightsAndBiases, key string, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", key, err)
	}
	if dst.Annotations == nil {
		dst.Annotations = make(map[string]string)
	}
	dst.Annotations[key] = string(payload)
	return nil
}

func sortedSizeList() string {
	names := make([]string, 0, len(validSizes))
	for k := range validSizes {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
