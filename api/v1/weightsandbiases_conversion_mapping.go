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
	"k8s.io/utils/ptr"

	appsv2 "github.com/wandb/operator/api/v2"
)

// Default Secret keys used by v1's legacy ref blocks when only the Secret
// name was specified.
const (
	defaultMySQLPasswordSecretKey = "MYSQL_PASSWORD"
	defaultRedisPasswordSecretKey = "REDIS_PASSWORD"
	defaultOIDCClientSecretKey    = "OIDC_SECRET"
	defaultBucketAccessKeyName    = "ACCESS_KEY"
	defaultBucketSecretKeyName    = "SECRET_KEY"
)

// Annotations carrying v1 literals the reconciler materializes into Secrets
// post-conversion (the webhook is stateless and can't create them itself).
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

// applyValueMappings is the top-level conversion orchestrator. It resolves
// the authoritative values (active-spec Secret if present, else the CR),
// runs peer-of-global mappers inline, and delegates global.* to
// applyGlobalMappings.
func applyValueMappings(src *WeightsAndBiases, dst *appsv2.WeightsAndBiases) error {
	values, err := resolveValues(src)
	if err != nil {
		return err
	}
	if values == nil {
		return nil
	}

	if err := mapVersion(values, dst); err != nil {
		return err
	}
	if err := mapServiceAccountAnnotations(values, dst); err != nil {
		return err
	}
	if err := mapInternalJWTIssuer(values, dst); err != nil {
		return err
	}
	if err := mapIngress(values, dst); err != nil {
		return err
	}

	globalMap, found, err := unstructured.NestedMap(values, "global")
	if err != nil {
		return fmt.Errorf("spec.values.global: %w", err)
	}
	if found {
		if err := applyGlobalMappings(globalMap, dst); err != nil {
			return err
		}
	}

	return nil
}

