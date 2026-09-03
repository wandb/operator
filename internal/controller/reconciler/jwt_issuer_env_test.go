package reconciler

import (
	"context"
	"strings"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func jwtIssuerTestCR(enabled bool, crIssuer string) *apiv2.WeightsAndBiases {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
	}
	wandb.Spec.Wandb.ServiceAccount.ServiceAccountName = "wandb-app"
	wandb.Spec.Wandb.InternalServiceAuth = apiv2.InternalServiceAuth{
		Enabled:    ptr.To(enabled),
		OIDCIssuer: crIssuer,
	}
	return wandb
}

func resolveJWTIssuerMap(t *testing.T, wandb *apiv2.WeightsAndBiases) ([]string, error) {
	t.Helper()
	resolved, err := resolveEnvvars(
		context.Background(),
		fake.NewClientBuilder().Build(),
		wandb,
		serverManifest.Manifest{},
		nil,
		[]serverManifest.EnvVar{{
			Name:    "JWT_ISSUER_MAP",
			Sources: []serverManifest.EnvSource{{Type: "jwt-issuer-map"}},
		}},
	)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(resolved))
	for _, env := range resolved {
		if env.Name == "JWT_ISSUER_MAP" {
			values = append(values, env.Value)
		}
	}
	return values, nil
}

// TestJWTIssuerMapFailsWithoutIssuer pins the fail-closed contract: an issuer the
// API server did not stamp on the token 401s and panics the API, so an unknown
// issuer must stop the reconcile rather than fall back to a guess.
func TestJWTIssuerMapFailsWithoutIssuer(t *testing.T) {
	utils.SetServiceAccountIssuer("")

	_, err := resolveJWTIssuerMap(t, jwtIssuerTestCR(true, ""))
	if err == nil {
		t.Fatal("expected an error when neither the CR nor discovery supplies an issuer")
	}
	if got := err.Error(); !strings.Contains(got, serviceAccountIssuerUnknownReason) {
		t.Fatalf("expected the error to carry %q, got %q", serviceAccountIssuerUnknownReason, got)
	}
}

func TestJWTIssuerMapUsesDiscoveredIssuer(t *testing.T) {
	const discovered = "https://oidc.eks.us-east-1.amazonaws.com/id/ABC123"
	utils.SetServiceAccountIssuer(discovered)
	defer utils.SetServiceAccountIssuer("")

	values, err := resolveJWTIssuerMap(t, jwtIssuerTestCR(true, ""))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 {
		t.Fatalf("expected one JWT_ISSUER_MAP env var, got %d", len(values))
	}
	if !strings.Contains(values[0], discovered) {
		t.Fatalf("expected the discovered issuer in %q", values[0])
	}
	if !strings.Contains(values[0], "system:serviceaccount:default:wandb-app") {
		t.Fatalf("expected the CR's ServiceAccount in %q", values[0])
	}
}

// TestJWTIssuerMapCRIssuerWinsOverDiscovered guards the precedence the operator
// documents: an explicit CR value overrides whatever start-up discovery found.
func TestJWTIssuerMapCRIssuerWinsOverDiscovered(t *testing.T) {
	utils.SetServiceAccountIssuer("https://oidc.eks.us-east-1.amazonaws.com/id/DISCOVERED")
	defer utils.SetServiceAccountIssuer("")

	values, err := resolveJWTIssuerMap(t, jwtIssuerTestCR(true, "https://issuer.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || !strings.Contains(values[0], "https://issuer.example.com") {
		t.Fatalf("expected the CR issuer to win, got %v", values)
	}
	if strings.Contains(values[0], "DISCOVERED") {
		t.Fatalf("expected the discovered issuer to be ignored, got %q", values[0])
	}
}

// TestJWTIssuerMapSkippedWhenDisabled: with internal service auth off there is no
// token to validate, so a missing issuer must not fail the reconcile.
func TestJWTIssuerMapSkippedWhenDisabled(t *testing.T) {
	utils.SetServiceAccountIssuer("")

	values, err := resolveJWTIssuerMap(t, jwtIssuerTestCR(false, ""))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		if value != "" {
			t.Fatalf("expected no issuer map when internal service auth is disabled, got %q", value)
		}
	}
}
