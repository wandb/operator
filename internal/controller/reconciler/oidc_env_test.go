package reconciler

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func oidcTestCR() *apiv2.WeightsAndBiases {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
	}
	wandb.Spec.Wandb.OIDC = apiv2.OidcSpec{
		ClientId:      apiv2.ValueFromSecret("my-oidc", "clientId", false),
		SessionLength: "48h",
	}
	return wandb
}

func TestResolveCRFieldSecretSelector(t *testing.T) {
	wandb := oidcTestCR()

	t.Run("returns selector for a secret-backed field", func(t *testing.T) {
		sel, ok := resolveCRFieldSecretSelector(wandb, "spec.wandb.oidc.clientId")
		if !ok {
			t.Fatalf("expected clientId to resolve as a secret selector")
		}
		if sel.Name != "my-oidc" || sel.Key != "clientId" {
			t.Fatalf("unexpected selector: %+v", sel)
		}
	})

	t.Run("not found when selector is unset", func(t *testing.T) {
		if _, ok := resolveCRFieldSecretSelector(wandb, "spec.wandb.oidc.clientSecret"); ok {
			t.Fatalf("expected unset clientSecret to be treated as not found")
		}
	})

	t.Run("not found for a plain string field", func(t *testing.T) {
		if _, ok := resolveCRFieldSecretSelector(wandb, "spec.wandb.oidc.sessionLength"); ok {
			t.Fatalf("expected string field not to resolve as a secret selector")
		}
	})

	t.Run("not found for a missing path", func(t *testing.T) {
		if _, ok := resolveCRFieldSecretSelector(wandb, "spec.wandb.nope"); ok {
			t.Fatalf("expected missing path to be not found")
		}
	})
}

func TestResolveCRFieldEnvValue(t *testing.T) {
	wandb := oidcTestCR()

	t.Run("returns a string unchanged", func(t *testing.T) {
		value, ok := resolveCRFieldEnvValue(wandb, "spec.wandb.oidc.sessionLength")
		if !ok || value != "48h" {
			t.Fatalf("expected session length to resolve to %q, got %q (found: %t)", "48h", value, ok)
		}
	})

	for _, value := range []bool{true, false} {
		t.Run(strconv.FormatBool(value), func(t *testing.T) {
			wandb.Spec.Wandb.BucketProxy = value

			resolved, ok := resolveCRFieldEnvValue(wandb, "spec.wandb.bucketProxy")
			if !ok || resolved != strconv.FormatBool(value) {
				t.Fatalf("expected bucketProxy to resolve to %q, got %q (found: %t)", strconv.FormatBool(value), resolved, ok)
			}
		})
	}

	t.Run("rejects a structured value", func(t *testing.T) {
		if _, ok := resolveCRFieldEnvValue(wandb, "spec.wandb.oidc.clientId"); ok {
			t.Fatal("expected a secret selector not to resolve as a literal env value")
		}
	})

	t.Run("rejects a missing path", func(t *testing.T) {
		if _, ok := resolveCRFieldEnvValue(wandb, "spec.wandb.nope"); ok {
			t.Fatal("expected a missing path not to resolve as a literal env value")
		}
	})
}

