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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wandb/operator/api/v1"
	apiv2 "github.com/wandb/operator/api/v2"
)

func newMigrationFixture(
	t *testing.T,
	annotations map[string]string,
	mutate func(*apiv2.WeightsAndBiases),
	seed ...ctrlClient.Object,
) (ctrlClient.Client, *apiv2.WeightsAndBiases) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "wandb",
			Namespace:   "default",
			Annotations: annotations,
		},
	}
	if mutate != nil {
		mutate(wandb)
	}
	objects := append([]ctrlClient.Object{wandb}, seed...)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	return client, wandb
}

func getConvertedSecret(t *testing.T, c ctrlClient.Client) (*corev1.Secret, error) {
	t.Helper()
	var secret corev1.Secret
	err := c.Get(context.Background(), types.NamespacedName{Name: "wandb-mysql-converted", Namespace: "default"}, &secret)
	return &secret, err
}

func getRedisConvertedSecret(t *testing.T, c ctrlClient.Client) (*corev1.Secret, error) {
	t.Helper()
	var secret corev1.Secret
	err := c.Get(context.Background(), types.NamespacedName{Name: "wandb-redis-converted", Namespace: "default"}, &secret)
	return &secret, err
}

func getBucketConvertedSecret(t *testing.T, c ctrlClient.Client) (*corev1.Secret, error) {
	t.Helper()
	var secret corev1.Secret
	err := c.Get(context.Background(), types.NamespacedName{Name: "wandb-bucket-converted", Namespace: "default"}, &secret)
	return &secret, err
}

func getOIDCConvertedSecret(t *testing.T, c ctrlClient.Client) (*corev1.Secret, error) {
	t.Helper()
	var secret corev1.Secret
	err := c.Get(context.Background(), types.NamespacedName{Name: "wandb-oidc-converted", Namespace: "default"}, &secret)
	return &secret, err
}

func TestMigrateLegacyAnnotations_NoAnnotation(t *testing.T) {
	client, wandb := newMigrationFixture(t, nil, nil)
	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.Zero(t, res.RequeueAfter)
	require.Nil(t, wandb.Spec.MySQL.ExternalMysql)

	_, err = getConvertedSecret(t, client)
	require.True(t, apiErrors.IsNotFound(err), "expected no converted Secret, got err=%v", err)
}

func TestMigrateLegacyMySQL_FullLiteralPayload(t *testing.T) {
	payload := `{"host":"mysql.example.com","port":3306,"database":"wandb_local","user":"wandb","password":"shh","caCert":"---cert---"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: payload,
	}, nil)

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	secret, err := getConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, corev1.SecretTypeOpaque, secret.Type)
	require.Equal(t, []byte("mysql.example.com"), secret.Data["host"])
	require.Equal(t, []byte("3306"), secret.Data["port"])
	require.Equal(t, []byte("wandb_local"), secret.Data["database"])
	require.Equal(t, []byte("wandb"), secret.Data["username"])
	require.Equal(t, []byte("shh"), secret.Data["password"])
	require.Equal(t, []byte("---cert---"), secret.Data["sslCa"])

	var fresh apiv2.WeightsAndBiases
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: "wandb", Namespace: "default"}, &fresh))
	require.NotContains(t, fresh.Annotations, apiv1.MySQLPendingAnnotation)
	require.NotNil(t, fresh.Spec.MySQL.ExternalMysql)
	conn := fresh.Spec.MySQL.ExternalMysql
	require.Equal(t, "wandb-mysql-converted", conn.Host.Name)
	require.Equal(t, "host", conn.Host.Key)
	require.Equal(t, "port", conn.Port.Key)
	require.Equal(t, "database", conn.Database.Key)
	require.Equal(t, "username", conn.Username.Key)
	require.Equal(t, "password", conn.Password.Key)
	require.Equal(t, "sslCa", conn.SslCa.Key)
	require.Empty(t, conn.Tls.Name)
	require.Empty(t, conn.SslCert.Name)
	require.Empty(t, conn.SslKey.Name)
	require.Empty(t, conn.URL.Name)
}

func TestMigrateLegacyMySQL_PartialPayload(t *testing.T) {
	payload := `{"host":"mysql.example.com","password":"shh"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: payload,
	}, nil)

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	secret, err := getConvertedSecret(t, client)
	require.NoError(t, err)
	require.Contains(t, secret.Data, "host")
	require.Contains(t, secret.Data, "password")
	require.NotContains(t, secret.Data, "port")
	require.NotContains(t, secret.Data, "database")
	require.NotContains(t, secret.Data, "username")
	require.NotContains(t, secret.Data, "sslCa")

	conn := wandb.Spec.MySQL.ExternalMysql
	require.NotNil(t, conn)
	require.Equal(t, "host", conn.Host.Key)
	require.Equal(t, "password", conn.Password.Key)
	require.Empty(t, conn.Port.Name)
	require.Empty(t, conn.Database.Name)
	require.Empty(t, conn.Username.Name)
	require.Empty(t, conn.SslCa.Name)
}

