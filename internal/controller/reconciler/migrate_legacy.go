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
	"net/url"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/wandb/operator/api/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/objectstore"
)

// migrateLegacyAnnotations drains `legacy.operator.wandb.com/*-pending`
// annotations into materialized Secrets and typed spec references. Returns
// a Requeue result when any change was applied.
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

// legacyMySQLPayload is the literal-string subset the webhook couldn't turn
// into typed selectors. Port is `any` to accept JSON number or string.
type legacyMySQLPayload struct {
	Host     string `json:"host,omitempty"`
	Port     any    `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	CaCert   string `json:"caCert,omitempty"`
}

// migrateLegacyMySQL drains the mysql-pending annotation into a Secret +
// externalMysql selectors. Fields the webhook already set are preserved;
// only zero selectors are filled. Returns (changed, err).
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
	conn := wandb.Spec.MySQL[apiv2.DefaultInstanceName].ExternalMysql
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

	setExternalInstance(&wandb.Spec.MySQL, func(s *apiv2.MySQLSpec) { s.ExternalMysql = conn })
	delete(wandb.Annotations, apiv1.MySQLPendingAnnotation)
	return true, nil
}

// legacyRedisPayload is the literal-string subset the webhook couldn't turn
// into typed selectors.
type legacyRedisPayload struct {
	Host     string `json:"host,omitempty"`
	Port     any    `json:"port,omitempty"`
	Password string `json:"password,omitempty"`
	CaCert   string `json:"caCert,omitempty"`
	Tls      string `json:"tls,omitempty"`
}

// migrateLegacyRedis drains the redis-pending annotation. Mirrors
// migrateLegacyMySQL — only zero externalRedis selectors are filled.
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
	conn := wandb.Spec.Redis[apiv2.DefaultInstanceName].ExternalRedis
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
	fill(&conn.Tls, "tls", payload.Tls)

	if err := materializeConvertedSecret(ctx, c, wandb, secretName, data); err != nil {
		return false, err
	}

	setExternalInstance(&wandb.Spec.Redis, func(s *apiv2.RedisSpec) { s.ExternalRedis = conn })
	delete(wandb.Annotations, apiv1.RedisPendingAnnotation)
	return true, nil
}

// legacyBucketPayload is the flat literal subset from the webhook's
// bucket+defaultBucket merge. kmsKey has no v2 home; ignored.
type legacyBucketPayload struct {
	Provider  string `json:"provider,omitempty"`
	Name      string `json:"name,omitempty"`
	Path      string `json:"path,omitempty"`
	Region    string `json:"region,omitempty"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
}

// migrateLegacyBucket drains the bucket-pending annotation. Webhook-set
// fields (typically AccessKey/SecretKey) are preserved.
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
	conn := wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ExternalObjectStore
	if conn == nil {
		conn = &apiv2.ObjectStoreConnection{}
	}

	name, path, query := splitBucketQuery(payload.Name, payload.Path)
	endpoint, port, bucket := parseBucketName(name)
	// Query param beats the region field, matching gorilla's precedence.
	region := payload.Region
	if v := query.Get("region"); v != "" {
		region = v
	}
	forcePathStyle, tlsEnabled := deriveBucketAddressing(payload.Provider, endpoint, query)

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
	fill(&conn.Path, "path", strings.Trim(path, "/"))
	fill(&conn.Region, "region", region)
	fill(&conn.AccessKey, "accessKey", payload.AccessKey)
	fill(&conn.SecretKey, "secretKey", payload.SecretKey)
	fill(&conn.ForcePathStyle, "forcePathStyle", forcePathStyle)
	fill(&conn.TlsEnabled, "tlsEnabled", tlsEnabled)

	if err := materializeConvertedSecret(ctx, c, wandb, secretName, data); err != nil {
		return false, err
	}

	setExternalInstance(&wandb.Spec.ObjectStore, func(s *apiv2.ObjectStoreSpec) { s.ExternalObjectStore = conn })
	delete(wandb.Annotations, apiv1.BucketPendingAnnotation)
	return true, nil
}

// setExternalInstance applies fn to the default instance of an infra map,
// creating the map and/or default entry when absent.
func setExternalInstance[T any](m *map[string]T, fn func(*T)) {
	if *m == nil {
		*m = map[string]T{}
	}
	instance := (*m)[apiv2.DefaultInstanceName]
	fn(&instance)
	(*m)[apiv2.DefaultInstanceName] = instance
}

// splitBucketQuery strips the ?tls=/?forcePathStyle=/?region= overrides gorilla
// accepted on v1 bucket URLs; they rode in bucket.name or bucket.path.
func splitBucketQuery(name, path string) (cleanName, cleanPath string, q url.Values) {
	raw := ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path, raw = path[:i], path[i+1:]
	}
	if i := strings.IndexByte(name, '?'); i >= 0 {
		name, raw = name[:i], name[i+1:]
	}
	query, err := url.ParseQuery(raw)
	if err != nil {
		return name, path, url.Values{}
	}
	return name, path, query
}

// deriveBucketAddressing decides forcePathStyle/tlsEnabled for a drained v1 bucket:
// explicit ?forcePathStyle=/?tls= win, else any embedded endpoint means path-style over
// http (prefixes belong in bucket.path, so a host in bucket.name is always an endpoint).
func deriveBucketAddressing(provider, endpoint string, query url.Values) (forcePathStyle, tlsEnabled string) {
	if provider != "" && provider != "s3" && provider != "cw" {
		return "", ""
	}
	fps := provider != "cw" && objectstore.RequiresPathStyle(endpoint)
	if v, err := strconv.ParseBool(query.Get("forcePathStyle")); err == nil {
		fps = v
	}
	forcePathStyle = strconv.FormatBool(fps)
	if endpoint == "" {
		return forcePathStyle, ""
	}
	// gorilla defaulted S3-compatible endpoints to http, CoreWeave to https.
	tls := provider == "cw"
	if v, err := strconv.ParseBool(query.Get("tls")); err == nil {
		tls = v
	}
	return forcePathStyle, strconv.FormatBool(tls)
}

// parseBucketName splits v1's bucket.name. A "/" indicates the embedded
// "host[:port]/bucket" form (S3 bucket names can't contain "/"); otherwise
// the whole string is the bucket name.
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

// legacyOIDCPayload is the literal-string subset the webhook couldn't turn
// into typed selectors.
type legacyOIDCPayload struct {
	ClientId   string `json:"clientId,omitempty"`
	Secret     string `json:"secret,omitempty"`
	AuthMethod string `json:"authMethod,omitempty"`
	Issuer     string `json:"issuer,omitempty"`
}

// migrateLegacyOIDC drains the oidc-pending annotation. Only zero
// spec.wandb.oidc selectors are filled.
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

// materializeConvertedSecret CreateOrUpdates an opaque Secret with data,
// no-op when empty. Safe to call on partial-migration retries.
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