func TestResolveEnvvarsCustomResourceOIDC(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	wandb := oidcTestCR()

	envs := []serverManifest.EnvVar{
		{
			Name: "GORILLA_OIDC_CLIENT_ID",
			Sources: []serverManifest.EnvSource{
				{Type: "custom-resource", Name: "oidc", Field: "spec.wandb.oidc.clientId"},
			},
		},
		{
			Name: "GORILLA_SESSION_LENGTH",
			Sources: []serverManifest.EnvSource{
				{Type: "custom-resource", Name: "oidc", Field: "spec.wandb.oidc.sessionLength"},
			},
			DefaultValue: "720h",
		},
	}

	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, envs)
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}

	clientID := mustFindEnvVar(t, resolved, "GORILLA_OIDC_CLIENT_ID")
	if clientID.ValueFrom == nil || clientID.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("expected GORILLA_OIDC_CLIENT_ID to resolve from a secret key ref, got %+v", clientID)
	}
	if clientID.ValueFrom.SecretKeyRef.Name != "my-oidc" || clientID.ValueFrom.SecretKeyRef.Key != "clientId" {
		t.Fatalf("unexpected client id secret ref: %+v", clientID.ValueFrom.SecretKeyRef)
	}

	sessionLen := mustFindEnvVar(t, resolved, "GORILLA_SESSION_LENGTH")
	if sessionLen.ValueFrom != nil {
		t.Fatalf("expected GORILLA_SESSION_LENGTH to be a plain value, got valueFrom %+v", sessionLen.ValueFrom)
	}
	if sessionLen.Value != "48h" {
		t.Fatalf("unexpected session length value: %q", sessionLen.Value)
	}
}

func TestResolveEnvvarsCustomResourceBoolean(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	envs := []serverManifest.EnvVar{
		{
			Name: "BUCKET_PROXY",
			Sources: []serverManifest.EnvSource{
				{Type: "custom-resource", Name: "bucket-proxy", Field: "spec.wandb.bucketProxy"},
			},
		},
	}

	for _, value := range []bool{true, false} {
		t.Run(strconv.FormatBool(value), func(t *testing.T) {
			wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"}}
			wandb.Spec.Wandb.BucketProxy = value

			resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, envs)
			if err != nil {
				t.Fatalf("resolveEnvvars returned error: %v", err)
			}

			bucketProxy := mustFindEnvVar(t, resolved, "BUCKET_PROXY")
			if bucketProxy.Value != strconv.FormatBool(value) {
				t.Fatalf("expected BUCKET_PROXY=%q, got %q", strconv.FormatBool(value), bucketProxy.Value)
			}
		})
	}
}

func TestResolveEnvvarsManifestBucketProxyBooleans(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	manifest, err := serverManifest.GetServerManifest(
		context.Background(),
		"file://"+filepath.Join(repoRoot, "hack", "testing-manifests", "server-manifest"),
		"0.83.0-clickhouse-keeper.2",
		nil,
	)
	if err != nil {
		t.Fatalf("load test server manifest: %v", err)
	}
	apiApp, ok := manifest.Applications["api"]
	if !ok {
		t.Fatal("test server manifest does not define the api application")
	}

	client := fake.NewClientBuilder().Build()
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"}}
	wandb.Spec.Wandb.BucketProxy = true
	resolved, err := resolveEnvvars(context.Background(), client, wandb, manifest, apiApp.CommonEnvs, apiApp.Env)
	if err != nil {
		t.Fatalf("resolve api environment: %v", err)
	}

	for _, name := range []string{
		"BUCKET_PROXY",
		"GORILLA_GLUE_FILE_STORE_IS_PROXIED",
		"GORILLA_FILE_STORE_IS_PROXIED",
	} {
		if value := mustFindEnvVar(t, resolved, name).Value; value != "true" {
			t.Errorf("expected %s=true, got %q", name, value)
		}
	}
}

func TestResolveEnvvarsSessionLengthDefault(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	// CR with no OIDC/session config set.
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"}}

	envs := []serverManifest.EnvVar{
		{
			Name: "GORILLA_SESSION_LENGTH",
			Sources: []serverManifest.EnvSource{
				{Type: "custom-resource", Name: "oidc", Field: "spec.wandb.oidc.sessionLength"},
			},
			DefaultValue: "720h",
		},
	}

	resolved, err := resolveEnvvars(context.Background(), client, wandb, serverManifest.Manifest{}, nil, envs)
	if err != nil {
		t.Fatalf("resolveEnvvars returned error: %v", err)
	}

	sessionLen := mustFindEnvVar(t, resolved, "GORILLA_SESSION_LENGTH")
	if sessionLen.Value != "720h" {
		t.Fatalf("expected session length to fall back to default, got %q", sessionLen.Value)
	}
}
