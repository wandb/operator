package v2

import (
	"strings"
	"testing"

	appsv2 "github.com/wandb/operator/api/v2"
	"k8s.io/utils/ptr"
)

func wandbWithWatchtower(watchtower appsv2.WatchtowerSpec) *appsv2.WeightsAndBiases {
	wandb := &appsv2.WeightsAndBiases{}
	wandb.Spec.Watchtower = watchtower
	return wandb
}

func TestValidateWatchtowerSpec(t *testing.T) {
	cases := []struct {
		name       string
		watchtower appsv2.WatchtowerSpec
		wantErr    string // substring; "" = accept
	}{
		{
			// Watchtower is opt-in, so a CR that never mentions it must validate.
			name:       "disabled by default",
			watchtower: appsv2.WatchtowerSpec{},
		},
		{
			name:       "explicitly disabled ignores other fields",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(false), BasePath: "no-leading-slash"},
		},
		{
			// The common case: enable it and take the defaults. BasePath is optional
			// and resolves to /watchtower, so an unset value must be accepted.
			name:       "enabled with defaults",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(true)},
		},
		{
			name:       "explicit base path",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(true), BasePath: "/console"},
		},
		{
			name:       "custom base path",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(true), BasePath: "/admin"},
		},
		{
			name:       "base path without a leading slash",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(true), BasePath: "watchtower"},
			wantErr:    "must start with '/'",
		},
		{
			// "/" is the W&B frontend's own path; mounting Watchtower there would
			// shadow the app it exists to manage.
			name:       "base path of root",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(true), BasePath: "/"},
			wantErr:    "must not be '/'",
		},
		{
			name:       "auth service host port",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(true), AuthService: "api:8081"},
		},
		{
			name:       "auth service with a scheme",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(true), AuthService: "http://api:8081"},
			wantErr:    "bare host:port",
		},
		{
			name:       "auth service with a path",
			watchtower: appsv2.WatchtowerSpec{Install: ptr.To(true), AuthService: "api:8081/oidc"},
			wantErr:    "bare host:port",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateWatchtowerSpec(wandbWithWatchtower(tc.watchtower))

			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Fatalf("expected the spec to validate, got %v", errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got %v", tc.wantErr, errs.ToAggregate())
			}
		})
	}
}
