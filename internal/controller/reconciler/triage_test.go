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
		ContainerName: "weave-trace",
		Args:          []string{"python", "-m", "weave_triage"},
		Env: []corev1.EnvVar{{
			Name:  "PYTHONPATH",
			Value: "/weave/src",
		}},
		Resources:      resources,
		TimeoutSeconds: 600,
		Actions: []serverManifest.TriageAction{{
			Name:        "default",
			Description: "Run all diagnostics",
			Args:        []string{"--verbose"},
		}},
	}

	resolved := resolveApplicationTriage(input)
	action := resolved.Actions[0]
	if resolved.ContainerName != "weave-trace" {
		t.Fatalf("containerName = %q", resolved.ContainerName)
	}
	if len(resolved.Args) != 3 || resolved.Args[0] != "python" {
		t.Fatalf("runner args = %#v", resolved.Args)
	}
	if resolved.Resources == nil || resolved.Resources.Requests.Memory().String() != "128Mi" {
		t.Fatalf("resources = %#v", resolved.Resources)
	}
	if resolved.TimeoutSeconds != 600 {
		t.Fatalf("timeoutSeconds = %d", resolved.TimeoutSeconds)
	}
	if action.Name != "default" || action.Description != "Run all diagnostics" ||
		len(action.Args) != 1 || action.Args[0] != "--verbose" {
		t.Fatalf("action = %#v", action)
	}

	input.Args[0] = "mutated"
	input.Actions[0].Args[0] = "mutated"
	resources.Requests[corev1.ResourceMemory] = resource.MustParse("1Gi")
	if resolved.Args[0] != "python" || action.Args[0] != "--verbose" ||
		resolved.Resources.Requests.Memory().String() != "128Mi" {
		t.Fatal("resolved action aliases mutable manifest data")
	}
}
