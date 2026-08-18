package registryauth_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/wandb/operator/pkg/wandb/manifest/registryauth"
)

func dockerConfigJSON(host, user, pass string) []byte {
	tok := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	cfg := map[string]any{
		"auths": map[string]any{
			host: map[string]any{"username": user, "password": pass, "auth": tok},
		},
	}
	b, _ := json.Marshal(cfg)
	return b
}

func dockerSecret(name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       data,
	}
}

func TestResolve_NoPullSecrets_Anonymous(t *testing.T) {
	c := fake.NewClientBuilder().Build()
	ra, err := registryauth.Resolve(context.Background(), c, "ns", nil, false)
	require.NoError(t, err)
	require.Nil(t, ra, "no pull secrets should resolve to anonymous (nil auth)")
}

func TestResolve_ValidSecret_MatchesHost(t *testing.T) {
	secret := dockerSecret("reg", map[string][]byte{
		corev1.DockerConfigJsonKey: dockerConfigJSON("myreg.example.com", "u", "p"),
	})
	c := fake.NewClientBuilder().WithObjects(secret).Build()

	ra, err := registryauth.Resolve(context.Background(), c, "ns",
		[]corev1.LocalObjectReference{{Name: "reg"}}, false)
	require.NoError(t, err)
	require.NotNil(t, ra)
	require.NotNil(t, ra.Credential)

	cred, err := ra.Credential(context.Background(), "myreg.example.com")
	require.NoError(t, err)
	require.Equal(t, "u", cred.Username)
	require.Equal(t, "p", cred.Password)
}

func TestResolve_MissingSecret_Errors(t *testing.T) {
	c := fake.NewClientBuilder().Build()
	_, err := registryauth.Resolve(context.Background(), c, "ns",
		[]corev1.LocalObjectReference{{Name: "nope"}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestResolve_OpaqueSecretWithDockerConfig_Errors(t *testing.T) {
	// Valid dockerconfigjson payload but typed Opaque: kubelet silently ignores such a secret on
	// the workload pods we propagate this ref to, so the operator must reject it rather than let the
	// manifest pull pass while every workload image pull falls back to anonymous and 401s.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "reg", Namespace: "ns"},
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: dockerConfigJSON("myreg.example.com", "u", "p"),
		},
	}
	c := fake.NewClientBuilder().WithObjects(secret).Build()
	_, err := registryauth.Resolve(context.Background(), c, "ns",
		[]corev1.LocalObjectReference{{Name: "reg"}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "type")
}

func TestResolve_MissingDockerConfigKey_Errors(t *testing.T) {
	secret := dockerSecret("reg", map[string][]byte{"wrong": []byte("x")})
	c := fake.NewClientBuilder().WithObjects(secret).Build()
	_, err := registryauth.Resolve(context.Background(), c, "ns",
		[]corev1.LocalObjectReference{{Name: "reg"}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing key")
}
