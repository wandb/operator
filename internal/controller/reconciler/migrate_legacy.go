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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wandb/operator/api/v1"
	apiv2 "github.com/wandb/operator/api/v2"
)

// migrateLegacyAnnotations consumes `legacy.operator.wandb.com/*-pending`
// annotations placed by the v1→v2 conversion webhook, materializes the Secret
// each annotation describes, and updates the v2 spec to reference it.
//
// Returns (Result{RequeueAfter}, nil) when a migration was applied so the
// next reconcile pass sees the updated spec. Returns (Result{}, nil) when
// there is nothing to do.
func migrateLegacyAnnotations(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) (ctrl.Result, error) {
	mysqlChanged, err := migrateLegacyMySQL(ctx, c, wandb)
	if err != nil {
		return ctrl.Result{}, err
	}
	redisChanged, err := migrateLegacyRedis(ctx, c, wandb)
	if err != nil {
		return ctrl.Result{}, err
	}
	bucketChanged, err := migrateLegacyBucket(ctx, c, wandb)
	if err != nil {
		return ctrl.Result{}, err
	}
	oidcChanged, err := migrateLegacyOIDC(ctx, c, wandb)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !mysqlChanged && !redisChanged && !bucketChanged && !oidcChanged {
		return ctrl.Result{}, nil
	}

	if err := c.Update(ctx, wandb); err != nil {
		return ctrl.Result{}, fmt.Errorf("update CR after legacy migration: %w", err)
	}
	return ctrl.Result{RequeueAfter: time.Second}, nil
}

// legacyMySQLPayload carries the *literal-string* fields from v1
// global.mysql that the conversion webhook could not turn into typed
// SecretKeySelectors on its own (the webhook already wrote any
// {valueFrom: {secretKeyRef}} and legacy passwordSecret refs directly).
// Port is `any` because v1 permitted either a JSON number or a string.
type legacyMySQLPayload struct {
	Host     string `json:"host,omitempty"`
	Port     any    `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	CaCert   string `json:"caCert,omitempty"`
}

// migrateLegacyMySQL materializes the mysql-pending annotation. Returns
// (true, nil) when it mutated wandb (caller must persist), (false, nil) when
// the annotation was absent. Any externalMysql field already populated by the
// conversion webhook is left alone — only fields with a zero selector are
// filled from the annotation.
func migrateLegacyMySQL(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) (bool, error) {
	raw, ok := wandb.Annotations[apiv1.MySQLPendingAnnotation]
	if !ok {
		return false, nil
	}

	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var payload legacyMySQLPayload
	if err := dec.Decode(&payload); err != nil {
		return false, fmt.Errorf("decode %s: %w", apiv1.MySQLPendingAnnotation, err)
	}

	secretName := fmt.Sprintf("%s-mysql-converted", wandb.Name)
	conn := wandb.Spec.MySQL.ExternalMysql
	if conn == nil {
		conn = &apiv2.MysqlConnection{}
	}

	data := map[string][]byte{}
	fill := func(target *corev1.SecretKeySelector, dataKey, value string) {
		if target.Name != "" || value == "" {
			return
		}
		data[dataKey] = []byte(value)
		*target = secretSelector(secretName, dataKey)
	}

	fill(&conn.Host, "host", payload.Host)
	fill(&conn.Port, "port", normalizePort(payload.Port))
	fill(&conn.Database, "database", payload.Database)
	fill(&conn.Username, "username", payload.User)
	fill(&conn.SslCa, "sslCa", payload.CaCert)
	fill(&conn.Password, "password", payload.Password)

	if err := materializeConvertedSecret(ctx, c, wandb, secretName, data); err != nil {
		return false, err
	}

	wandb.Spec.MySQL.ExternalMysql = conn
	delete(wandb.Annotations, apiv1.MySQLPendingAnnotation)
	return true, nil
}

// legacyRedisPayload carries the literal-string fields from v1 global.redis
// that the conversion webhook could not turn into typed SecretKeySelectors on
// its own.
type legacyRedisPayload struct {
	Host     string `json:"host,omitempty"`
	Port     any    `json:"port,omitempty"`
	Password string `json:"password,omitempty"`
	CaCert   string `json:"caCert,omitempty"`
}

// migrateLegacyRedis materializes the redis-pending annotation. Mirrors
// migrateLegacyMySQL: only fills externalRedis fields whose selector is
// currently zero, so anything the webhook already populated is left alone.
func migrateLegacyRedis(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) (bool, error) {
	raw, ok := wandb.Annotations[apiv1.RedisPendingAnnotation]
	if !ok {
		return false, nil
	}

	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var payload legacyRedisPayload
	if err := dec.Decode(&payload); err != nil {
		return false, fmt.Errorf("decode %s: %w", apiv1.RedisPendingAnnotation, err)
	}

	secretName := fmt.Sprintf("%s-redis-converted", wandb.Name)
	conn := wandb.Spec.Redis.ExternalRedis
	if conn == nil {
		conn = &apiv2.RedisConnection{}
	}

	data := map[string][]byte{}
	fill := func(target *corev1.SecretKeySelector, dataKey, value string) {
		if target.Name != "" || value == "" {
			return
		}
		data[dataKey] = []byte(value)
		*target = secretSelector(secretName, dataKey)
	}

	fill(&conn.Host, "host", payload.Host)
	fill(&conn.Port, "port", normalizePort(payload.Port))
	fill(&conn.Password, "password", payload.Password)
	fill(&conn.SslCa, "sslCa", payload.CaCert)

	if err := materializeConvertedSecret(ctx, c, wandb, secretName, data); err != nil {
		return false, err
	}

	wandb.Spec.Redis.ExternalRedis = conn
	delete(wandb.Annotations, apiv1.RedisPendingAnnotation)
	return true, nil
}

// legacyBucketPayload carries the flat literal fields produced by the
// conversion webhook's bucket merge (bucket+defaultBucket minus the secret
// block). Fields with no v2 equivalent (provider, path, kmsKey) are tolerated
// but ignored.
type legacyBucketPayload struct {
	Name      string `json:"name,omitempty"`
	Region    string `json:"region,omitempty"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
}