func TestMigrateLegacyMySQL_PreSetFieldsAreRespected(t *testing.T) {
	payload := `{"host":"mysql.example.com","port":3306,"database":"wandb_local"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: payload,
	}, func(w *apiv2.WeightsAndBiases) {
		w.Spec.MySQL.ExternalMysql = &apiv2.MysqlConnection{
			Host: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "preset-secret"},
				Key:                  "preset-host-key",
			},
		}
	})

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	secret, err := getConvertedSecret(t, client)
	require.NoError(t, err)
	require.NotContains(t, secret.Data, "host", "host was already set by the webhook; reconciler must not overwrite")
	require.Contains(t, secret.Data, "port")
	require.Contains(t, secret.Data, "database")

	conn := wandb.Spec.MySQL.ExternalMysql
	require.Equal(t, "preset-secret", conn.Host.Name)
	require.Equal(t, "preset-host-key", conn.Host.Key)
	require.Equal(t, "wandb-mysql-converted", conn.Port.Name)
	require.Equal(t, "wandb-mysql-converted", conn.Database.Name)
}

func TestMigrateLegacyMySQL_AllPreSetEmptyAnnotationPayload(t *testing.T) {
	payload := `{}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: payload,
	}, func(w *apiv2.WeightsAndBiases) {
		w.Spec.MySQL.ExternalMysql = &apiv2.MysqlConnection{
			Host: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "preset"},
				Key:                  "host",
			},
		}
	})

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter, "annotation removal alone still constitutes a change")

	_, err = getConvertedSecret(t, client)
	require.True(t, apiErrors.IsNotFound(err), "no literal data, so no converted Secret should be created")

	var fresh apiv2.WeightsAndBiases
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: "wandb", Namespace: "default"}, &fresh))
	require.NotContains(t, fresh.Annotations, apiv1.MySQLPendingAnnotation)
	require.Equal(t, "preset", fresh.Spec.MySQL.ExternalMysql.Host.Name)
}

func TestMigrateLegacyMySQL_PreExistingSecretOverwritten(t *testing.T) {
	stale := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-mysql-converted", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"host":     []byte("OLD"),
			"obsolete": []byte("OLD"),
		},
	}
	payload := `{"host":"mysql.example.com","password":"shh"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: payload,
	}, nil, stale)

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	secret, err := getConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("mysql.example.com"), secret.Data["host"])
	require.NotContains(t, secret.Data, "obsolete")
}

func TestMigrateLegacyMySQL_MalformedJSON(t *testing.T) {
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: "{not json",
	}, nil)
	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.Error(t, err)

	require.Contains(t, wandb.Annotations, apiv1.MySQLPendingAnnotation)
	require.Nil(t, wandb.Spec.MySQL.ExternalMysql)
}

func TestMigrateLegacyMySQL_EmptyAnnotation(t *testing.T) {
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: "",
	}, nil)
	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.Error(t, err)
}

func TestMigrateLegacyMySQL_PortStringified(t *testing.T) {
	payload := `{"host":"mysql.example.com","port":3307}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: payload,
	}, nil)

	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("3307"), secret.Data["port"])
}

