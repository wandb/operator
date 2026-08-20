package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveEnvvarsCustomResourceSlack(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"}}
	wandb.Spec.Wandb.Notifications = &apiv2.NotificationsSpec{}
	wandb.Spec.Wandb.Notifications.Slack = &apiv2.SlackSpec{
		ClientID:     emailTestSelector("slack", "client-id"),
		ClientSecret: emailTestSelector("slack", "client-secret"),
	}
	envs := []serverManifest.EnvVar{
		{
			Name: "GORILLA_SLACK_CLIENT_ID",
			Sources: []serverManifest.EnvSource{{
				Type: "custom-resource", Field: "spec.wandb.notifications.slack.clientId",
			}},
		},
		{
			Name: "GORILLA_SLACK_SECRET",
			Sources: []serverManifest.EnvSource{{
				Type: "custom-resource", Field: "spec.wandb.notifications.slack.clientSecret",
			}},
		},
	}

	resolved, err := resolveEnvvars(context.Background(), fake.NewClientBuilder().Build(), wandb, serverManifest.Manifest{}, nil, envs)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		key  string
	}{
		{"GORILLA_SLACK_CLIENT_ID", "client-id"},
		{"GORILLA_SLACK_SECRET", "client-secret"},
	} {
		env := mustFindEnvVar(t, resolved, tc.name)
		if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
			t.Fatalf("%s did not resolve to a SecretKeyRef: %+v", tc.name, env)
		}
		if env.ValueFrom.SecretKeyRef.Name != "slack" || env.ValueFrom.SecretKeyRef.Key != tc.key {
			t.Fatalf("unexpected %s selector: %+v", tc.name, env.ValueFrom.SecretKeyRef)
		}
	}
}
