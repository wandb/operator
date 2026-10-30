package preflight

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
)

func secretRef(name, key string) *apiv2.ValueOrSecret {
	return &apiv2.ValueOrSecret{ValueFrom: &apiv2.SecretValueSource{SecretKeyRef: &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key,
	}}}
}

func TestRunProxyCheck(t *testing.T) {
	literal := func(v string) *apiv2.ValueOrSecret { return &apiv2.ValueOrSecret{Value: v} }
	tests := []struct {
		name     string
		proxy    apiv2.ProxySpec
		secret   string
		want     Outcome
		contains string
	}{
		{name: "literal pass", proxy: apiv2.ProxySpec{HTTPProxy: literal("http://proxy:3128"), HTTPSProxy: literal("http://proxy:3128"), NoProxy: []string{"internal.example"}}, want: OutcomePass},
		{name: "bad scheme", proxy: apiv2.ProxySpec{HTTPSProxy: literal("socks5://proxy:1080")}, want: OutcomeFail, contains: "scheme"},
		{name: "no host", proxy: apiv2.ProxySpec{HTTPProxy: literal("http://")}, want: OutcomeFail, contains: "no host"},
		{name: "literal credentials", proxy: apiv2.ProxySpec{HTTPProxy: literal("http://u:p@proxy:3128")}, want: OutcomeFail, contains: "credentials"},
		{name: "secret credentials allowed", proxy: apiv2.ProxySpec{HTTPProxy: secretRef("proxy", "url")}, secret: "http://u:p@proxy:3128", want: OutcomePass},
		{name: "secret not a url", proxy: apiv2.ProxySpec{HTTPProxy: secretRef("proxy", "url")}, secret: "http://u:p@[::1", want: OutcomeFail, contains: "not a valid URL"},
		{name: "secret empty", proxy: apiv2.ProxySpec{HTTPProxy: secretRef("proxy", "url")}, want: OutcomeFail, contains: "empty"},
		{name: "noProxy comma", proxy: apiv2.ProxySpec{HTTPProxy: literal("http://proxy:3128"), NoProxy: []string{"a,b"}}, want: OutcomeFail, contains: "noProxy[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolve := func(_ context.Context, v apiv2.ValueOrSecret) (string, error) {
				if v.Value != "" {
					return v.Value, nil
				}
				return tt.secret, nil
			}
			got := RunProxyCheck(context.Background(), &tt.proxy, Env{Resolve: resolve})
			if got.Outcome != tt.want || !strings.Contains(got.Message, tt.contains) {
				t.Errorf("got %s %q, want %s containing %q", got.Outcome, got.Message, tt.want, tt.contains)
			}
			if strings.Contains(got.Message, "u:p") {
				t.Errorf("message leaks proxy credentials: %q", got.Message)
			}
		})
	}
}

// A caller that can't read Secrets (wsm --offline) gets skipped, not unknown, so it doesn't fail the run.
func TestRunProxyCheck_SecretUnavailable(t *testing.T) {
	resolve := func(context.Context, apiv2.ValueOrSecret) (string, error) {
		return "", fmt.Errorf("secret proxy: %w", ErrSecretUnavailable)
	}
	proxy := &apiv2.ProxySpec{HTTPProxy: secretRef("proxy", "url")}
	if got := RunProxyCheck(context.Background(), proxy, Env{Resolve: resolve}); got.Outcome != OutcomeSkipped {
		t.Errorf("outcome = %s, want skipped", got.Outcome)
	}

	// The unreadable field is skipped, but a broken literal beside it still fails.
	offline := func(_ context.Context, v apiv2.ValueOrSecret) (string, error) {
		if v.Value != "" {
			return v.Value, nil
		}
		return "", ErrSecretUnavailable
	}
	mixed := &apiv2.ProxySpec{HTTPProxy: secretRef("proxy", "url"), HTTPSProxy: &apiv2.ValueOrSecret{Value: "ftp://proxy"}}
	if got := RunProxyCheck(context.Background(), mixed, Env{Resolve: offline}); got.Outcome != OutcomeFail || !strings.Contains(got.Message, "httpsProxy") {
		t.Errorf("mixed: got %s %q, want fail naming httpsProxy", got.Outcome, got.Message)
	}
}

func testCertPEM(t *testing.T, cn string, notBefore, notAfter time.Time) string {
	t.Helper()
	return testCertPEMWith(t, cn, notBefore, notAfter, true)
}