func TestMigrateLegacyRedis_FullLiteralPayload(t *testing.T) {
	payload := `{"host":"redis.example.com","port":6379,"password":"shh","caCert":"---cert---"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.RedisPendingAnnotation: payload,
	}, nil)

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	secret, err := getRedisConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, corev1.SecretTypeOpaque, secret.Type)
	require.Equal(t, []byte("redis.example.com"), secret.Data["host"])
	require.Equal(t, []byte("6379"), secret.Data["port"])
	require.Equal(t, []byte("shh"), secret.Data["password"])
	require.Equal(t, []byte("---cert---"), secret.Data["sslCa"])

	var fresh apiv2.WeightsAndBiases
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: "wandb", Namespace: "default"}, &fresh))
	require.NotContains(t, fresh.Annotations, apiv1.RedisPendingAnnotation)
	conn := fresh.Spec.Redis.ExternalRedis
	require.NotNil(t, conn)
	require.Equal(t, "wandb-redis-converted", conn.Host.Name)
	require.Equal(t, "host", conn.Host.Key)
	require.Equal(t, "port", conn.Port.Key)
	require.Equal(t, "password", conn.Password.Key)
	require.Equal(t, "sslCa", conn.SslCa.Key)
	require.Empty(t, conn.Tls.Name)
	require.Empty(t, conn.URL.Name)
}

func TestMigrateLegacyRedis_PreSetFieldsAreRespected(t *testing.T) {
	payload := `{"host":"redis.example.com","port":6379}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.RedisPendingAnnotation: payload,
	}, func(w *apiv2.WeightsAndBiases) {
		w.Spec.Redis.ExternalRedis = &apiv2.RedisConnection{
			Host: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "preset-secret"},
				Key:                  "preset-host-key",
			},
		}
	})

	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getRedisConvertedSecret(t, client)
	require.NoError(t, err)
	require.NotContains(t, secret.Data, "host")
	require.Contains(t, secret.Data, "port")

	conn := wandb.Spec.Redis.ExternalRedis
	require.Equal(t, "preset-secret", conn.Host.Name)
	require.Equal(t, "preset-host-key", conn.Host.Key)
	require.Equal(t, "wandb-redis-converted", conn.Port.Name)
}

func TestMigrateLegacyRedis_MalformedJSON(t *testing.T) {
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.RedisPendingAnnotation: "{not json",
	}, nil)
	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.Error(t, err)

	require.Contains(t, wandb.Annotations, apiv1.RedisPendingAnnotation)
	require.Nil(t, wandb.Spec.Redis.ExternalRedis)
}

func TestMigrateLegacyAnnotations_MySQLAndRedisInOneCall(t *testing.T) {
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: `{"host":"mysql.example.com"}`,
		apiv1.RedisPendingAnnotation: `{"host":"redis.example.com"}`,
	}, nil)

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	var fresh apiv2.WeightsAndBiases
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: "wandb", Namespace: "default"}, &fresh))
	require.NotContains(t, fresh.Annotations, apiv1.MySQLPendingAnnotation)
	require.NotContains(t, fresh.Annotations, apiv1.RedisPendingAnnotation)
	require.NotNil(t, fresh.Spec.MySQL.ExternalMysql)
	require.NotNil(t, fresh.Spec.Redis.ExternalRedis)

	mysqlSecret, err := getConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("mysql.example.com"), mysqlSecret.Data["host"])

	redisSecret, err := getRedisConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("redis.example.com"), redisSecret.Data["host"])
}

