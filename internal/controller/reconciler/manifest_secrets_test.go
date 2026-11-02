package reconciler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	servermanifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// manifestSecretTestClient supplies only the API client and installation identity.
// Each test declares the manifest and any behavior-specific configuration itself.
func manifestSecretTestClient(t *testing.T) (client.WithWatch, *apiv2.WeightsAndBiases) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "test", UID: types.UID("install-1")},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(wandb).WithObjects(wandb).Build()
	return c, wandb
}

func TestManifestSecretsCreatesEmptySecret(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	manifest := servermanifest.Manifest{Secrets: []servermanifest.Secret{{Name: "signing", UseExactName: true}}}
	ctx := context.Background()

	_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "signing", Namespace: wandb.Namespace}, secret))
	require.Empty(t, secret.Data)
	require.True(t, hasOwnerReference(secret, wandb), "empty Secret must belong to the installation")
	require.Equal(t, corev1.SecretTypeOpaque, secret.Type)
	require.NotContains(t, wandb.Status.GeneratedSecrets, "signing", "empty Secret must not advertise a generated key")
}

func TestManifestSecretsCreatesGeneratedSecret(t *testing.T) {
	tests := []struct {
		name             string
		declaration      servermanifest.Secret
		wantResourceName string
		wantLength       int
	}{
		{
			name:             "configured generation uses installation prefix",
			declaration:      servermanifest.Secret{Name: "session", Generated: &servermanifest.SecretGeneration{Length: 17, CharacterType: "password"}},
			wantResourceName: "wandb-session",
			wantLength:       17,
		},
		{
			name:             "empty generation configuration uses default length and exact name",
			declaration:      servermanifest.Secret{Name: "default", UseExactName: true, Generated: &servermanifest.SecretGeneration{}},
			wantResourceName: "default",
			wantLength:       32,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, wandb := manifestSecretTestClient(t)
			manifest := servermanifest.Manifest{Secrets: []servermanifest.Secret{test.declaration}}
			ctx := context.Background()

			_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
			require.NoError(t, err)

			secret := &corev1.Secret{}
			require.NoError(t, c.Get(ctx, client.ObjectKey{Name: test.wantResourceName, Namespace: wandb.Namespace}, secret))
			require.Len(t, secret.Data["key"], test.wantLength)
			selector, exists := wandb.Status.GeneratedSecrets[test.declaration.Name]
			require.True(t, exists, "generated Secret must advertise its key")
			require.Equal(t, test.wantResourceName, selector.Name)
			require.Equal(t, "key", selector.Key)
		})
	}
}

func TestManifestSecretsGeneratedKeyResolvesAsEnvReference(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	manifest := servermanifest.Manifest{Secrets: []servermanifest.Secret{
		{Name: "session", Generated: &servermanifest.SecretGeneration{Length: 17}},
	}}
	ctx := context.Background()
	_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)

	env, err := resolveEnvvars(ctx, c, wandb, manifest, nil, []servermanifest.EnvVar{
		{Name: "SESSION", Sources: []servermanifest.EnvSource{{Type: "generatedSecret", Name: "session"}}},
	})
	require.NoError(t, err)
	require.Len(t, env, 1)
	require.NotNil(t, env[0].ValueFrom)
	require.NotNil(t, env[0].ValueFrom.SecretKeyRef)
	require.Equal(t, "wandb-session", env[0].ValueFrom.SecretKeyRef.Name)
}

func TestManifestSecretsRepeatedReconciliationPreservesDataAndMetadata(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	manifest := servermanifest.Manifest{Secrets: []servermanifest.Secret{
		{Name: "signing", UseExactName: true},
		{Name: "session", Generated: &servermanifest.SecretGeneration{Length: 17}},
		{Name: "default", UseExactName: true, Generated: &servermanifest.SecretGeneration{}},
	}}
	ctx := context.Background()
	_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)

	// Simulate a migration filling the precreated Secret before the next reconciliation.
	signer := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "signing", Namespace: wandb.Namespace}, signer))
	signer.Data = map[string][]byte{"key.pem": []byte("signer material")}
	require.NoError(t, c.Update(ctx, signer))
	before := &corev1.SecretList{}
	require.NoError(t, c.List(ctx, before))

	_, err = reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)

	after := &corev1.SecretList{}
	require.NoError(t, c.List(ctx, after))
	require.Equal(t, before.Items, after.Items, "reconciliation must preserve existing Secret data and metadata")
}