func testCertPEMWith(t *testing.T, cn string, notBefore, notAfter time.Time, isCA bool) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: cn},
		NotBefore: notBefore, NotAfter: notAfter, IsCA: isCA, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestRunCustomCACertsCheck(t *testing.T) {
	now := time.Now()
	valid := testCertPEM(t, "corp-root", now.Add(-time.Hour), now.Add(24*time.Hour))
	expired := testCertPEM(t, "old-root", now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	keyPEM := "-----BEGIN PRIVATE KEY-----\nc2VjcmV0\n-----END PRIVATE KEY-----\n"

	tests := []struct {
		name     string
		certs    []string
		want     Outcome
		contains string
	}{
		{name: "valid", certs: []string{valid}, want: OutcomePass, contains: "1 certificates"},
		{name: "bundle with a retired cert", certs: []string{valid + expired}, want: OutcomePass, contains: "old-root"},
		{name: "only expired", certs: []string{valid, expired}, want: OutcomeFail, contains: "customCACerts[1] has no currently valid"},
		{name: "not pem", certs: []string{"not a certificate"}, want: OutcomeFail, contains: "no PEM certificate"},
		{name: "private key pasted", certs: []string{keyPEM}, want: OutcomeFail, contains: `"PRIVATE KEY" block`},
		{name: "corrupt certificate", certs: []string{"-----BEGIN CERTIFICATE-----\nbm9wZQ==\n-----END CERTIFICATE-----\n"}, want: OutcomeFail, contains: "does not parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RunCustomCACertsCheck(context.Background(), &tt.certs, Env{})
			if got.Outcome != tt.want || !strings.Contains(got.Message, tt.contains) {
				t.Errorf("got %s %q, want %s containing %q", got.Outcome, got.Message, tt.want, tt.contains)
			}
			if strings.Contains(got.Message, "c2VjcmV0") {
				t.Errorf("message leaks key material: %q", got.Message)
			}
		})
	}
}

func TestRunServerManifestCheck(t *testing.T) {
	dir, err := filepath.Abs("../../hack/testing-manifests/server-manifest")
	if err != nil {
		t.Fatal(err)
	}
	repo := "file://" + dir
	spec := func(version string) *apiv2.WeightsAndBiasesSpec {
		return &apiv2.WeightsAndBiasesSpec{Wandb: apiv2.WandbAppSpec{ManifestRepository: repo, Version: version}}
	}
	tests := []struct {
		name     string
		version  string
		operator string
		want     Outcome
		contains string
	}{
		{name: "published", version: "0.85.1", want: OutcomePass, contains: "found"},
		{name: "unpublished", version: "0.0.1", want: OutcomeFail, contains: "not published"},
		{name: "latest", version: "latest", want: OutcomeFail, contains: "pinned"},
		{name: "operator satisfies", version: "0.85.1", operator: "2.1.0", want: OutcomePass, contains: "satisfies ^2.0.0"},
		{name: "operator beta satisfies", version: "0.85.1", operator: "2.0.0-beta.9", want: OutcomePass, contains: "satisfies"},
		{name: "operator too old", version: "0.85.1", operator: "1.22.0", want: OutcomeFail, contains: "requires operator ^2.0.0"},
		{name: "operator not semver", version: "0.85.1", operator: "main", want: OutcomeUnknown, contains: "not semver"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RunServerManifestCheck(context.Background(), spec(tt.version), Env{OperatorVersion: tt.operator})
			if got.Outcome != tt.want || !strings.Contains(got.Message, tt.contains) {
				t.Errorf("got %s %q, want %s containing %q", got.Outcome, got.Message, tt.want, tt.contains)
			}
		})
	}
}

func TestEffectiveManifestRepository(t *testing.T) {
	tests := []struct {
		spec apiv2.WeightsAndBiasesSpec
		want string
	}{
		{apiv2.WeightsAndBiasesSpec{}, apiv2.DefaultManifestRepository},
		{apiv2.WeightsAndBiasesSpec{Global: apiv2.GlobalSpec{ImageRegistry: "registry.corp"}}, apiv2.ManifestRepositoryFor("registry.corp")},
		{apiv2.WeightsAndBiasesSpec{Wandb: apiv2.WandbAppSpec{ManifestRepository: "registry.corp/manifests"}}, "oci://registry.corp/manifests"},
	}
	for _, tt := range tests {
		if got := effectiveManifestRepository(&tt.spec); got != tt.want {
			t.Errorf("effectiveManifestRepository(%+v) = %q, want %q", tt.spec, got, tt.want)
		}
	}
}

