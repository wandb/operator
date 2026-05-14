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

func TestConvertTo_OIDCAllLiterals(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId":   "abc",
					"secret":     "shh",
					"authMethod": "client_secret_post",
					"issuer":     "https://example.com",
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
	require.NotContains(t, decoded, "oidcSecret")

	require.Empty(t, dst.Spec.Wandb.OIDC.ClientId.Name, "no ref-shaped values, so spec.wandb.oidc stays unset")
	require.Empty(t, dst.Spec.Wandb.OIDC.ClientSecret.Name)
}

func TestConvertTo_OIDCLegacyOidcSecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId": "abc",
					"secret":   "shh",
					"oidcSecret": map[string]interface{}{
						"name":      "user-oidc-secret",
						"secretKey": "MY_KEY",
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "user-oidc-secret", dst.Spec.Wandb.OIDC.ClientSecret.Name)
	require.Equal(t, "MY_KEY", dst.Spec.Wandb.OIDC.ClientSecret.Key)

	raw := dst.Annotations[OIDCPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "abc", decoded["clientId"])
	require.NotContains(t, decoded, "secret", "literal secret must not be stashed when oidcSecret ref took over")
	require.NotContains(t, decoded, "oidcSecret")
}

func TestConvertTo_OIDCLegacyOidcSecretDefaultKey(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"oidcSecret": map[string]interface{}{
						"name": "user-oidc-secret",
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "user-oidc-secret", dst.Spec.Wandb.OIDC.ClientSecret.Name)
	require.Equal(t, "OIDC_SECRET", dst.Spec.Wandb.OIDC.ClientSecret.Key)
}

func TestConvertTo_OIDCValueFromRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "oidc-settings",
								"key":  "clientId",
							},
						},
					},
					"secret": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "oidc-settings",
								"key":  "clientSecret",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	oidc := dst.Spec.Wandb.OIDC
	require.Equal(t, "oidc-settings", oidc.ClientId.Name)
	require.Equal(t, "clientId", oidc.ClientId.Key)
	require.Equal(t, "oidc-settings", oidc.ClientSecret.Name)
	require.Equal(t, "clientSecret", oidc.ClientSecret.Key)

	require.NotContains(t, dst.Annotations, OIDCPendingAnnotation,
		"no literals provided, so no annotation should be created")
}

func TestConvertTo_OIDCMixedLiteralsAndRefs(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId":   "abc",
					"authMethod": "client_secret_post",
					"secret": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "oidc-secret",
								"key":  "clientSecret",
							},
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "oidc-secret", dst.Spec.Wandb.OIDC.ClientSecret.Name)
	require.Empty(t, dst.Spec.Wandb.OIDC.ClientId.Name)

	raw := dst.Annotations[OIDCPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "abc", decoded["clientId"])
	require.Equal(t, "client_secret_post", decoded["authMethod"])
	require.NotContains(t, decoded, "secret")
}

func TestConvertTo_OIDCValueFromWinsOverLegacyOidcSecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"secret": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "valueFrom-secret",
								"key":  "clientSecret",
							},
						},
					},
					"oidcSecret": map[string]interface{}{
						"name":      "legacy-secret",
						"secretKey": "LEGACY_KEY",
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Equal(t, "valueFrom-secret", dst.Spec.Wandb.OIDC.ClientSecret.Name,
		"secret.valueFrom should win over the legacy oidcSecret block")
}

func TestConvertTo_OIDCMalformedRefMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"auth": map[string]interface{}{
				"oidc": map[string]interface{}{
					"clientId": map[string]interface{}{
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name": "no-key-here",
							},
						},
					},
				},
			},
		},
	})
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "clientId")
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
	require.Empty(t, dst.Spec.Wandb.OIDC.ClientId.Name)
}