func TestManifestSecretsRepairsMissingLegacyKey(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	ctx := context.Background()
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy", Namespace: wandb.Namespace, Annotations: map[string]string{"keep": "metadata"}},
		Data:       map[string][]byte{"other": []byte("preserved")},
	}
	require.NoError(t, c.Create(ctx, existing))
	manifest := servermanifest.Manifest{GeneratedSecrets: []servermanifest.GeneratedSecret{{Name: "legacy", UseExactName: true, Length: 21}}}

	_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(existing), existing))
	require.Len(t, existing.Data["key"], 21)
	require.Equal(t, "preserved", string(existing.Data["other"]))
	require.Equal(t, "metadata", existing.Annotations["keep"])
	require.True(t, hasOwnerReference(existing, wandb), "repaired Secret must belong to the installation")
}

func TestManifestSecretsLegacyGenerationUsesInstallationPrefix(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	manifest := servermanifest.Manifest{GeneratedSecrets: []servermanifest.GeneratedSecret{{Name: "prefixed", Length: 15}}}
	ctx := context.Background()

	_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)
	require.Equal(t, "wandb-prefixed", wandb.Status.GeneratedSecrets["prefixed"].Name)
}

func TestManifestSecretsSchemaMigrationPreservesExistingKey(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	ctx := context.Background()
	manifest := servermanifest.Manifest{GeneratedSecrets: []servermanifest.GeneratedSecret{{Name: "legacy", UseExactName: true, Length: 21}}}
	_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)
	existing := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "legacy", Namespace: wandb.Namespace}, existing))
	original := existing.DeepCopy()

	// Changing both the schema and requested length must not regenerate an existing key.
	manifest.GeneratedSecrets = nil
	manifest.Secrets = []servermanifest.Secret{{Name: "legacy", UseExactName: true, Generated: &servermanifest.SecretGeneration{Length: 99}}}
	_, err = reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.NoError(t, err)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(existing), existing))
	require.Equal(t, original, existing, "schema migration must preserve the key and metadata even when the requested length changes")
}

func TestManifestSecretsRejectInvalidOrConflictingDeclarationsBeforeWrites(t *testing.T) {
	tests := []struct {
		name             string
		additionalSecret servermanifest.Secret
		legacySecrets    []servermanifest.GeneratedSecret
	}{
		{name: "missing name", additionalSecret: servermanifest.Secret{}},
		{name: "invalid name", additionalSecret: servermanifest.Secret{Name: "invalid/name"}},
		{name: "duplicate logical name", additionalSecret: servermanifest.Secret{Name: "first", UseExactName: true}},
		{name: "duplicate resource name", additionalSecret: servermanifest.Secret{Name: "wandb-first", UseExactName: true}},
		{name: "legacy collision", additionalSecret: servermanifest.Secret{Name: "second"}, legacySecrets: []servermanifest.GeneratedSecret{{Name: "first"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, wandb := manifestSecretTestClient(t)
			// The valid first declaration must not be written before the later conflict is found.
			manifest := servermanifest.Manifest{
				Secrets:          []servermanifest.Secret{{Name: "first"}, test.additionalSecret},
				GeneratedSecrets: test.legacySecrets,
			}
			_, err := reconcileManifestSecrets(context.Background(), c, wandb, manifest)
			require.Error(t, err, "invalid declaration must be rejected")
			secrets := &corev1.SecretList{}
			require.NoError(t, c.List(context.Background(), secrets))
			require.Empty(t, secrets.Items, "all declarations must be validated before creating Secrets")
		})
	}
}

func TestManifestSecretsFinalizationPreservesKeys(t *testing.T) {
	tests := []struct {
		name      string
		policy    apiv2.OnDeletePolicy
		wantOwned bool
	}{
		{name: "detach removes installation ownership", policy: apiv2.DetachOnDelete, wantOwned: false},
		{name: "unspecified policy defaults to detach", policy: "", wantOwned: false},
		{name: "purge retains ownership for garbage collection", policy: apiv2.PurgeOnDelete, wantOwned: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, wandb := manifestSecretTestClient(t)
			ctx := context.Background()
			wandb.Spec.RetentionPolicy.OnDelete = test.policy
			require.NoError(t, c.Update(ctx, wandb))
			manifest := servermanifest.Manifest{Secrets: []servermanifest.Secret{
				{Name: "signer", UseExactName: true},
				{Name: "session", Generated: &servermanifest.SecretGeneration{Length: 17}},
			}}
			_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
			require.NoError(t, err)
			signer := &corev1.Secret{}
			require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "signer", Namespace: wandb.Namespace}, signer))
			signer.Data = map[string][]byte{"key.pem": []byte("existing signer key")}
			require.NoError(t, c.Update(ctx, signer))
			session := &corev1.Secret{}
			require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "wandb-session", Namespace: wandb.Namespace}, session))
			originalSession := string(session.Data["key"])
			// Older generated Secrets are identified through status rather than the inventory label.
			delete(session.Labels, manifestSecretLabel)
			require.NoError(t, c.Update(ctx, session))

			require.NoError(t, finalizeManifestSecrets(ctx, c, wandb), "first finalization")
			require.NoError(t, finalizeManifestSecrets(ctx, c, wandb), "repeated finalization must be safe")

			for _, secret := range []*corev1.Secret{signer, session} {
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), secret))
				require.Equal(t, test.wantOwned, hasOwnerReference(secret, wandb), "ownership of %s", secret.Name)
			}
			require.Equal(t, "existing signer key", string(signer.Data["key.pem"]), "finalization must preserve signer data")
			require.Equal(t, originalSession, string(session.Data["key"]), "finalization must preserve generated keys")
		})
	}
}

