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

// oidcManifestEnvs mirrors the server-manifest entries that wire the gorilla
// OIDC login env vars to spec.wandb.oidc.
func oidcManifestEnvs() []serverManifest.EnvVar {
	return []serverManifest.EnvVar{
		{Name: "GORILLA_OIDC_CLIENT_ID", Sources: []serverManifest.EnvSource{{Type: "oidc", Field: "clientId"}}},
		{Name: "GORILLA_OIDC_SECRET", Sources: []serverManifest.EnvSource{{Type: "oidc", Field: "clientSecret"}}},
		{Name: "GORILLA_OIDC_ISSUER", Sources: []serverManifest.EnvSource{{Type: "oidc", Field: "issuerUrl"}}},
		{Name: "GORILLA_AUTH_METHOD", Sources: []serverManifest.EnvSource{{Type: "oidc", Field: "authMethod"}}},
	}
}

func newTestClient(t *testing.T) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme)
}

func TestResolveEnvvarsOidcSourcePopulated(t *testing.T) {
	client := newTestClient(t).Build()

	sel := func(key string) corev1.SecretKeySelector {
		return corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-oidc"},
			Key:                  key,
		}
	}
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-dev-v2", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Wandb: apiv2.WandbAppSpec{
				OIDC: apiv2.OidcSpec{
					ClientId:     sel("client-id"),
					ClientSecret: sel("client-secret"),
					IssuerUrl:    sel("issuer-url"),
					AuthMethod:   sel("auth-method"),
				},
			},
		},
	}

	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, oidcManifestEnvs())
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}

	cases := map[string]string{
		"GORILLA_OIDC_CLIENT_ID": "client-id",
		"GORILLA_OIDC_SECRET":    "client-secret",
		"GORILLA_OIDC_ISSUER":    "issuer-url",
		"GORILLA_AUTH_METHOD":    "auth-method",
	}
	for envName, wantKey := range cases {
		env := mustFindEnvVar(t, resolved, envName)
		if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
			t.Fatalf("%s: expected a SecretKeyRef, got %+v", envName, env)
		}
		ref := env.ValueFrom.SecretKeyRef
		if ref.Name != "wandb-oidc" || ref.Key != wantKey {
			t.Fatalf("%s: unexpected secret ref %s/%s (want wandb-oidc/%s)", envName, ref.Name, ref.Key, wantKey)
		}
	}
}

func TestResolveEnvvarsOidcSourceUnconfigured(t *testing.T) {
	client := newTestClient(t).Build()

	// No OIDC configured: selectors have empty Name, so nothing should be emitted.
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb-dev-v2", Namespace: "default"}}

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
