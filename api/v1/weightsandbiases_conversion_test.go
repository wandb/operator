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
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv2 "github.com/wandb/operator/api/v2"
)

func newV1(values map[string]interface{}) *WeightsAndBiases {
	return &WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
		},
		Spec: WeightsAndBiasesSpec{
			Values: Object{Object: values},
		},
	}
}

func TestConvertTo_EmptyValues(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	require.NoError(t, newV1(nil).ConvertTo(dst))
	require.Equal(t, "wandb", dst.Name)
	require.Empty(t, dst.Spec.Wandb.Hostname)
	require.Empty(t, dst.Spec.Wandb.License)
	require.Empty(t, string(dst.Spec.Size))
	require.NotContains(t, dst.Annotations, OIDCPendingAnnotation)
}

func TestConvertTo_NoGlobalKey(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"mysql": map[string]interface{}{"install": true},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, dst.Spec.Wandb.Hostname)
	require.Empty(t, string(dst.Spec.Size))
}

func TestConvertTo_HostnameAndLicense(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"host":    "http://wandb.localhost",
			"license": "jwt-token-here",
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "http://wandb.localhost", dst.Spec.Wandb.Hostname)
	require.Equal(t, "jwt-token-here", dst.Spec.Wandb.License)
	require.NotContains(t, dst.Annotations, OIDCPendingAnnotation)
}

func TestConvertTo_AllValidSizes(t *testing.T) {
	cases := []appsv2.Size{
		appsv2.SizeDev,
		appsv2.SizeMicro,
		appsv2.SizeSmall,
		appsv2.SizeMedium,
		appsv2.SizeLarge,
		appsv2.SizeXLarge,
		appsv2.SizeXXLarge,
	}
	for _, size := range cases {
		t.Run(string(size), func(t *testing.T) {
			dst := &appsv2.WeightsAndBiases{}
			src := newV1(map[string]interface{}{
				"global": map[string]interface{}{"size": string(size)},
			})
			require.NoError(t, src.ConvertTo(dst))
			require.Equal(t, size, dst.Spec.Size)
		})
	}
}

func TestConvertTo_SizeEmptyString(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"size": ""},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.Empty(t, string(dst.Spec.Size))
}

func TestConvertTo_SizeUnrecognized(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"size": "testing"},
	})
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"testing"`)
}

func TestConvertTo_OIDCPopulated(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId":   "abc",
					"secret":     "shh",
					"authMethod": "client_secret_post",
					"issuer":     "https://example.com",
					"oidcSecret": map[string]interface{}{
						"name":      "oidc-secret",
						"secretKey": "OIDC_SECRET",
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	raw, ok := dst.Annotations[OIDCPendingAnnotation]
	require.True(t, ok, "expected oidc-pending annotation to be set")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "abc", decoded["clientId"])
	require.Equal(t, "shh", decoded["secret"])
	require.Equal(t, "client_secret_post", decoded["authMethod"])
	require.Equal(t, "https://example.com", decoded["issuer"])
	require.Contains(t, decoded, "oidcSecret")
}

func TestConvertTo_OIDCAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"host": "http://wandb.localhost",
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, OIDCPendingAnnotation)
}

func TestConvertTo_MySQLPopulated(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host":     "mysql.example.com",
				"port":     int64(3306),
				"database": "wandb_local",
				"user":     "wandb",
				"password": "shh",
				"passwordSecret": map[string]interface{}{
					"name":            "mysql-creds",
					"rootPasswordKey": "MYSQL_ROOT_PASSWORD",
					"passwordKey":     "MYSQL_PASSWORD",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	raw, ok := dst.Annotations[MySQLPendingAnnotation]
	require.True(t, ok, "expected mysql-pending annotation")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "mysql.example.com", decoded["host"])
	require.Equal(t, "wandb_local", decoded["database"])
	require.Equal(t, "shh", decoded["password"])
	require.Contains(t, decoded, "passwordSecret")
}

func TestConvertTo_MySQLAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, MySQLPendingAnnotation)
}

func TestConvertTo_MySQLEmptyMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, MySQLPendingAnnotation)
}

func TestConvertTo_RedisPopulated(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host":     "redis.example.com",
				"port":     int64(6379),
				"password": "shh",
				"external": true,
				"caCert":   "----BEGIN CERT----",
				"secret": map[string]interface{}{
					"secretName": "redis-creds",
					"secretKey":  "REDIS_PASSWORD",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	raw, ok := dst.Annotations[RedisPendingAnnotation]
	require.True(t, ok, "expected redis-pending annotation")

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "redis.example.com", decoded["host"])
	require.Equal(t, "shh", decoded["password"])
	require.Equal(t, true, decoded["external"])
	require.Contains(t, decoded, "secret")
}

func TestConvertTo_RedisAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, RedisPendingAnnotation)
}

func TestConvertTo_RedisEmptyMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, RedisPendingAnnotation)
}

func TestConvertTo_BucketOnly(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"provider":  "s3",
				"name":      "wandb-bucket",
				"region":    "us-east-1",
				"accessKey": "AKIA...",
				"secretKey": "secret",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	raw, ok := dst.Annotations[BucketPendingAnnotation]
	require.True(t, ok)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Contains(t, decoded, "bucket")
	require.NotContains(t, decoded, "defaultBucket")
	bucket := decoded["bucket"].(map[string]interface{})
	require.Equal(t, "s3", bucket["provider"])
	require.Equal(t, "wandb-bucket", bucket["name"])
}

func TestConvertTo_DefaultBucketOnly(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"defaultBucket": map[string]interface{}{
				"provider": "s3",
				"name":     "wandb-bucket",
				"region":   "us-east-1",
				"kmsKey":   "arn:aws:kms:...",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	raw, ok := dst.Annotations[BucketPendingAnnotation]
	require.True(t, ok)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.NotContains(t, decoded, "bucket")
	require.Contains(t, decoded, "defaultBucket")
}

func TestConvertTo_BothBucketAndDefaultBucket(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName":    "bucket-creds",
					"accessKeyName": "ACCESS_KEY",
					"secretKeyName": "SECRET_KEY",
				},
			},
			"defaultBucket": map[string]interface{}{
				"provider": "gcs",
				"name":     "wandb-bucket",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	raw, ok := dst.Annotations[BucketPendingAnnotation]
	require.True(t, ok)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Contains(t, decoded, "bucket")
	require.Contains(t, decoded, "defaultBucket")
}

func TestConvertTo_BucketAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, BucketPendingAnnotation)
}

func TestConvertTo_BucketEmptyMaps(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket":        map[string]interface{}{},
			"defaultBucket": map[string]interface{}{},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, BucketPendingAnnotation)
}

func TestConvertTo_GlobalNotAMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": "not-a-map",
	})
	require.Error(t, src.ConvertTo(dst))
}

func TestConvertTo_PreservesObjectMeta(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(nil)
	src.Labels = map[string]string{"app.kubernetes.io/name": "weightsandbiases"}
	src.Annotations = map[string]string{"existing": "value"}
	require.NoError(t, src.ConvertTo(dst))
	require.Equal(t, "weightsandbiases", dst.Labels["app.kubernetes.io/name"])
	require.Equal(t, "value", dst.Annotations["existing"])
}

func TestConvertFrom_AlwaysErrors(t *testing.T) {
	dst := &WeightsAndBiases{}
	src := &appsv2.WeightsAndBiases{}
	require.Error(t, dst.ConvertFrom(src))
}
