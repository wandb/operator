package reconciler

import (
	"context"
	"errors"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"strings"
	"testing"

	servermanifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestManifestSecretsCreateEmptyAndGenerated(t *testing.T) {
	c, w, _ := migrationIdentityFixture(t)
	m := servermanifest.Manifest{Secrets: []servermanifest.Secret{
		{Name: "signing", UseExactName: true},
		{Name: "session", Generated: &servermanifest.SecretGeneration{Length: 17, CharacterType: "password"}},
		{Name: "default", UseExactName: true, Generated: &servermanifest.SecretGeneration{}},
	}}
	ctx := context.Background()
	if _, err := reconcileManifestSecrets(ctx, c, w, m); err != nil {
		t.Fatal(err)
	}
	empty := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: "signing", Namespace: w.Namespace}, empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Data) != 0 || !hasOwnerReference(empty, w) || empty.Type != corev1.SecretTypeOpaque {
		t.Fatalf("invalid empty Secret: %+v", empty)
	}
	if _, exists := w.Status.GeneratedSecrets["signing"]; exists {
		t.Fatal("empty Secret advertised a generated key")
	}
	for _, tc := range []struct {
		name, logical string
		length        int
	}{{"wandb-session", "session", 17}, {"default", "default", 32}} {
		secret := &corev1.Secret{}
		if err := c.Get(ctx, client.ObjectKey{Name: tc.name, Namespace: w.Namespace}, secret); err != nil {
			t.Fatal(err)
		}
		if len(secret.Data["key"]) != tc.length {
			t.Fatalf("%s length = %d", tc.name, len(secret.Data["key"]))
		}
		selector := w.Status.GeneratedSecrets[tc.logical]
		if selector.Name != tc.name || selector.Key != "key" {
			t.Fatalf("wrong generated selector: %+v", selector)
		}
	}
	env, err := resolveEnvvars(ctx, c, w, m, nil, []servermanifest.EnvVar{{Name: "SESSION", Sources: []servermanifest.EnvSource{{Type: "generatedSecret", Name: "session"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 1 || env[0].ValueFrom == nil || env[0].ValueFrom.SecretKeyRef == nil || env[0].ValueFrom.SecretKeyRef.Name != "wandb-session" {
		t.Fatalf("generated env source lost compatibility: %+v", env)
	}
	// A migration writes data after precreation; reconciliation must not reset it.
	empty.Data = map[string][]byte{"key.pem": []byte("signer material")}
	if err := c.Update(ctx, empty); err != nil {
		t.Fatal(err)
	}
	before := &corev1.SecretList{}
	if err := c.List(ctx, before); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileManifestSecrets(ctx, c, w, m); err != nil {
		t.Fatal(err)
	}
	after := &corev1.SecretList{}
	if err := c.List(ctx, after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Items, after.Items) {
		t.Fatal("reconciliation changed existing data or metadata")
	}
}

func TestManifestSecretsPreserveLegacyGenerationAndRepairMissingKey(t *testing.T) {
	c, w, _ := migrationIdentityFixture(t)
	ctx := context.Background()
	existing := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "legacy", Namespace: w.Namespace, Annotations: map[string]string{"keep": "metadata"}}, Data: map[string][]byte{"other": []byte("preserved")}}
	if err := c.Create(ctx, existing); err != nil {
		t.Fatal(err)
	}
	m := servermanifest.Manifest{GeneratedSecrets: []servermanifest.GeneratedSecret{{Name: "legacy", UseExactName: true, Length: 21}, {Name: "prefixed", Length: 15}}}
	if _, err := reconcileManifestSecrets(ctx, c, w, m); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
		t.Fatal(err)
	}
	if len(existing.Data["key"]) != 21 || string(existing.Data["other"]) != "preserved" || existing.Annotations["keep"] != "metadata" || !hasOwnerReference(existing, w) {
		t.Fatal("legacy repair changed unrelated contents or ownership")
	}
	if w.Status.GeneratedSecrets["prefixed"].Name != "wandb-prefixed" {
		t.Fatal("legacy prefixed naming changed")
	}
	// Migrating the declaration to the new shape must preserve the generated value.
	original := existing.DeepCopy()
	m.GeneratedSecrets = nil
	m.Secrets = []servermanifest.Secret{{Name: "legacy", UseExactName: true, Generated: &servermanifest.SecretGeneration{Length: 99}}}
	if _, err := reconcileManifestSecrets(ctx, c, w, m); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(existing), existing); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, existing) {
		t.Fatal("schema migration regenerated an existing key")
	}
}