// pem.Decode alone silently skips what follows the last complete block; every BEGIN must decode.
func TestRunCustomCACertsCheck_StrictBlocks(t *testing.T) {
	now := time.Now()
	valid := testCertPEM(t, "corp-root", now.Add(-time.Hour), now.Add(24*time.Hour))
	leaf := testCertPEMWith(t, "minio.internal", now.Add(-time.Hour), now.Add(24*time.Hour), false)

	tests := []struct {
		name     string
		entry    string
		want     Outcome
		contains string
	}{
		{name: "truncated block after a valid one", entry: valid + "-----BEGIN CERTIFICATE-----\nMIIB\n", want: OutcomeFail, contains: "block 2 is malformed"},
		{name: "key fragment after a valid one", entry: valid + "-----BEGIN PRIVATE KEY-----\nc2VjcmV0\n", want: OutcomeFail, contains: "block 2 is malformed"},
		{name: "stray END before the first block", entry: "-----END CERTIFICATE-----\n" + valid, want: OutcomeFail, contains: "END marker without a BEGIN"},
		{name: "stray END after a valid block", entry: valid + "-----END CERTIFICATE-----\n", want: OutcomeFail, contains: "malformed"},
		{name: "leaf certificate", entry: leaf, want: OutcomeFail, contains: `"minio.internal") is not a CA`},
		{name: "unknown label isn't echoed", entry: "-----BEGIN hunter2-password-----\nc2VjcmV0\n-----END hunter2-password-----\n", want: OutcomeFail, contains: "unrecognized block"},
		{name: "comments between blocks are fine", entry: "# Issuer: Corp Root\n" + valid + "\nsubject=CN=corp-root\n" + valid, want: OutcomePass, contains: "2 certificates"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certs := []string{tt.entry}
			got := RunCustomCACertsCheck(context.Background(), &certs, Env{})
			if got.Outcome != tt.want || !strings.Contains(got.Message, tt.contains) {
				t.Errorf("got %s %q, want %s containing %q", got.Outcome, got.Message, tt.want, tt.contains)
			}
			for _, secret := range []string{"hunter2", "c2VjcmV0"} {
				if strings.Contains(got.Message, secret) {
					t.Errorf("message leaks %q: %q", secret, got.Message)
				}
			}
		})
	}
}

func TestComparableOperatorVersion(t *testing.T) {
	tests := []struct {
		constraint, operator string
		want                 bool
	}{
		{"^2.0.0", "2.0.0-beta.9", true},      // stable constraint: betas of the release count
		{"^2.0.0", "1.22.0", false},           // unchanged for stable versions
		{">=2.0.0-rc.5", "2.0.0-rc.1", false}, // prerelease bound: rc.1 precedes rc.5
		{">=2.0.0-rc.5", "2.0.0-rc.6", true},
		{">=2.0.0-rc.5", "2.0.0", true},
		{"1.2 - 1.4", "1.3.0-beta.1", true}, // a hyphen range isn't a prerelease bound
	}
	for _, tt := range tests {
		c, err := semver.NewConstraint(tt.constraint)
		if err != nil {
			t.Fatal(err)
		}
		v := semver.MustParse(tt.operator)
		if got := c.Check(comparableOperatorVersion(v, tt.constraint)); got != tt.want {
			t.Errorf("%s satisfies %q = %v, want %v", tt.operator, tt.constraint, got, tt.want)
		}
	}
}

func TestRunServerManifestCheck_RedactsRepositoryCredentials(t *testing.T) {
	if got := redactRepository("oci://robot:s3cret@registry.corp/wandb/server-manifest"); got != "oci://registry.corp/wandb/server-manifest" {
		t.Errorf("redactRepository = %q", got)
	}

	// Nothing listens on port 1, so the pull fails and the error path formats the repository.
	spec := &apiv2.WeightsAndBiasesSpec{Wandb: apiv2.WandbAppSpec{
		ManifestRepository: "oci://robot:s3cret@127.0.0.1:1/wandb/server-manifest", Version: "0.85.1",
	}}
	got := RunServerManifestCheck(context.Background(), spec, Env{})
	if got.Outcome == OutcomePass {
		t.Fatalf("outcome = pass, want a failure from the unreachable registry")
	}
	for _, secret := range []string{"s3cret", "robot"} {
		if strings.Contains(got.Message, secret) {
			t.Errorf("message leaks %q: %q", secret, got.Message)
		}
	}
}
