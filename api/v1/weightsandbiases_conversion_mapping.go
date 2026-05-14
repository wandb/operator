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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appsv2 "github.com/wandb/operator/api/v2"
)

// Default Secret keys used by v1's legacy ref blocks when only the Secret
// name was specified.
const (
	defaultMySQLPasswordSecretKey = "MYSQL_PASSWORD"
	defaultRedisPasswordSecretKey = "REDIS_PASSWORD"
	defaultBucketAccessKeyName    = "ACCESS_KEY"
	defaultBucketSecretKeyName    = "SECRET_KEY"
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
	if err := mapMySQL(globalMap, dst); err != nil {
		return err
	}
	if err := mapRedis(globalMap, dst); err != nil {
		return err
	}
	if err := mapBucket(globalMap, dst); err != nil {
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

// mapBucket handles v1's two bucket-related sub-trees:
//   - global.bucket: optionally contains a `secret: {secretName, accessKeyName,
//     secretKeyName}` block plus any number of literal fields (provider, name,
//     region, path, kmsKey, etc.).
//   - global.defaultBucket: literal fields only.
//
// The `bucket.secret` block is the one piece of v1 with an explicit Secret
// reference shape, so it goes straight to spec.objectStore.externalObjectStore
// .{AccessKey, SecretKey}. Everything else is merged into a single flat
// annotation payload (bucket entries win over defaultBucket on collision) so a
// follow-up reconciler step can later materialize the literals.
func mapBucket(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
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

	if sec, ok, err := unstructured.NestedMap(bucket, "secret"); err != nil {
		return fmt.Errorf("spec.values.global.bucket.secret: %w", err)
	} else if ok {
		name, _, _ := unstructured.NestedString(sec, "secretName")
		if name != "" {
			accessKeyName, _, _ := unstructured.NestedString(sec, "accessKeyName")
			if accessKeyName == "" {
				accessKeyName = defaultBucketAccessKeyName
			}
			secretKeyName, _, _ := unstructured.NestedString(sec, "secretKeyName")
			if secretKeyName == "" {
				secretKeyName = defaultBucketSecretKeyName
			}
			dst.Spec.ObjectStore.ExternalObjectStore = &appsv2.ObjectStoreConnection{
				AccessKey: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
					Key:                  accessKeyName,
				},
				SecretKey: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
					Key:                  secretKeyName,
				},
			}
		}
	}

	merged := map[string]interface{}{}
	for k, v := range defaultBucket {
		if isNonEmptyValue(v) {
			merged[k] = v
		}
	}
	for k, v := range bucket {
		if k == "secret" {
			continue
		}
		if isNonEmptyValue(v) {
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return writeAnnotation(dst, BucketPendingAnnotation, merged)
}

func isNonEmptyValue(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	default:
		return true
	}
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

// mysqlFields maps each v1 `global.mysql.<key>` to the corresponding setter
// on a *MysqlConnection. Order is preserved so any test failures point at the
// first offending field.
var mysqlFields = []struct {
	v1Key  string
	setRef func(*appsv2.MysqlConnection, corev1.SecretKeySelector)
}{
	{"host", func(c *appsv2.MysqlConnection, s corev1.SecretKeySelector) { c.Host = s }},
	{"port", func(c *appsv2.MysqlConnection, s corev1.SecretKeySelector) { c.Port = s }},
	{"database", func(c *appsv2.MysqlConnection, s corev1.SecretKeySelector) { c.Database = s }},
	{"user", func(c *appsv2.MysqlConnection, s corev1.SecretKeySelector) { c.Username = s }},
	{"password", func(c *appsv2.MysqlConnection, s corev1.SecretKeySelector) { c.Password = s }},
	{"caCert", func(c *appsv2.MysqlConnection, s corev1.SecretKeySelector) { c.SslCa = s }},
}

// mapMySQL splits v1 global.mysql values by shape:
//   - {valueFrom: {secretKeyRef: {name, key}}} maps go to typed
//     spec.mysql.externalMysql.* SecretKeySelectors directly.
//   - Plain scalars are written to the mysql-pending annotation so the
//     reconciler can materialize them into a Secret.
//   - The legacy `passwordSecret.{name, passwordKey}` block also goes
//     direct to externalMysql.password, but only when no password ref
//     has already been set by a `password.valueFrom` block.
func mapMySQL(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	mysqlMap, found, err := unstructured.NestedMap(globalMap, "mysql")
	if err != nil {
		return fmt.Errorf("spec.values.global.mysql: %w", err)
	}
	if !found || len(mysqlMap) == 0 {
		return nil
	}

	var conn *appsv2.MysqlConnection
	ensureConn := func() *appsv2.MysqlConnection {
		if conn == nil {
			conn = &appsv2.MysqlConnection{}
		}
		return conn
	}

	remaining := map[string]interface{}{}

	for _, f := range mysqlFields {
		raw, ok := mysqlMap[f.v1Key]
		if !ok {
			continue
		}
		ref, literal, classifyErr := classifyValueFromOrLiteral(raw)
		if classifyErr != nil {
			return fmt.Errorf("spec.values.global.mysql.%s: %w", f.v1Key, classifyErr)
		}
		switch {
		case ref != nil:
			f.setRef(ensureConn(), *ref)
		case literal != nil:
			remaining[f.v1Key] = literal
		}
	}

	if ps, ok, err := unstructured.NestedMap(mysqlMap, "passwordSecret"); err != nil {
		return fmt.Errorf("spec.values.global.mysql.passwordSecret: %w", err)
	} else if ok {
		name, _, _ := unstructured.NestedString(ps, "name")
		alreadyHasPassword := conn != nil && conn.Password.Name != ""
		if name != "" && !alreadyHasPassword {
			key, _, _ := unstructured.NestedString(ps, "passwordKey")
			if key == "" {
				key = defaultMySQLPasswordSecretKey
			}
			ensureConn().Password = corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Key:                  key,
			}
			delete(remaining, "password")
		}
	}

	if conn != nil {
		dst.Spec.MySQL.ExternalMysql = conn
	}
	if len(remaining) > 0 {
		return writeAnnotation(dst, MySQLPendingAnnotation, remaining)
	}
	return nil
}

// redisFields maps each v1 `global.redis.<key>` to the corresponding setter
// on a *RedisConnection.
var redisFields = []struct {
	v1Key  string
	setRef func(*appsv2.RedisConnection, corev1.SecretKeySelector)
}{
	{"host", func(c *appsv2.RedisConnection, s corev1.SecretKeySelector) { c.Host = s }},
	{"port", func(c *appsv2.RedisConnection, s corev1.SecretKeySelector) { c.Port = s }},
	{"password", func(c *appsv2.RedisConnection, s corev1.SecretKeySelector) { c.Password = s }},
	{"caCert", func(c *appsv2.RedisConnection, s corev1.SecretKeySelector) { c.SslCa = s }},
}

// mapRedis splits v1 global.redis values by shape:
//   - {valueFrom: {secretKeyRef: {name, key}}} maps go to typed
//     spec.redis.externalRedis.* SecretKeySelectors directly.
//   - Plain scalars are written to the redis-pending annotation so the
//     reconciler can materialize them into a Secret.
//   - The legacy `secret.{secretName, secretKey}` block also goes direct to
//     externalRedis.password, but only when no password ref has already been
//     set by a `password.valueFrom` block.
//
// Fields outside the known set (e.g. v1's `external`, `parameters`, `params`)
// have no v2 equivalent and are dropped.
func mapRedis(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	redisMap, found, err := unstructured.NestedMap(globalMap, "redis")
	if err != nil {
		return fmt.Errorf("spec.values.global.redis: %w", err)
	}
	if !found || len(redisMap) == 0 {
		return nil
	}

	var conn *appsv2.RedisConnection
	ensureConn := func() *appsv2.RedisConnection {
		if conn == nil {
			conn = &appsv2.RedisConnection{}
		}
		return conn
	}

	remaining := map[string]interface{}{}

	for _, f := range redisFields {
		raw, ok := redisMap[f.v1Key]
		if !ok {
			continue
		}
		ref, literal, classifyErr := classifyValueFromOrLiteral(raw)
		if classifyErr != nil {
			return fmt.Errorf("spec.values.global.redis.%s: %w", f.v1Key, classifyErr)
		}
		switch {
		case ref != nil:
			f.setRef(ensureConn(), *ref)
		case literal != nil:
			remaining[f.v1Key] = literal
		}
	}

	if sec, ok, err := unstructured.NestedMap(redisMap, "secret"); err != nil {
		return fmt.Errorf("spec.values.global.redis.secret: %w", err)
	} else if ok {
		name, _, _ := unstructured.NestedString(sec, "secretName")
		alreadyHasPassword := conn != nil && conn.Password.Name != ""
		if name != "" && !alreadyHasPassword {
			key, _, _ := unstructured.NestedString(sec, "secretKey")
			if key == "" {
				key = defaultRedisPasswordSecretKey
			}
			ensureConn().Password = corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Key:                  key,
			}
			delete(remaining, "password")
		}
	}

	if conn != nil {
		dst.Spec.Redis.ExternalRedis = conn
	}
	if len(remaining) > 0 {
		return writeAnnotation(dst, RedisPendingAnnotation, remaining)
	}
	return nil
}