func TestConvertTo_MySQLAllLiterals(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host":     "mysql.example.com",
				"port":     int64(3306),
				"database": "wandb_local",
				"user":     "wandb",
				"password": "shh",
				"caCert":   "---cert---",
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
	require.Equal(t, "wandb", decoded["user"])
	require.Equal(t, "shh", decoded["password"])
	require.Equal(t, "---cert---", decoded["caCert"])
	require.NotContains(t, decoded, "passwordSecret")

	require.Nil(t, dst.Spec.MySQL.ExternalMysql, "no ref-shaped values, so externalMysql stays unset")
}

func TestConvertTo_MySQLLegacyPasswordSecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host":     "mysql.example.com",
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

	require.NotNil(t, dst.Spec.MySQL.ExternalMysql)
	require.Equal(t, "mysql-creds", dst.Spec.MySQL.ExternalMysql.Password.Name)
	require.Equal(t, "MYSQL_PASSWORD", dst.Spec.MySQL.ExternalMysql.Password.Key)

	raw := dst.Annotations[MySQLPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "mysql.example.com", decoded["host"])
	require.NotContains(t, decoded, "password", "literal password must not be stashed when passwordSecret took over")
	require.NotContains(t, decoded, "passwordSecret")
}

func TestConvertTo_MySQLLegacyPasswordSecretDefaultKey(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"passwordSecret": map[string]interface{}{
					"name": "mysql-creds",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL.ExternalMysql)
	require.Equal(t, "mysql-creds", dst.Spec.MySQL.ExternalMysql.Password.Name)
	require.Equal(t, "MYSQL_PASSWORD", dst.Spec.MySQL.ExternalMysql.Password.Key)
}

func TestConvertTo_MySQLValueFromRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "mysql-settings",
							"key":  "endpoint",
						},
					},
				},
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "mysql-secret",
							"key":  "password",
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL.ExternalMysql)
	conn := dst.Spec.MySQL.ExternalMysql
	require.Equal(t, "mysql-settings", conn.Host.Name)
	require.Equal(t, "endpoint", conn.Host.Key)
	require.Equal(t, "mysql-secret", conn.Password.Name)
	require.Equal(t, "password", conn.Password.Key)

	require.NotContains(t, dst.Annotations, MySQLPendingAnnotation,
		"no literals provided, so no annotation should be created")
}

func TestConvertTo_MySQLMixedLiteralsAndRefs(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host": "mysql.example.com",
				"port": int64(3306),
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "mysql-secret",
							"key":  "password",
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL.ExternalMysql)
	require.Equal(t, "mysql-secret", dst.Spec.MySQL.ExternalMysql.Password.Name)
	require.Empty(t, dst.Spec.MySQL.ExternalMysql.Host.Name)

	raw := dst.Annotations[MySQLPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "mysql.example.com", decoded["host"])
	require.Contains(t, decoded, "port")
	require.NotContains(t, decoded, "password")
}

func TestConvertTo_MySQLValueFromWinsOverPasswordSecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "valueFrom-secret",
							"key":  "password",
						},
					},
				},
				"passwordSecret": map[string]interface{}{
					"name":        "legacy-secret",
					"passwordKey": "LEGACY_KEY",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.MySQL.ExternalMysql)
	require.Equal(t, "valueFrom-secret", dst.Spec.MySQL.ExternalMysql.Password.Name,
		"password.valueFrom should win over the legacy passwordSecret block")
}

func TestConvertTo_MySQLMalformedRefMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"mysql": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "no-key-here",
						},
					},
				},
			},
		},
	})
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "host")
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

func TestConvertTo_RedisAllLiterals(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host":     "redis.example.com",
				"port":     int64(6379),
				"password": "shh",
				"external": true,
				"caCert":   "----BEGIN CERT----",
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
	require.Equal(t, "----BEGIN CERT----", decoded["caCert"])
	require.NotContains(t, decoded, "external", "fields outside the known v2 mapping must be dropped")
	require.NotContains(t, decoded, "secret")

	require.Nil(t, dst.Spec.Redis.ExternalRedis, "no ref-shaped values, so externalRedis stays unset")
}