func TestMigrateLegacyMySQL_PortStringValueAccepted(t *testing.T) {
	payload := `{"host":"mysql.example.com","port":"3308"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.MySQLPendingAnnotation: payload,
	}, nil)

	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("3308"), secret.Data["port"])
}

func TestParseBucketName(t *testing.T) {
	cases := []struct {
		name                string
		endpoint, port, bkt string
	}{
		{"", "", "", ""},
		{"my-bucket", "", "", "my-bucket"},
		{"minio.example.com/wandb", "minio.example.com", "", "wandb"},
		{"minio.example.com:9000/wandb", "minio.example.com", "9000", "wandb"},
		{"minio:9000/wandb", "minio", "9000", "wandb"},
		{"minio.minio.svc.cluster.local:9000/bucket", "minio.minio.svc.cluster.local", "9000", "bucket"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, p, b := parseBucketName(tc.name)
			require.Equal(t, tc.endpoint, e)
			require.Equal(t, tc.port, p)
			require.Equal(t, tc.bkt, b)
		})
	}
}

func TestMigrateLegacyBucket_BareBucketName(t *testing.T) {
	payload := `{"name":"my-bucket","region":"us-east-1","accessKey":"AKIA","secretKey":"shh"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.BucketPendingAnnotation: payload,
	}, nil)

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	secret, err := getBucketConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("my-bucket"), secret.Data["bucket"])
	require.Equal(t, []byte("us-east-1"), secret.Data["region"])
	require.Equal(t, []byte("AKIA"), secret.Data["accessKey"])
	require.Equal(t, []byte("shh"), secret.Data["secretKey"])
	require.NotContains(t, secret.Data, "endpoint")
	require.NotContains(t, secret.Data, "port")

	conn := wandb.Spec.ObjectStore.ExternalObjectStore
	require.NotNil(t, conn)
	require.Equal(t, "bucket", conn.Bucket.Key)
	require.Equal(t, "region", conn.Region.Key)
	require.Equal(t, "accessKey", conn.AccessKey.Key)
	require.Equal(t, "secretKey", conn.SecretKey.Key)
	require.Empty(t, conn.Endpoint.Name)
	require.Empty(t, conn.Port.Name)
}

func TestMigrateLegacyBucket_EmbeddedEndpoint(t *testing.T) {
	payload := `{"name":"minio.minio.svc:9000/wandb-bucket"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.BucketPendingAnnotation: payload,
	}, nil)

	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getBucketConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("minio.minio.svc"), secret.Data["endpoint"])
	require.Equal(t, []byte("9000"), secret.Data["port"])
	require.Equal(t, []byte("wandb-bucket"), secret.Data["bucket"])

	conn := wandb.Spec.ObjectStore.ExternalObjectStore
	require.Equal(t, "endpoint", conn.Endpoint.Key)
	require.Equal(t, "port", conn.Port.Key)
	require.Equal(t, "bucket", conn.Bucket.Key)
}

func TestMigrateLegacyBucket_PreSetCredentialsRespected(t *testing.T) {
	payload := `{"name":"my-bucket","accessKey":"FROM_ANNOTATION","secretKey":"FROM_ANNOTATION"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.BucketPendingAnnotation: payload,
	}, func(w *apiv2.WeightsAndBiases) {
		w.Spec.ObjectStore.ExternalObjectStore = &apiv2.ObjectStoreConnection{
			AccessKey: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "preset"},
				Key:                  "ACCESS_KEY",
			},
			SecretKey: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "preset"},
				Key:                  "SECRET_KEY",
			},
		}
	})

	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getBucketConvertedSecret(t, client)
	require.NoError(t, err)
	require.NotContains(t, secret.Data, "accessKey", "webhook-set AccessKey must not be overwritten")
	require.NotContains(t, secret.Data, "secretKey", "webhook-set SecretKey must not be overwritten")
	require.Contains(t, secret.Data, "bucket")

	conn := wandb.Spec.ObjectStore.ExternalObjectStore
	require.Equal(t, "preset", conn.AccessKey.Name)
	require.Equal(t, "preset", conn.SecretKey.Name)
	require.Equal(t, "wandb-bucket-converted", conn.Bucket.Name)
}