// applyGlobalMappings runs every mapper sourced from spec.values.global.
func applyGlobalMappings(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	if err := mapHostnameLicense(globalMap, dst); err != nil {
		return err
	}
	if err := mapSize(globalMap, dst); err != nil {
		return err
	}
	if err := mapOIDC(globalMap, dst); err != nil {
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

// mapVersion sets spec.wandb.version from app.image.tag, falling back to
// api.image.tag.
func mapVersion(values map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	if tag, found, err := unstructured.NestedString(values, "app", "image", "tag"); err != nil {
		return fmt.Errorf("spec.values.app.image.tag: %w", err)
	} else if found && tag != "" {
		dst.Spec.Wandb.Version = tag
		return nil
	}
	if tag, found, err := unstructured.NestedString(values, "api", "image", "tag"); err != nil {
		return fmt.Errorf("spec.values.api.image.tag: %w", err)
	} else if found && tag != "" {
		dst.Spec.Wandb.Version = tag
	}
	return nil
}

// mapServiceAccountAnnotations maps v1's per-sub-chart ServiceAccount
// annotations to v2's single spec.wandb.serviceAccount.annotations,
// preferring `app` and falling back to `api`.
func mapServiceAccountAnnotations(values map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	anns, err := readServiceAccountAnnotations(values, "app")
	if err != nil {
		return err
	}
	if len(anns) == 0 {
		anns, err = readServiceAccountAnnotations(values, "api")
		if err != nil {
			return err
		}
	}
	if len(anns) > 0 {
		dst.Spec.Wandb.ServiceAccount.Annotations = anns
	}
	return nil
}

// mapInternalJWTIssuer pulls the first entry from global.internalJWTMap (or
// app as fallback) into spec.wandb.internalServiceAuth.oidcIssuer.
func mapInternalJWTIssuer(values map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	issuer, err := readFirstInternalJWTIssuer(values, "global")
	if err != nil {
		return err
	}
	if issuer == "" {
		issuer, err = readFirstInternalJWTIssuer(values, "app")
		if err != nil {
			return err
		}
	}
	if issuer != "" {
		dst.Spec.Wandb.InternalServiceAuth.OIDCIssuer = issuer
	}
	return nil
}

// mapIngress maps v1's ingress section to spec.networking and
// spec.wandb.additionalHostnames. ingress.install and ingress.create both
// default to true in the chart, so an absent or partial ingress block still
// enables networking mode "ingress".
func mapIngress(values map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	ingressMap, _, err := unstructured.NestedMap(values, "ingress")
	if err != nil {
		return fmt.Errorf("spec.values.ingress: %w", err)
	}

	if !boolFromValues(ingressMap, true, "install") || !boolFromValues(ingressMap, true, "create") {
		return nil
	}

	if dst.Spec.Networking.Mode == "" {
		dst.Spec.Networking.Mode = appsv2.NetworkingModeIngress
	}

	if class, _, err := unstructured.NestedString(ingressMap, "class"); err != nil {
		return fmt.Errorf("spec.values.ingress.class: %w", err)
	} else if class != "" {
		if dst.Spec.Networking.Ingress == nil {
			dst.Spec.Networking.Ingress = &appsv2.IngressConfig{}
		}
		dst.Spec.Networking.Ingress.IngressClassName = ptr.To(class)
	}

	if anns, _, err := unstructured.NestedStringMap(ingressMap, "annotations"); err != nil {
		return fmt.Errorf("spec.values.ingress.annotations: %w", err)
	} else if len(anns) > 0 {
		dst.Spec.Networking.Annotations = anns
	}

	if hosts, _, err := unstructured.NestedStringSlice(ingressMap, "additionalHosts"); err != nil {
		return fmt.Errorf("spec.values.ingress.additionalHosts: %w", err)
	} else if len(hosts) > 0 {
		dst.Spec.Wandb.AdditionalHostnames = hosts
	}

	if name, err := firstIngressTLSSecretName(ingressMap); err != nil {
		return err
	} else if name != "" {
		if dst.Spec.Networking.TLS == nil {
			dst.Spec.Networking.TLS = &appsv2.TLSConfig{}
		}
		dst.Spec.Networking.TLS.SecretName = name
	}

	return nil
}

// firstIngressTLSSecretName returns the secretName on the first ingress.tls
// entry. v2 carries only a single TLS Secret; multi-cert v1 configs collapse
// to the first entry.
func firstIngressTLSSecretName(ingressMap map[string]interface{}) (string, error) {
	list, _, err := unstructured.NestedSlice(ingressMap, "tls")
	if err != nil {
		return "", fmt.Errorf("spec.values.ingress.tls: %w", err)
	}
	if len(list) == 0 {
		return "", nil
	}
	entry, ok := list[0].(map[string]interface{})
	if !ok {
		return "", nil
	}
	name, _, err := unstructured.NestedString(entry, "secretName")
	if err != nil {
		return "", fmt.Errorf("spec.values.ingress.tls[0].secretName: %w", err)
	}
	return name, nil
}

// boolFromValues reads a bool at path within m, returning def when the key
// is absent or the value isn't a bool. Used to apply v1 chart defaults
// during conversion.
func boolFromValues(m map[string]interface{}, def bool, path ...string) bool {
	if m == nil {
		return def
	}
	v, found, err := unstructured.NestedBool(m, path...)
	if err != nil || !found {
		return def
	}
	return v
}

// readFirstInternalJWTIssuer returns the issuer on the first internalJWTMap
// entry under values.<service>, or "" when the list is missing or empty.
func readFirstInternalJWTIssuer(values map[string]interface{}, service string) (string, error) {
	list, found, err := unstructured.NestedSlice(values, service, "internalJWTMap")
	if err != nil {
		return "", fmt.Errorf("spec.values.%s.internalJWTMap: %w", service, err)
	}
	if !found || len(list) == 0 {
		return "", nil
	}
	entry, ok := list[0].(map[string]interface{})
	if !ok {
		return "", nil
	}
	issuer, _, err := unstructured.NestedString(entry, "issuer")
	if err != nil {
		return "", fmt.Errorf("spec.values.%s.internalJWTMap[0].issuer: %w", service, err)
	}
	return issuer, nil
}

// readServiceAccountAnnotations reads values.<service>.serviceAccount.annotations.
func readServiceAccountAnnotations(values map[string]interface{}, service string) (map[string]string, error) {
	anns, found, err := unstructured.NestedStringMap(values, service, "serviceAccount", "annotations")
	if err != nil {
		return nil, fmt.Errorf("spec.values.%s.serviceAccount.annotations: %w", service, err)
	}
	if !found {
		return nil, nil
	}
	return anns, nil
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

// mapBucket pulls `bucket.secret` directly into externalObjectStore
// .{AccessKey,SecretKey} and merges the remaining bucket + defaultBucket
// literals (bucket wins on collision) into the bucket-pending annotation.
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

	dst.Spec.ObjectStore.ExternalObjectStore = &appsv2.ObjectStoreConnection{}

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
			dst.Spec.ObjectStore.ExternalObjectStore.AccessKey = corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Key:                  accessKeyName,
			}
			dst.Spec.ObjectStore.ExternalObjectStore.SecretKey = corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Key:                  secretKeyName,
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

// mysqlFields maps each v1 global.mysql.<key> to a *MysqlConnection setter.
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

// mapMySQL routes valueFrom-shaped fields to externalMysql.*, scalars to
// the mysql-pending annotation, and the legacy passwordSecret block to
// externalMysql.password (skipped when password.valueFrom already won).
func mapMySQL(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	mysqlMap, found, err := unstructured.NestedMap(globalMap, "mysql")
	if err != nil {
		return fmt.Errorf("spec.values.global.mysql: %w", err)
	}
	if !found || len(mysqlMap) == 0 {
		return nil
	}

	var conn = &appsv2.MysqlConnection{}

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
			f.setRef(conn, *ref)
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
			conn.Password = corev1.SecretKeySelector{
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

// redisFields maps each v1 global.redis.<key> to a *RedisConnection setter.
var redisFields = []struct {
	v1Key  string
	setRef func(*appsv2.RedisConnection, corev1.SecretKeySelector)
}{
	{"host", func(c *appsv2.RedisConnection, s corev1.SecretKeySelector) { c.Host = s }},
	{"port", func(c *appsv2.RedisConnection, s corev1.SecretKeySelector) { c.Port = s }},
	{"password", func(c *appsv2.RedisConnection, s corev1.SecretKeySelector) { c.Password = s }},
	{"caCert", func(c *appsv2.RedisConnection, s corev1.SecretKeySelector) { c.SslCa = s }},
}

// mapRedis routes valueFrom-shaped fields to externalRedis.*, scalars to
// the redis-pending annotation, and the legacy secret block to
// externalRedis.password (skipped when password.valueFrom already won).
// Unknown fields (external, parameters, params) have no v2 home and are
// dropped.
func mapRedis(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	redisMap, found, err := unstructured.NestedMap(globalMap, "redis")
	if err != nil {
		return fmt.Errorf("spec.values.global.redis: %w", err)
	}
	if !found || len(redisMap) == 0 {
		return nil
	}

	var conn = &appsv2.RedisConnection{}

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
			f.setRef(conn, *ref)
		case literal != nil:
			remaining[f.v1Key] = literal
		}
	}

	// tls is nested under redis.params (preferred) or redis.parameters in v1.
	for _, parent := range []string{"params", "parameters"} {
		raw, found, err := unstructured.NestedFieldNoCopy(redisMap, parent, "tls")
		if err != nil {
			return fmt.Errorf("spec.values.global.redis.%s.tls: %w", parent, err)
		}
		if !found {
			continue
		}
		ref, literal, classifyErr := classifyValueFromOrLiteral(raw)
		if classifyErr != nil {
			return fmt.Errorf("spec.values.global.redis.%s.tls: %w", parent, classifyErr)
		}
		if ref != nil {
			conn.Tls = *ref
			break
		}
		if literal != nil {
			remaining["tls"] = literal
			break
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
			conn.Password = corev1.SecretKeySelector{
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

// oidcFields maps each v1 global.auth.oidc.<key> to an *OidcSpec setter.
// v1's `secret` is the client secret; `issuer` is the issuer URL.
var oidcFields = []struct {
	v1Key  string
	setRef func(*appsv2.OidcSpec, corev1.SecretKeySelector)
}{
	{"clientId", func(o *appsv2.OidcSpec, s corev1.SecretKeySelector) { o.ClientId = s }},
	{"secret", func(o *appsv2.OidcSpec, s corev1.SecretKeySelector) { o.ClientSecret = s }},
	{"authMethod", func(o *appsv2.OidcSpec, s corev1.SecretKeySelector) { o.AuthMethod = s }},
	{"issuer", func(o *appsv2.OidcSpec, s corev1.SecretKeySelector) { o.IssuerUrl = s }},
}

// mapOIDC routes valueFrom-shaped fields to spec.wandb.oidc.*, scalars to
// the oidc-pending annotation, and the legacy oidcSecret block to
// oidc.clientSecret (skipped when secret.valueFrom already won).
func mapOIDC(globalMap map[string]interface{}, dst *appsv2.WeightsAndBiases) error {
	oidcMap, found, err := unstructured.NestedMap(globalMap, "auth", "oidc")
	if err != nil {
		return fmt.Errorf("spec.values.global.auth.oidc: %w", err)
	}
	if !found || len(oidcMap) == 0 {
		return nil
	}

	oidc := &dst.Spec.Wandb.OIDC
	remaining := map[string]interface{}{}

	for _, f := range oidcFields {
		raw, ok := oidcMap[f.v1Key]
		if !ok {
			continue
		}
		ref, literal, classifyErr := classifyValueFromOrLiteral(raw)
		if classifyErr != nil {
			return fmt.Errorf("spec.values.global.auth.oidc.%s: %w", f.v1Key, classifyErr)
		}
		switch {
		case ref != nil:
			f.setRef(oidc, *ref)
		case literal != nil:
			remaining[f.v1Key] = literal
		}
	}

	if os, ok, err := unstructured.NestedMap(oidcMap, "oidcSecret"); err != nil {
		return fmt.Errorf("spec.values.global.auth.oidc.oidcSecret: %w", err)
	} else if ok {
		name, _, _ := unstructured.NestedString(os, "name")
		alreadyHasClientSecret := oidc.ClientSecret.Name != ""
		if name != "" && !alreadyHasClientSecret {
			key, _, _ := unstructured.NestedString(os, "secretKey")
			if key == "" {
				key = defaultOIDCClientSecretKey
			}
			oidc.ClientSecret = corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Key:                  key,
			}
			delete(remaining, "secret")
		}
	}

	if len(remaining) > 0 {
		return writeAnnotation(dst, OIDCPendingAnnotation, remaining)
	}
	return nil
}

// classifyValueFromOrLiteral returns a SecretKeySelector when raw is
// {valueFrom: {secretKeyRef: {name, key}}}, otherwise the non-empty scalar
// as a literal, otherwise (nil, nil). Malformed ref maps error out.
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
		// numbers, bools, etc. — reconciler stringifies as needed.
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