func TestManifestSecretsRejectInvalidOrConflictingDeclarationsBeforeWrites(t *testing.T) {
	for _, problem := range []string{"missing name", "invalid name", "duplicate logical name", "duplicate resource name", "legacy collision"} {
		t.Run(problem, func(t *testing.T) {
			c, w, _ := migrationIdentityFixture(t)
			m := servermanifest.Manifest{Secrets: []servermanifest.Secret{{Name: "first"}}}
			switch problem {
			case "missing name":
				m.Secrets = append(m.Secrets, servermanifest.Secret{})
			case "invalid name":
				m.Secrets = append(m.Secrets, servermanifest.Secret{Name: "invalid/name"})
			case "duplicate logical name":
				m.Secrets = append(m.Secrets, servermanifest.Secret{Name: "first", UseExactName: true})
			case "duplicate resource name":
				m.Secrets = append(m.Secrets, servermanifest.Secret{Name: "wandb-first", UseExactName: true})
			case "legacy collision":
				m.GeneratedSecrets = []servermanifest.GeneratedSecret{{Name: "first"}}
			}
			if _, err := reconcileManifestSecrets(context.Background(), c, w, m); err == nil {
				t.Fatal("invalid declaration accepted")
			}
			secrets := &corev1.SecretList{}
			if err := c.List(context.Background(), secrets); err != nil {
				t.Fatal(err)
			}
			if len(secrets.Items) != 0 {
				t.Fatal("Secret created before all declarations were validated")
			}
		})
	}
}

func TestManifestSecretsDetachAndReadoptWithoutRotatingKeys(t *testing.T) {
	for _, policy := range []apiv2.OnDeletePolicy{apiv2.DetachOnDelete, "", apiv2.PurgeOnDelete} {
		t.Run(string(policy), func(t *testing.T) {
			c, w, _ := migrationIdentityFixture(t)
			ctx := context.Background()
			w.Spec.RetentionPolicy.OnDelete = policy
			if err := c.Update(ctx, w); err != nil {
				t.Fatal(err)
			}
			m := servermanifest.Manifest{Secrets: []servermanifest.Secret{{Name: "signer", UseExactName: true}, {Name: "session", Generated: &servermanifest.SecretGeneration{Length: 17}}}}
			if _, err := reconcileManifestSecrets(ctx, c, w, m); err != nil {
				t.Fatal(err)
			}
			signer := &corev1.Secret{}
			if err := c.Get(ctx, client.ObjectKey{Name: "signer", Namespace: w.Namespace}, signer); err != nil {
				t.Fatal(err)
			}
			signer.Data = map[string][]byte{"key.pem": []byte("existing signer key")}
			if err := c.Update(ctx, signer); err != nil {
				t.Fatal(err)
			}
			// Exercise legacy generated Secrets without the new inventory label too.
			session := &corev1.Secret{}
			if err := c.Get(ctx, client.ObjectKey{Name: "wandb-session", Namespace: w.Namespace}, session); err != nil {
				t.Fatal(err)
			}
			originalSession := string(session.Data["key"])
			delete(session.Labels, manifestSecretLabel)
			if err := c.Update(ctx, session); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := finalizeManifestSecrets(ctx, c, w); err != nil {
					t.Fatal(err)
				}
			}
			for _, secret := range []*corev1.Secret{signer, session} {
				if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
					t.Fatal(err)
				}
				if hasOwnerReference(secret, w) != (policy == apiv2.PurgeOnDelete) {
					t.Fatalf("unexpected ownership under %q: %+v", policy, secret.OwnerReferences)
				}
			}
			if string(signer.Data["key.pem"]) != "existing signer key" || string(session.Data["key"]) != originalSession {
				t.Fatal("cleanup changed key material")
			}
			if policy == apiv2.PurgeOnDelete {
				return
			}
			if err := c.Delete(ctx, w); err != nil {
				t.Fatal(err)
			}
			replacement := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: w.Name, Namespace: w.Namespace, UID: types.UID("replacement")}, Spec: w.Spec}
			if err := c.Create(ctx, replacement); err != nil {
				t.Fatal(err)
			}
			if _, err := reconcileManifestSecrets(ctx, c, replacement, m); err != nil {
				t.Fatal(err)
			}
			for _, secret := range []*corev1.Secret{signer, session} {
				if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
					t.Fatal(err)
				}
				if !hasOwnerReference(secret, replacement) || hasOwnerReference(secret, w) {
					t.Fatal("ownership not transferred to replacement CR")
				}
			}
			if string(signer.Data["key.pem"]) != "existing signer key" || string(session.Data["key"]) != originalSession {
				t.Fatal("adoption regenerated key material")
			}
		})
	}
}

