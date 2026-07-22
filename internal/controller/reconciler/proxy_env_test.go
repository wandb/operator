package reconciler

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	apiv2 "github.com/wandb/operator/api/v2"
)

func envByName(vars []corev1.EnvVar, name string) (corev1.EnvVar, bool) {
	for _, v := range vars {
		if v.Name == name {
			return v, true
		}
	}
	return corev1.EnvVar{}, false
}

func TestComputeNoProxyBaselineAndExtras(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	got := computeNoProxy([]string{"internal.example.com", "127.0.0.1"})
	parts := strings.Split(got, ",")
	for _, want := range []string{
		"localhost", "127.0.0.1", "::1", ".svc", ".svc.cluster.local",
		".cluster.local", "kubernetes.default.svc", "10.96.0.1", "internal.example.com",
	} {
		found := false
		for _, p := range parts {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Errorf("NO_PROXY missing %q: %s", want, got)
		}
	}
	// 127.0.0.1 appears once despite being both baseline and an extra.
	count := 0
	for _, p := range parts {
		if p == "127.0.0.1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("127.0.0.1 duplicated (%d): %s", count, got)
	}
}

func TestComputeNoProxyNoAPIServerHost(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	got := computeNoProxy(nil)
	if strings.Contains(got, ",,") || strings.HasPrefix(got, ",") || strings.HasSuffix(got, ",") {
		t.Errorf("blank entry in NO_PROXY: %q", got)
	}
	if !strings.Contains(got, ".svc.cluster.local") {
		t.Errorf("baseline missing suffix: %q", got)
	}
}

func TestProxyEnvVarsLiteral(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	proxy := &apiv2.ProxySpec{
		HTTPProxy:  &apiv2.ProxyValue{Value: "http://proxy:3128"},
		HTTPSProxy: &apiv2.ProxyValue{Value: "http://proxy:3128"},
		NoProxy:    []string{"wandb.localhost"},
	}
	vars := proxyEnvVars(proxy)
	// Six vars: HTTP_PROXY/HTTPS_PROXY/NO_PROXY + lowercase.
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"} {
		v, ok := envByName(vars, name)
		if !ok {
			t.Fatalf("missing env var %q", name)
		}
		if v.ValueFrom != nil {
			t.Errorf("%s should be a literal, got ValueFrom", name)
		}
	}
	if v, _ := envByName(vars, "HTTP_PROXY"); v.Value != "http://proxy:3128" {
		t.Errorf("HTTP_PROXY = %q", v.Value)
	}
	np, _ := envByName(vars, "NO_PROXY")
	if !strings.Contains(np.Value, "wandb.localhost") || !strings.Contains(np.Value, "10.96.0.1") || !strings.Contains(np.Value, ".svc.cluster.local") {
		t.Errorf("NO_PROXY missing computed baseline/extras: %q", np.Value)
	}
}

func TestProxyEnvVarsValueFromStaysSecretRef(t *testing.T) {
	proxy := &apiv2.ProxySpec{
		HTTPSProxy: &apiv2.ProxyValue{
			ValueFrom: &apiv2.ProxyValueSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "egress-proxy"},
					Key:                  "httpsProxy",
				},
			},
		},
	}
	vars := proxyEnvVars(proxy)
	// httpsProxy only: no HTTP_PROXY pair, but NO_PROXY still emitted.
	if _, ok := envByName(vars, "HTTP_PROXY"); ok {
		t.Errorf("HTTP_PROXY should be absent when only httpsProxy is set")
	}
	for _, name := range []string{"HTTPS_PROXY", "https_proxy"} {
		v, ok := envByName(vars, name)
		if !ok {
			t.Fatalf("missing %q", name)
		}
		// Credential-bearing values stay a SecretKeyRef, never a literal.
		if v.Value != "" || v.ValueFrom == nil || v.ValueFrom.SecretKeyRef == nil {
			t.Errorf("%s should be a SecretKeyRef, got %+v", name, v)
		}
		if v.ValueFrom.SecretKeyRef.Name != "egress-proxy" || v.ValueFrom.SecretKeyRef.Key != "httpsProxy" {
			t.Errorf("%s secret ref wrong: %+v", name, v.ValueFrom.SecretKeyRef)
		}
	}
	if _, ok := envByName(vars, "NO_PROXY"); !ok {
		t.Errorf("NO_PROXY should be emitted whenever any proxy URL is set")
	}
}

func TestProxyEnvVarsNil(t *testing.T) {
	if proxyEnvVars(nil) != nil {
		t.Errorf("nil proxy should yield no env vars")
	}
	// A ProxySpec with no URLs set emits nothing (NO_PROXY only rides with a proxy).
	if got := proxyEnvVars(&apiv2.ProxySpec{}); len(got) != 0 {
		t.Errorf("empty proxy spec should yield no env vars, got %v", got)
	}
}

func TestApplyProxyToWorkload(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	base := []corev1.EnvVar{{Name: "EXISTING", Value: "x"}, {Name: "HTTP_PROXY", Value: "manifest-value"}}

	// No proxy spec: unchanged.
	wandbNoProxy := &apiv2.WeightsAndBiases{}
	if got := applyProxyToWorkload(wandbNoProxy, base); len(got) != len(base) {
		t.Fatalf("no-proxy workload should be unchanged, got %v", got)
	}

	// With proxy: appends missing vars, does not clobber an existing HTTP_PROXY
	// (appendMissing semantics — legacy/manifest precedence handled elsewhere).
	wandb := &apiv2.WeightsAndBiases{}
	wandb.Spec.Global.Proxy = &apiv2.ProxySpec{HTTPProxy: &apiv2.ProxyValue{Value: "http://proxy:3128"}}
	got := applyProxyToWorkload(wandb, base)
	if v, _ := envByName(got, "HTTP_PROXY"); v.Value != "manifest-value" {
		t.Errorf("existing HTTP_PROXY should be preserved by appendMissing, got %q", v.Value)
	}
	if _, ok := envByName(got, "NO_PROXY"); !ok {
		t.Errorf("NO_PROXY should have been appended")
	}
}