func TestManifestSecretsReadoptionPreservesKeys(t *testing.T) {
	for _, test := range []struct {
		name   string
		policy apiv2.OnDeletePolicy
	}{
		{name: "explicit detach", policy: apiv2.DetachOnDelete},
		{name: "unspecified policy defaults to detach", policy: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, wandb := manifestSecretTestClient(t)
			ctx := context.Background()
			wandb.Spec.RetentionPolicy.OnDelete = test.policy
			require.NoError(t, c.Update(ctx, wandb))
			manifest := servermanifest.Manifest{Secrets: []servermanifest.Secret{
				{Name: "signer", UseExactName: true},
				{Name: "session", Generated: &servermanifest.SecretGeneration{Length: 17}},
			}}
			_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
			require.NoError(t, err)
			signer := &corev1.Secret{}
			require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "signer", Namespace: wandb.Namespace}, signer))
			signer.Data = map[string][]byte{"key.pem": []byte("existing signer key")}
			require.NoError(t, c.Update(ctx, signer))
			session := &corev1.Secret{}
			require.NoError(t, c.Get(ctx, client.ObjectKey{Name: "wandb-session", Namespace: wandb.Namespace}, session))
			originalSession := string(session.Data["key"])
			// Keep legacy generated Secrets, identified by status, in the reinstallation scenario.
			delete(session.Labels, manifestSecretLabel)
			require.NoError(t, c.Update(ctx, session))
			require.NoError(t, finalizeManifestSecrets(ctx, c, wandb))
			require.NoError(t, c.Delete(ctx, wandb))

			// Reinstall with the same name but a new UID: ownership must move to the new
			// installation without replacing either the signer data or generated key.
			replacement := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{Name: wandb.Name, Namespace: wandb.Namespace, UID: types.UID("replacement")},
				Spec:       wandb.Spec,
			}
			require.NoError(t, c.Create(ctx, replacement))
			_, err = reconcileManifestSecrets(ctx, c, replacement, manifest)
			require.NoError(t, err)
			for _, secret := range []*corev1.Secret{signer, session} {
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), secret))
				require.True(t, hasOwnerReference(secret, replacement), "%s must belong to the replacement installation", secret.Name)
				require.False(t, hasOwnerReference(secret, wandb), "%s must no longer belong to the deleted installation", secret.Name)
			}
			require.Equal(t, "existing signer key", string(signer.Data["key.pem"]))
			require.Equal(t, originalSession, string(session.Data["key"]))
		})
	}
}