func TestConvertTo_RedisLegacySecretRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host":     "redis.example.com",
				"password": "shh",
				"secret": map[string]interface{}{
					"secretName": "redis-creds",
					"secretKey":  "REDIS_PASSWORD",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis.ExternalRedis)
	require.Equal(t, "redis-creds", dst.Spec.Redis.ExternalRedis.Password.Name)
	require.Equal(t, "REDIS_PASSWORD", dst.Spec.Redis.ExternalRedis.Password.Key)

	raw := dst.Annotations[RedisPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "redis.example.com", decoded["host"])
	require.NotContains(t, decoded, "password", "literal password must not be stashed when secret ref took over")
}

func TestConvertTo_RedisLegacySecretRefDefaultKey(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName": "redis-creds",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis.ExternalRedis)
	require.Equal(t, "redis-creds", dst.Spec.Redis.ExternalRedis.Password.Name)
	require.Equal(t, "REDIS_PASSWORD", dst.Spec.Redis.ExternalRedis.Password.Key)
}

func TestConvertTo_RedisValueFromRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "redis-settings",
							"key":  "endpoint",
						},
					},
				},
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "redis-secret",
							"key":  "password",
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis.ExternalRedis)
	conn := dst.Spec.Redis.ExternalRedis
	require.Equal(t, "redis-settings", conn.Host.Name)
	require.Equal(t, "endpoint", conn.Host.Key)
	require.Equal(t, "redis-secret", conn.Password.Name)
	require.Equal(t, "password", conn.Password.Key)

	require.NotContains(t, dst.Annotations, RedisPendingAnnotation,
		"no literals provided, so no annotation should be created")
}

func TestConvertTo_RedisMixedLiteralsAndRefs(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host": "redis.example.com",
				"port": int64(6379),
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "redis-secret",
							"key":  "password",
						},
					},
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis.ExternalRedis)
	require.Equal(t, "redis-secret", dst.Spec.Redis.ExternalRedis.Password.Name)
	require.Empty(t, dst.Spec.Redis.ExternalRedis.Host.Name)

	raw := dst.Annotations[RedisPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "redis.example.com", decoded["host"])
	require.Contains(t, decoded, "port")
	require.NotContains(t, decoded, "password")
}

func TestConvertTo_RedisValueFromWinsOverLegacySecret(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"password": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "valueFrom-secret",
							"key":  "password",
						},
					},
				},
				"secret": map[string]interface{}{
					"secretName": "legacy-secret",
					"secretKey":  "LEGACY_KEY",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.Redis.ExternalRedis)
	require.Equal(t, "valueFrom-secret", dst.Spec.Redis.ExternalRedis.Password.Name,
		"password.valueFrom should win over the legacy secret block")
}

func TestConvertTo_RedisMalformedRefMap(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"redis": map[string]interface{}{
				"host": map[string]interface{}{
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name": "no-key-here",
						},
					},
				},
			},
		},
	})
	err := src.ConvertTo(dst)
	require.Error(t, err)
	require.Contains(t, err.Error(), "host")
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

func TestConvertTo_BucketSecretRef(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName":    "bucket-creds",
					"accessKeyName": "MY_ACCESS",
					"secretKeyName": "MY_SECRET",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.ObjectStore.ExternalObjectStore)
	ext := dst.Spec.ObjectStore.ExternalObjectStore
	require.Equal(t, "bucket-creds", ext.AccessKey.Name)
	require.Equal(t, "MY_ACCESS", ext.AccessKey.Key)
	require.Equal(t, "bucket-creds", ext.SecretKey.Name)
	require.Equal(t, "MY_SECRET", ext.SecretKey.Key)

	require.NotContains(t, dst.Annotations, BucketPendingAnnotation,
		"no literals besides the secret block, so no annotation should be created")
}

func TestConvertTo_BucketSecretRefDefaultKeys(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName": "bucket-creds",
				},
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.ObjectStore.ExternalObjectStore)
	ext := dst.Spec.ObjectStore.ExternalObjectStore
	require.Equal(t, "ACCESS_KEY", ext.AccessKey.Key)
	require.Equal(t, "SECRET_KEY", ext.SecretKey.Key)
}