// migrateLegacyBucket materializes the bucket-pending annotation. Any
// externalObjectStore field already populated by the conversion webhook
// (typically AccessKey / SecretKey from a `bucket.secret` block) is left
// alone — only fields with a zero selector are filled from the annotation.
func migrateLegacyBucket(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) (bool, error) {
	raw, ok := wandb.Annotations[apiv1.BucketPendingAnnotation]
	if !ok {
		return false, nil
	}

	var payload legacyBucketPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false, fmt.Errorf("decode %s: %w", apiv1.BucketPendingAnnotation, err)
	}

	secretName := fmt.Sprintf("%s-bucket-converted", wandb.Name)
	conn := wandb.Spec.ObjectStore.ExternalObjectStore
	if conn == nil {
		conn = &apiv2.ObjectStoreConnection{}
	}

	endpoint, port, bucket := parseBucketName(payload.Name)

	data := map[string][]byte{}
	fill := func(target *corev1.SecretKeySelector, dataKey, value string) {
		if target.Name != "" || value == "" {
			return
		}
		data[dataKey] = []byte(value)
		*target = secretSelector(secretName, dataKey)
	}

	fill(&conn.Endpoint, "endpoint", endpoint)
	fill(&conn.Port, "port", port)
	fill(&conn.Bucket, "bucket", bucket)
	fill(&conn.Region, "region", payload.Region)
	fill(&conn.AccessKey, "accessKey", payload.AccessKey)
	fill(&conn.SecretKey, "secretKey", payload.SecretKey)

	if err := materializeConvertedSecret(ctx, c, wandb, secretName, data); err != nil {
		return false, err
	}

	wandb.Spec.ObjectStore.ExternalObjectStore = conn
	delete(wandb.Annotations, apiv1.BucketPendingAnnotation)
	return true, nil
}