func TestManifestSecretDetachPreservesUnrelatedOwnership(t *testing.T) {
	c, w, _ := migrationIdentityFixture(t)
	ctx := context.Background()
	owner := metav1.OwnerReference{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases", Name: w.Name, UID: w.UID}
	other := metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "other", UID: types.UID("other")}
	for _, secret := range []*corev1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: w.Namespace, Labels: map[string]string{manifestSecretLabel: "true"}, OwnerReferences: []metav1.OwnerReference{owner, other}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: w.Namespace, OwnerReferences: []metav1.OwnerReference{owner}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: w.Namespace, Labels: map[string]string{manifestSecretLabel: "true"}, OwnerReferences: []metav1.OwnerReference{other}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "outside", Namespace: "elsewhere", Labels: map[string]string{manifestSecretLabel: "true"}, OwnerReferences: []metav1.OwnerReference{owner}}},
	} {
		if err := c.Create(ctx, secret); err != nil {
			t.Fatal(err)
		}
	}
	if err := finalizeManifestSecrets(ctx, c, w); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shared", "unrelated", "foreign", "outside"} {
		secret := &corev1.Secret{}
		ns := w.Namespace
		if name == "outside" {
			ns = "elsewhere"
		}
		if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, secret); err != nil {
			t.Fatal(err)
		}
		expected := []metav1.OwnerReference{owner}
		if name == "shared" || name == "foreign" {
			expected = []metav1.OwnerReference{other}
		}
		if !reflect.DeepEqual(secret.OwnerReferences, expected) {
			t.Fatalf("%s changed unrelated ownership: %+v", name, secret.OwnerReferences)
		}
	}
}

func TestManifestSecretsRefuseForeignOwnerBeforeChangingData(t *testing.T) {
	c, w, _ := migrationIdentityFixture(t)
	ctx := context.Background()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: w.Namespace, OwnerReferences: []metav1.OwnerReference{{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases", Name: "elsewhere", UID: types.UID("foreign")}}}}
	if err := c.Create(ctx, secret); err != nil {
		t.Fatal(err)
	}
	original := secret.DeepCopy()
	m := servermanifest.Manifest{Secrets: []servermanifest.Secret{{Name: secret.Name, UseExactName: true, Generated: &servermanifest.SecretGeneration{}}}}
	if _, err := reconcileManifestSecrets(ctx, c, w, m); err == nil {
		t.Fatal("adopted foreign-owned Secret")
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, secret) {
		t.Fatal("foreign-owned Secret was modified")
	}
}

func TestManifestSecretDetachFailureKeepsCleanupFinalizer(t *testing.T) {
	c, w, _ := migrationIdentityFixture(t)
	ctx := context.Background()
	w.Finalizers = []string{CleanupFinalizer}
	w.Spec.Wandb.ManifestRepository = "file:///unavailable-during-deletion"
	if err := c.Update(ctx, w); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileManifestSecrets(ctx, c, w, servermanifest.Manifest{Secrets: []servermanifest.Secret{{Name: "signer", UseExactName: true}}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, w); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(w), w); err != nil {
		t.Fatal(err)
	}
	failing := interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
		if _, ok := obj.(*corev1.Secret); ok {
			return errors.New("detach write failed")
		}
		return c.Update(ctx, obj, opts...)
	}})
	if _, err := Reconcile(ctx, failing, record.NewFakeRecorder(10), w, telemetry.TelemetryRuntimeConfig{}); err == nil || !strings.Contains(err.Error(), "detach write failed") {
		t.Fatalf("expected detach failure before manifest lookup, got %v", err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(w), w); err != nil {
		t.Fatal(err)
	}
	if len(w.Finalizers) != 1 || w.Finalizers[0] != CleanupFinalizer {
		t.Fatal("cleanup finalizer removed before Secret detach succeeded")
	}
}
