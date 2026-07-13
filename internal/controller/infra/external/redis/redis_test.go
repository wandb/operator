package redis

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const redisSourceSecretName = "external-redis"

func redisSel(key string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: redisSourceSecretName},
		Key:                  key,
	}
}

func TestWriteStateAddsCACertPathAndTLSWhenCACertPresent(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	source := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: redisSourceSecretName, Namespace: "default"},
		Data: map[string][]byte{
			"Host":     []byte("redis.example.com"),
			"Port":     []byte("6379"),
			"Password": []byte("secret"),
			"SslCa":    []byte("---ca---"),
		},
	}
	wandb := &apiv2.WeightsAndBiases{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Redis: map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {
				ExternalRedis: &apiv2.RedisConnection{
					Host:     redisSel("Host"),
					Port:     redisSel("Port"),
					Password: redisSel("Password"),
					SslCa:    redisSel("SslCa"),
				},
			}},
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, source).Build()

	conditions := WriteState(context.Background(), client, wandb, apiv2.DefaultInstanceName, wandb.Spec.Redis[apiv2.DefaultInstanceName].ExternalRedis)
	require.Nil(t, conditions)

	written := &corev1.Secret{}
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: ConnectionSecretName, Namespace: "default"}, written))
	data := redisConnectionData(written)
	parsed, err := url.Parse(data["url"])
	require.NoError(t, err)
	require.Equal(t, "redis", parsed.Scheme)
	require.Equal(t, "redis.example.com:6379", parsed.Host)
	require.Equal(t, "true", parsed.Query().Get("tls"))
	require.Equal(t, caCertPath, parsed.Query().Get("caCertPath"))
}

func redisConnectionData(secret *corev1.Secret) map[string]string {
	out := map[string]string{}
	for k, v := range secret.Data {
		out[k] = string(v)
	}
	for k, v := range secret.StringData {
		out[k] = v
	}
	return out
}
