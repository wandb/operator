package reconciler

import (
	"testing"

	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestResolveApplicationTriageCopiesCompactAction(t *testing.T) {
	resources := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
	}
	input := &serverManifest.ApplicationTriage{
		Actions: map[string]serverManifest.TriageAction{
			"default": {
				ContainerName: "weave-trace",
				Args:          []string{"python", "-m", "weave_triage", "run-all", "--stream"},
				Env: []corev1.EnvVar{{
					Name:  "PYTHONPATH",
					Value: "/weave/src",
				}},
				Resources:      resources,
				TimeoutSeconds: 600,
			},
		},
	}

	resolved := resolveApplicationTriage(input)
	action := resolved.Actions["default"]
	if action.ContainerName != "weave-trace" {
		t.Fatalf("containerName = %q", action.ContainerName)
	}
	if len(action.Args) != 5 || action.Args[4] != "--stream" {
		t.Fatalf("args = %#v", action.Args)
	}
	if action.Resources == nil || action.Resources.Requests.Memory().String() != "128Mi" {
		t.Fatalf("resources = %#v", action.Resources)
	}
	if action.TimeoutSeconds != 600 {
		t.Fatalf("timeoutSeconds = %d", action.TimeoutSeconds)
	}

	input.Actions["default"].Args[0] = "mutated"
	resources.Requests[corev1.ResourceMemory] = resource.MustParse("1Gi")
	if action.Args[0] != "python" || action.Resources.Requests.Memory().String() != "128Mi" {
		t.Fatal("resolved action aliases mutable manifest data")
	}
}