func TestConvertTo_BucketSecretRefEmptyName(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName": "",
				},
				"provider": "s3",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.Nil(t, dst.Spec.ObjectStore.ExternalObjectStore,
		"empty secretName should not produce an externalObjectStore")

	raw := dst.Annotations[BucketPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.NotContains(t, decoded, "secret", "secret block must be stripped even when its secretName is empty")
	require.Equal(t, "s3", decoded["provider"])
}

func TestConvertTo_BucketLiteralsOnlyBucket(t *testing.T) {
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
	require.Nil(t, dst.Spec.ObjectStore.ExternalObjectStore)

	raw, ok := dst.Annotations[BucketPendingAnnotation]
	require.True(t, ok)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "s3", decoded["provider"])
	require.Equal(t, "wandb-bucket", decoded["name"])
	require.Equal(t, "us-east-1", decoded["region"])
	require.Equal(t, "AKIA...", decoded["accessKey"])
	require.Equal(t, "secret", decoded["secretKey"])
}

func TestConvertTo_BucketLiteralsOnlyDefaultBucket(t *testing.T) {
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
	require.Equal(t, "s3", decoded["provider"])
	require.Equal(t, "wandb-bucket", decoded["name"])
	require.Equal(t, "us-east-1", decoded["region"])
	require.Equal(t, "arn:aws:kms:...", decoded["kmsKey"])
}

func TestConvertTo_BucketLiteralsMerged_BucketWins(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"provider": "gcs",
				"name":     "bucket-name",
			},
			"defaultBucket": map[string]interface{}{
				"provider": "s3",
				"name":     "default-name",
				"region":   "us-east-1",
				"kmsKey":   "kms-fallback",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw := dst.Annotations[BucketPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "gcs", decoded["provider"], "bucket overrides defaultBucket")
	require.Equal(t, "bucket-name", decoded["name"], "bucket overrides defaultBucket")
	require.Equal(t, "us-east-1", decoded["region"], "defaultBucket fills in fields bucket didn't set")
	require.Equal(t, "kms-fallback", decoded["kmsKey"])
}

func TestConvertTo_BucketEmptyValueDoesNotOverride(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"provider": "",
			},
			"defaultBucket": map[string]interface{}{
				"provider": "s3",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	raw := dst.Annotations[BucketPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "s3", decoded["provider"], "empty bucket value must not erase the defaultBucket value")
}

func TestConvertTo_BucketSecretRefAndLiterals(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"secret": map[string]interface{}{
					"secretName": "bucket-creds",
				},
				"provider": "s3",
				"name":     "wandb-bucket",
			},
			"defaultBucket": map[string]interface{}{
				"region": "us-east-1",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))

	require.NotNil(t, dst.Spec.ObjectStore.ExternalObjectStore)
	require.Equal(t, "bucket-creds", dst.Spec.ObjectStore.ExternalObjectStore.AccessKey.Name)

	raw := dst.Annotations[BucketPendingAnnotation]
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.NotContains(t, decoded, "secret")
	require.Equal(t, "s3", decoded["provider"])
	require.Equal(t, "wandb-bucket", decoded["name"])
	require.Equal(t, "us-east-1", decoded["region"])
}

func TestConvertTo_BucketAbsent(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{"host": "http://x"},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, BucketPendingAnnotation)
	require.Nil(t, dst.Spec.ObjectStore.ExternalObjectStore)
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
	require.Nil(t, dst.Spec.ObjectStore.ExternalObjectStore)
}

func TestConvertTo_BucketAllEmptyValues(t *testing.T) {
	dst := &appsv2.WeightsAndBiases{}
	src := newV1(map[string]interface{}{
		"global": map[string]interface{}{
			"bucket": map[string]interface{}{
				"provider": "",
				"name":     "",
			},
			"defaultBucket": map[string]interface{}{
				"region": "",
			},
		},
	})
	require.NoError(t, src.ConvertTo(dst))
	require.NotContains(t, dst.Annotations, BucketPendingAnnotation,
		"every value is empty, so the merged annotation should be skipped")
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