// classifyValueFromOrLiteral inspects a v1 field value. If it is a map
// matching `{valueFrom: {secretKeyRef: {name, key}}}`, it returns a typed
// SecretKeySelector. Otherwise, if the value is a non-zero scalar (string
// or number), it returns the literal. Empty/nil scalars return (nil, nil).
// A malformed map (a map missing the valueFrom path) returns an error.
func classifyValueFromOrLiteral(raw interface{}) (*corev1.SecretKeySelector, interface{}, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil, nil
	case string:
		if v == "" {
			return nil, nil, nil
		}
		return nil, v, nil
	case map[string]interface{}:
		name, _, _ := unstructured.NestedString(v, "valueFrom", "secretKeyRef", "name")
		key, _, _ := unstructured.NestedString(v, "valueFrom", "secretKeyRef", "key")
		if name == "" || key == "" {
			return nil, nil, fmt.Errorf("map value must be {valueFrom: {secretKeyRef: {name, key}}}")
		}
		return &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			Key:                  key,
		}, nil, nil
	default:
		// numbers (port arrives as float64 from a JSON decode), bools, etc.
		// Treat as literal; the reconciler stringifies as needed.
		return nil, v, nil
	}
}

func sortedSizeList() string {
	names := make([]string, 0, len(validSizes))
	for k := range validSizes {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