// parseBucketName splits v1's `bucket.name` field. v1 sometimes embedded the
// endpoint as "host[:port]/bucket-name". S3 bucket names cannot contain a
// "/", so the presence of one unambiguously signals the embedded form. A bare
// name (no slash) returns it as bucket only.
func parseBucketName(name string) (endpoint, port, bucket string) {
	if name == "" || !strings.Contains(name, "/") {
		return "", "", name
	}
	slash := strings.IndexByte(name, '/')
	host := name[:slash]
	bucket = name[slash+1:]
	if colon := strings.IndexByte(host, ':'); colon >= 0 {
		return host[:colon], host[colon+1:], bucket
	}
	return host, "", bucket
}

// legacyOIDCPayload carries the literal-string fields from v1
// global.auth.oidc that the conversion webhook could not turn into typed
// SecretKeySelectors on its own (the webhook already wrote any
// {valueFrom: {secretKeyRef}} and legacy oidcSecret refs directly).
type legacyOIDCPayload struct {
	ClientId   string `json:"clientId,omitempty"`
	Secret     string `json:"secret,omitempty"`
	AuthMethod string `json:"authMethod,omitempty"`
	Issuer     string `json:"issuer,omitempty"`
}

// migrateLegacyOIDC materializes the oidc-pending annotation. Same merge
// semantics as the others: only fills wandb.Spec.Wandb.OIDC selectors that
// are still zero.
func migrateLegacyOIDC(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) (bool, error) {
	raw, ok := wandb.Annotations[apiv1.OIDCPendingAnnotation]
	if !ok {
		return false, nil
	}

	var payload legacyOIDCPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false, fmt.Errorf("decode %s: %w", apiv1.OIDCPendingAnnotation, err)
	}

	secretName := fmt.Sprintf("%s-oidc-converted", wandb.Name)
	oidc := &wandb.Spec.Wandb.OIDC

	data := map[string][]byte{}
	fill := func(target *corev1.SecretKeySelector, dataKey, value string) {
		if target.Name != "" || value == "" {
			return
		}
		data[dataKey] = []byte(value)
		*target = secretSelector(secretName, dataKey)
	}

	fill(&oidc.ClientId, "clientId", payload.ClientId)
	fill(&oidc.ClientSecret, "clientSecret", payload.Secret)
	fill(&oidc.AuthMethod, "authMethod", payload.AuthMethod)
	fill(&oidc.IssuerUrl, "issuerUrl", payload.Issuer)

	if err := materializeConvertedSecret(ctx, c, wandb, secretName, data); err != nil {
		return false, err
	}

	delete(wandb.Annotations, apiv1.OIDCPendingAnnotation)
	return true, nil
}

// materializeConvertedSecret writes data as an opaque Secret in
// wandb.Namespace. No-op when data is empty. Uses CreateOrUpdate so retries
// (and reruns after partial migration) are safe.
func materializeConvertedSecret(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	secretName string,
	data map[string][]byte,
) error {
	if len(data) == 0 {
		return nil
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: wandb.Namespace,
		},
	}
	if _, err := ctrl.CreateOrUpdate(ctx, c, secret, func() error {
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = data
		return nil
	}); err != nil {
		return fmt.Errorf("create or update %s: %w", secretName, err)
	}
	return nil
}

func normalizePort(v any) string {
	switch p := v.(type) {
	case nil:
		return ""
	case string:
		return p
	case json.Number:
		if i, err := p.Int64(); err == nil {
			if i == 0 {
				return ""
			}
			return strconv.FormatInt(i, 10)
		}
		return string(p)
	default:
		return fmt.Sprintf("%v", p)
	}
}

func secretSelector(name, key string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  key,
	}
}
