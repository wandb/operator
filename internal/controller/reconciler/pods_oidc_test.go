package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func oidcManifestEnvs() []serverManifest.EnvVar {
	return []serverManifest.EnvVar{
		{Name: "GORILLA_OIDC_CLIENT_ID", Sources: []serverManifest.EnvSource{{Type: "custom-resource", Field: "spec.wandb.oidc.clientId"}}},
		{Name: "GORILLA_OIDC_SECRET", Sources: []serverManifest.EnvSource{{Type: "custom-resource", Field: "spec.wandb.oidc.clientSecret"}}},
		{Name: "GORILLA_OIDC_ISSUER", Sources: []serverManifest.EnvSource{{Type: "custom-resource", Field: "spec.wandb.oidc.issuerUrl"}}},
		{Name: "GORILLA_AUTH_METHOD", Sources: []serverManifest.EnvSource{{Type: "custom-resource", Field: "spec.wandb.oidc.authMethod"}}},
	}
}

// oidcCR returns a CR with all four OIDC fields referencing keys in the "wandb-oidc" secret.
func oidcCR() *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Wandb: apiv2.WandbAppSpec{
				OIDC: apiv2.OidcSpec{
					ClientId:     corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-oidc"}, Key: "client-id"},
					ClientSecret: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-oidc"}, Key: "client-secret"},
					IssuerUrl:    corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-oidc"}, Key: "issuer-url"},
					AuthMethod:   corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-oidc"}, Key: "auth-method"},
				},
			},
		},
	}
}

func TestResolveEnvvarsOidcSourcePopulated(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	wandb := oidcCR()

	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, oidcManifestEnvs())
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}

	cases := []struct{ env, wantKey string }{
		{"GORILLA_OIDC_CLIENT_ID", "client-id"},
		{"GORILLA_OIDC_SECRET", "client-secret"},
		{"GORILLA_OIDC_ISSUER", "issuer-url"},
		{"GORILLA_AUTH_METHOD", "auth-method"},
	}
	for _, c := range cases {
		env := mustFindEnvVar(t, resolved, c.env)
		if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
			t.Fatalf("%s: expected a SecretKeyRef, got %+v", c.env, env)
		}
		ref := env.ValueFrom.SecretKeyRef
		if ref.Name != "wandb-oidc" || ref.Key != c.wantKey {
			t.Fatalf("%s: unexpected secret ref %s/%s (want wandb-oidc/%s)", c.env, ref.Name, ref.Key, c.wantKey)
		}
	}
}

func TestResolveEnvvarsOidcSourceUnconfigured(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	// No OIDC configured: selectors have empty Name, so nothing should be emitted.
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"}}

	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, oidcManifestEnvs())
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}

	for _, name := range []string{"GORILLA_OIDC_CLIENT_ID", "GORILLA_OIDC_SECRET", "GORILLA_OIDC_ISSUER", "GORILLA_AUTH_METHOD"} {
		for _, env := range resolved {
			if env.Name == name {
				t.Fatalf("expected %s to be omitted when OIDC is unconfigured, got %+v", name, env)
			}
		}
	}
}

// TestOidcManifestFieldPathsResolve fails if a manifest field path stops resolving
// against the CR types (e.g. a renamed CRD field) — catching an otherwise-silent break.
func TestOidcManifestFieldPathsResolve(t *testing.T) {
	wandb := oidcCR()
	for _, env := range oidcManifestEnvs() {
		for _, src := range env.Sources {
			if _, ok := resolveCRFieldSecretSelector(wandb, src.Field); !ok {
				t.Errorf("manifest path %q (env %s) does not resolve to a SecretKeySelector — was a CRD field renamed?", src.Field, env.Name)
			}
		}
	}
}
