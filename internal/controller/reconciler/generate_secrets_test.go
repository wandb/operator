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
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

func newGenerateSecretsFixture(
	t *testing.T,
	seed ...ctrlClient.Object,
) (ctrlClient.Client, *apiv2.WeightsAndBiases) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	wandb := &apiv2.WeightsAndBiases{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
	}
	objects := append([]ctrlClient.Object{wandb}, seed...)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv2.WeightsAndBiases{}).
		WithObjects(objects...).
		Build()
	return client, wandb
}

// effectiveSecretValue returns the value under key. The fake client does not
// fold StringData into Data, so prefer StringData then fall back to Data.
func effectiveSecretValue(sec *corev1.Secret, key string) string {
	if v, ok := sec.StringData[key]; ok {
		return v
	}
	return string(sec.Data[key])
}

func weaveWorkerAuthManifest() serverManifest.Manifest {
	return serverManifest.Manifest{
		GeneratedSecrets: []serverManifest.GeneratedSecret{
			{Name: "weave-worker-auth", Length: 32, CharacterType: "password", UseExactName: true},
		},
	}
}

// TestGenerateSecrets_FailsOnNonUTF8AdoptedSecret: a non-UTF-8 token must fail
// the reconcile loudly (error + Ready=false condition + warning event) rather
// than being silently rewritten.
func TestGenerateSecrets_FailsOnNonUTF8AdoptedSecret(t *testing.T) {
	invalid := []byte{0xff, 0xfe, 0xfd, 0x00, 0x80}
	require.False(t, utf8.Valid(invalid), "test precondition: bytes must be invalid UTF-8")

	seeded := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "weave-worker-auth", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"key": invalid},
	}
	client, wandb := newGenerateSecretsFixture(t, seeded)
	recorder := record.NewFakeRecorder(10)

	_, err := generateSecrets(context.Background(), client, recorder, wandb, weaveWorkerAuthManifest())
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-UTF-8")

	var sec corev1.Secret
	require.NoError(t, client.Get(context.Background(),
		types.NamespacedName{Name: "weave-worker-auth", Namespace: "default"}, &sec))
	require.Equal(t, invalid, sec.Data["key"], "invalid secret must not be overwritten")
	require.NotContains(t, sec.StringData, "key", "no regeneration should have occurred")

	require.False(t, wandb.Status.Ready)
	cond := apimeta.FindStatusCondition(wandb.Status.Conditions, readyConditionType)
	require.NotNil(t, cond)
	require.Equal(t, metav1.ConditionFalse, cond.Status)
	require.Equal(t, common.InvalidSecretEncodingReason, cond.Reason)

	select {
	case ev := <-recorder.Events:
		require.Contains(t, ev, common.InvalidSecretEncodingReason)
	default:
		t.Fatal("expected a warning event to be recorded")
	}
}

// TestGenerateSecrets_LeavesValidExistingValueUntouched: a valid adopted token
// is preserved (no needless rotation).
func TestGenerateSecrets_LeavesValidExistingValueUntouched(t *testing.T) {
	valid := []byte("already-valid-token-123")
	seeded := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "weave-worker-auth", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"key": valid},
	}
	client, wandb := newGenerateSecretsFixture(t, seeded)

	_, err := generateSecrets(context.Background(), client, record.NewFakeRecorder(10), wandb, weaveWorkerAuthManifest())
	require.NoError(t, err)

	var sec corev1.Secret
	require.NoError(t, client.Get(context.Background(),
		types.NamespacedName{Name: "weave-worker-auth", Namespace: "default"}, &sec))

	require.Equal(t, valid, sec.Data["key"], "valid existing value must not be overwritten")
	require.NotContains(t, sec.StringData, "key", "no regeneration should have occurred")
}

// TestGenerateSecrets_CreatesMissingSecretWithUTF8Token: fresh secrets hold a
// UTF-8-safe token.
func TestGenerateSecrets_CreatesMissingSecretWithUTF8Token(t *testing.T) {
	client, wandb := newGenerateSecretsFixture(t)

	_, err := generateSecrets(context.Background(), client, record.NewFakeRecorder(10), wandb, weaveWorkerAuthManifest())
	require.NoError(t, err)

	var sec corev1.Secret
	require.NoError(t, client.Get(context.Background(),
		types.NamespacedName{Name: "weave-worker-auth", Namespace: "default"}, &sec))

	value := effectiveSecretValue(&sec, "key")
	require.NotEmpty(t, value)
	require.True(t, utf8.ValidString(value))
	require.Len(t, value, 32)
}