func TestMigrateLegacyBucket_UnknownFieldsIgnored(t *testing.T) {
	payload := `{"name":"my-bucket","provider":"s3","path":"sub/path","kmsKey":"arn:aws:kms:..."}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.BucketPendingAnnotation: payload,
	}, nil)

	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err, "unknown fields should be tolerated and dropped")

	secret, err := getBucketConvertedSecret(t, client)
	require.NoError(t, err)
	require.NotContains(t, secret.Data, "provider")
	require.NotContains(t, secret.Data, "path")
	require.NotContains(t, secret.Data, "kmsKey")
}

func TestMigrateLegacyBucket_MalformedJSON(t *testing.T) {
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.BucketPendingAnnotation: "{not json",
	}, nil)
	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.Error(t, err)
	require.Contains(t, wandb.Annotations, apiv1.BucketPendingAnnotation)
	require.Nil(t, wandb.Spec.ObjectStore.ExternalObjectStore)
}

func TestMigrateLegacyOIDC_AllLiterals(t *testing.T) {
	payload := `{"clientId":"abc","secret":"shh","authMethod":"client_secret_post","issuer":"https://idp.example.com"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.OIDCPendingAnnotation: payload,
	}, nil)

	res, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)
	require.NotZero(t, res.RequeueAfter)

	secret, err := getOIDCConvertedSecret(t, client)
	require.NoError(t, err)
	require.Equal(t, []byte("abc"), secret.Data["clientId"])
	require.Equal(t, []byte("shh"), secret.Data["clientSecret"])
	require.Equal(t, []byte("client_secret_post"), secret.Data["authMethod"])
	require.Equal(t, []byte("https://idp.example.com"), secret.Data["issuerUrl"])

	oidc := wandb.Spec.Wandb.OIDC
	require.Equal(t, "wandb-oidc-converted", oidc.ClientId.Name)
	require.Equal(t, "clientId", oidc.ClientId.Key)
	require.Equal(t, "clientSecret", oidc.ClientSecret.Key)
	require.Equal(t, "authMethod", oidc.AuthMethod.Key)
	require.Equal(t, "issuerUrl", oidc.IssuerUrl.Key)

	var fresh apiv2.WeightsAndBiases
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: "wandb", Namespace: "default"}, &fresh))
	require.NotContains(t, fresh.Annotations, apiv1.OIDCPendingAnnotation)
}

func TestMigrateLegacyOIDC_PreSetClientSecretRespected(t *testing.T) {
	payload := `{"clientId":"abc","secret":"shh"}`
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.OIDCPendingAnnotation: payload,
	}, func(w *apiv2.WeightsAndBiases) {
		w.Spec.Wandb.OIDC.ClientSecret = corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "preset-oidc"},
			Key:                  "PRESET",
		}
	})

	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.NoError(t, err)

	secret, err := getOIDCConvertedSecret(t, client)
	require.NoError(t, err)
	require.Contains(t, secret.Data, "clientId")
	require.NotContains(t, secret.Data, "clientSecret")

	oidc := wandb.Spec.Wandb.OIDC
	require.Equal(t, "preset-oidc", oidc.ClientSecret.Name)
	require.Equal(t, "PRESET", oidc.ClientSecret.Key)
}

func TestMigrateLegacyOIDC_MalformedJSON(t *testing.T) {
	client, wandb := newMigrationFixture(t, map[string]string{
		apiv1.OIDCPendingAnnotation: "{not json",
	}, nil)
	_, err := migrateLegacyAnnotations(context.Background(), client, wandb)
	require.Error(t, err)
	require.Contains(t, wandb.Annotations, apiv1.OIDCPendingAnnotation)
}
