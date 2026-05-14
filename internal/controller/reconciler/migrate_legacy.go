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
	if !mysqlChanged && !redisChanged {
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

	if len(data) > 0 {
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
			return false, fmt.Errorf("create or update %s: %w", secretName, err)
		}
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

	if len(data) > 0 {
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
			return false, fmt.Errorf("create or update %s: %w", secretName, err)
		}
	}

	wandb.Spec.Redis.ExternalRedis = conn
	delete(wandb.Annotations, apiv1.RedisPendingAnnotation)
	return true, nil
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
