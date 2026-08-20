package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveEnvvarsCustomResourceSecurityFlags(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
	}
	wandb.Spec.Wandb.Security = apiv2.SecuritySpec{
		AllowUserTeamCreation:          true,
		DisableCodeSaving:              true,
		AllowAnonymousPublicProjects:   true,
		DisableSSOProvisioning:         true,
		InsecureAllowAPIKeyAdminAccess: true,
		HideUpgradeBanner:              true,
	}
	wandb.Spec.ObjectStore = map[string]apiv2.ObjectStoreSpec{
		apiv2.DefaultInstanceName: {BucketAttributionDisabled: true},
	}

	tests := []struct {
		name  string
		field string
	}{
		{"GORILLA_ALLOW_USER_TEAM_CREATION", "spec.wandb.security.allowUserTeamCreation"},
		{"GORILLA_DISABLE_CODE_SAVING", "spec.wandb.security.disableCodeSaving"},
		{"GORILLA_ALLOW_ANONYMOUS_PUBLIC_PROJECTS", "spec.wandb.security.allowAnonymousPublicProjects"},
		{"GORILLA_DISABLE_SSO_PROVISIONING", "spec.wandb.security.disableSSOProvisioning"},
		{"GORILLA_INSECURE_ALLOW_API_KEY_ADMIN_ACCESS", "spec.wandb.security.insecureAllowAPIKeyAdminAccess"},
		{"GORILLA_BUCKET_ATTRIBUTION_DISABLED", "spec.objectStore.default.bucketAttributionDisabled"},
		{"GORILLA_ONPREM_HIDE_UPGRADE_BANNER", "spec.wandb.security.hideUpgradeBanner"},
	}

	envs := make([]serverManifest.EnvVar, 0, len(tests))
	for _, tc := range tests {
		envs = append(envs, serverManifest.EnvVar{
			Name: tc.name,
			Sources: []serverManifest.EnvSource{{
				Type: "custom-resource", Field: tc.field,
			}},
		})
	}

	resolved, err := resolveEnvvars(
		context.Background(),
		fake.NewClientBuilder().Build(),
		wandb,
		serverManifest.Manifest{},
		nil,
		envs,
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range tests {
		env := mustFindEnvVar(t, resolved, tc.name)
		if env.Value != "true" {
			t.Errorf("expected %s=true, got %q", tc.name, env.Value)
		}
		if env.ValueFrom != nil {
			t.Errorf("expected %s to be a plain value, got valueFrom %+v", tc.name, env.ValueFrom)
		}
	}
}
