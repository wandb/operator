package reconciler

import (
	"context"
	"testing"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveEnvvarsCustomResourceRetention(t *testing.T) {
	retentionPeriod := metav1.Duration{Duration: 168 * time.Hour}
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
	}
	wandb.Spec.Wandb.Retention = &apiv2.RetentionSpec{
		ArtifactGarbageCollection: true,
		DataRetentionPeriod:       &retentionPeriod,
	}

	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{
			name:     "GORILLA_ARTIFACT_GC_ENABLED",
			field:    "spec.wandb.retention.artifactGarbageCollection",
			expected: "true",
		},
		{
			name:     "GORILLA_DATA_RETENTION_PERIOD",
			field:    "spec.wandb.retention.dataRetentionPeriod",
			expected: "168h0m0s",
		},
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
		if env.Value != tc.expected {
			t.Errorf("expected %s=%q, got %q", tc.name, tc.expected, env.Value)
		}
		if env.ValueFrom != nil {
			t.Errorf("expected %s to be a plain value, got valueFrom %+v", tc.name, env.ValueFrom)
		}
	}
}