func TestManifestSecretDetachPreservesUnrelatedOwnership(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	ctx := context.Background()
	owner := metav1.OwnerReference{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases", Name: wandb.Name, UID: wandb.UID}
	other := metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "other", UID: types.UID("other")}
	tests := []struct {
		name       string
		secret     *corev1.Secret
		wantOwners []metav1.OwnerReference
	}{
		{
			name: "shared Secret loses only installation owner",
			secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: "shared", Namespace: wandb.Namespace,
				Labels:          map[string]string{manifestSecretLabel: "true"},
				OwnerReferences: []metav1.OwnerReference{owner, other},
			}},
			wantOwners: []metav1.OwnerReference{other},
		},
		{
			name: "Secret outside manifest inventory retains installation owner",
			secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: "unrelated", Namespace: wandb.Namespace,
				OwnerReferences: []metav1.OwnerReference{owner},
			}},
			wantOwners: []metav1.OwnerReference{owner},
		},
		{
			name: "foreign Secret retains other owner",
			secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: "foreign", Namespace: wandb.Namespace,
				Labels:          map[string]string{manifestSecretLabel: "true"},
				OwnerReferences: []metav1.OwnerReference{other},
			}},
			wantOwners: []metav1.OwnerReference{other},
		},
		{
			name: "Secret outside installation namespace is unchanged",
			secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: "outside", Namespace: "elsewhere",
				Labels:          map[string]string{manifestSecretLabel: "true"},
				OwnerReferences: []metav1.OwnerReference{owner},
			}},
			wantOwners: []metav1.OwnerReference{owner},
		},
	}
	for _, test := range tests {
		require.NoError(t, c.Create(ctx, test.secret))
	}

	require.NoError(t, finalizeManifestSecrets(ctx, c, wandb))

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := &corev1.Secret{}
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(test.secret), actual))
			require.Equal(t, test.wantOwners, actual.OwnerReferences)
		})
	}
}

func TestManifestSecretsRefuseForeignOwnerBeforeChangingData(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	ctx := context.Background()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: "foreign", Namespace: wandb.Namespace,
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases",
			Name: "elsewhere", UID: types.UID("foreign"),
		}},
	}}
	require.NoError(t, c.Create(ctx, secret))
	original := secret.DeepCopy()
	manifest := servermanifest.Manifest{Secrets: []servermanifest.Secret{{Name: secret.Name, UseExactName: true, Generated: &servermanifest.SecretGeneration{}}}}
	_, err := reconcileManifestSecrets(ctx, c, wandb, manifest)
	require.Error(t, err, "foreign-owned Secret must not be adopted")
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(secret), secret))
	require.Equal(t, original, secret, "foreign-owned Secret must not be modified")
}

func TestManifestSecretDetachFailureKeepsCleanupFinalizer(t *testing.T) {
	c, wandb := manifestSecretTestClient(t)
	ctx := context.Background()
	wandb.Finalizers = []string{CleanupFinalizer}
	// Deletion must detach Secrets using persisted state even if the manifest cannot be fetched.
	wandb.Spec.Wandb.ManifestRepository = "file:///unavailable-during-deletion"
	require.NoError(t, c.Update(ctx, wandb))
	_, err := reconcileManifestSecrets(ctx, c, wandb, servermanifest.Manifest{Secrets: []servermanifest.Secret{{Name: "signer", UseExactName: true}}})
	require.NoError(t, err)
	require.NoError(t, c.Delete(ctx, wandb))
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(wandb), wandb))
	failing := interceptor.NewClient(c, interceptor.Funcs{Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
		if _, ok := obj.(*corev1.Secret); ok {
			return errors.New("detach write failed")
		}
		return c.Update(ctx, obj, opts...)
	}})
	_, err = Reconcile(ctx, failing, record.NewFakeRecorder(10), wandb, telemetry.TelemetryRuntimeConfig{})
	require.ErrorContains(t, err, "detach write failed", "detach must happen before looking up the unavailable manifest")
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(wandb), wandb))
	require.Equal(t, []string{CleanupFinalizer}, wandb.Finalizers, "cleanup finalizer must remain until Secret detach succeeds")
}
