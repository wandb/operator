package v2

import (
	"strings"
	"testing"

	appsv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
)

func wandbWithProxy(proxy *appsv2.ProxySpec) *appsv2.WeightsAndBiases {
	wandb := &appsv2.WeightsAndBiases{}
	wandb.Spec.Global.Proxy = proxy
	return wandb
}

func TestValidateProxySpec(t *testing.T) {
	secretRef := &appsv2.ProxyValueSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "egress-proxy"},
			Key:                  "httpsProxy",
		},
	}
	cases := []struct {
		name    string
		proxy   *appsv2.ProxySpec
		wantErr string // substring; "" = accept
	}{
		{"nil proxy", nil, ""},
		{"literal http url", &appsv2.ProxySpec{HTTPProxy: &appsv2.ProxyValue{Value: "http://proxy.corp:3128"}}, ""},
		{"secret-backed https", &appsv2.ProxySpec{HTTPSProxy: &appsv2.ProxyValue{ValueFrom: secretRef}}, ""},
		{"noProxy extras ok", &appsv2.ProxySpec{HTTPProxy: &appsv2.ProxyValue{Value: "http://p:3128"}, NoProxy: []string{"internal.example.com", "10.0.0.0/8"}}, ""},
		{"both value and valueFrom", &appsv2.ProxySpec{HTTPProxy: &appsv2.ProxyValue{Value: "http://p:3128", ValueFrom: secretRef}}, "exactly one"},
		{"neither value nor valueFrom", &appsv2.ProxySpec{HTTPProxy: &appsv2.ProxyValue{}}, "one of value or valueFrom is required"},
		{"userinfo in literal", &appsv2.ProxySpec{HTTPProxy: &appsv2.ProxyValue{Value: "http://user:pass@proxy:3128"}}, "must not contain credentials"},
		{"bad scheme", &appsv2.ProxySpec{HTTPProxy: &appsv2.ProxyValue{Value: "socks5://proxy:1080"}}, "scheme must be http or https"},
		{"comma in noProxy", &appsv2.ProxySpec{HTTPProxy: &appsv2.ProxyValue{Value: "http://p:3128"}, NoProxy: []string{"a,b"}}, "must not contain commas"},
		{"empty noProxy entry", &appsv2.ProxySpec{HTTPProxy: &appsv2.ProxyValue{Value: "http://p:3128"}, NoProxy: []string{""}}, "must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateProxySpec(wandbWithProxy(tc.proxy))
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("expected no errors, got %v", errs)
				}
				return
			}
			if len(errs) == 0 || !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, errs)
			}
		})
	}
}
