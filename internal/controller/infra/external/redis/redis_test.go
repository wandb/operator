package redis

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const redisSourceSecretName = "external-redis"

func redisSel(key string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: redisSourceSecretName},
		Key:                  key,
	}
}

func redisWriteStateFixture(t *testing.T, sourceData map[string][]byte) (ctrlclient.Client, *apiv2.WeightsAndBiases) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	source := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: redisSourceSecretName, Namespace: "default"},
		Data:       sourceData,
	}
	wandb := &apiv2.WeightsAndBiases{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Redis: map[string]apiv2.RedisSpec{apiv2.DefaultInstanceName: {
				ExternalRedis: &apiv2.RedisConnection{
					Host: redisSel("Host"),
					Port: redisSel("Port"),
				},
			}},
		},
	}
	if _, ok := sourceData["Password"]; ok {
		connection := wandb.Spec.Redis[apiv2.DefaultInstanceName]
		connection.ExternalRedis.Password = redisSel("Password")
		wandb.Spec.Redis[apiv2.DefaultInstanceName] = connection
	}
	if _, ok := sourceData["SslCa"]; ok {
		connection := wandb.Spec.Redis[apiv2.DefaultInstanceName]
		connection.ExternalRedis.SslCa = redisSel("SslCa")
		wandb.Spec.Redis[apiv2.DefaultInstanceName] = connection
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, source).Build(), wandb
}

func TestWriteStateAddsCACertPathAndTLSWhenCACertPresent(t *testing.T) {
	client, wandb := redisWriteStateFixture(t, map[string][]byte{
		"Host":     []byte("redis.example.com"),
		"Port":     []byte("6379"),
		"Password": []byte("secret"),
		"SslCa":    []byte("---ca---"),
	})

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

func TestWriteStateRejectsInvalidRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		data map[string][]byte
	}{
		{name: "empty host", data: map[string][]byte{"Host": {}, "Port": []byte("6379")}},
		{name: "empty port", data: map[string][]byte{"Host": []byte("redis.example.com"), "Port": {}}},
		{name: "non-numeric port", data: map[string][]byte{"Host": []byte("redis.example.com"), "Port": []byte("redis")}},
		{name: "zero port", data: map[string][]byte{"Host": []byte("redis.example.com"), "Port": []byte("0")}},
		{name: "port above range", data: map[string][]byte{"Host": []byte("redis.example.com"), "Port": []byte("65536")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, wandb := redisWriteStateFixture(t, test.data)

			conditions := WriteState(context.Background(), client, wandb, apiv2.DefaultInstanceName, wandb.Spec.Redis[apiv2.DefaultInstanceName].ExternalRedis)

			require.Len(t, conditions, 1)
			require.Equal(t, common.ReconciledType, conditions[0].Type)
			require.Equal(t, metav1.ConditionFalse, conditions[0].Status)
			require.Equal(t, common.ResourceErrorReason, conditions[0].Reason)
			require.NotEmpty(t, conditions[0].Message)

			err := client.Get(context.Background(), types.NamespacedName{Name: ConnectionSecretName, Namespace: "default"}, &corev1.Secret{})
			require.True(t, apierrors.IsNotFound(err))
		})
	}
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
