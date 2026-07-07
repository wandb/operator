package reconciler

import (
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
)

func testWeightsAndBiases() *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{}
}

func TestResolveContainersPreservesResolvedProbes(t *testing.T) {
	containers := resolveContainers(serverManifest.Application{
		Name: "worker",
		Image: serverManifest.ImageRef{
			Repository: "worker",
			Tag:        "test",
		},
		Containers: []serverManifest.ContainerSpec{
			{
				Name: "worker",
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						Exec: &corev1.ExecAction{Command: []string{"true"}},
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						TCPSocket: &corev1.TCPSocketAction{},
					},
				},
			},
		},
	}, testWeightsAndBiases(), nil, nil)

	if len(containers) != 1 {
		t.Fatalf("expected one container, got %d", len(containers))
	}
	if containers[0].LivenessProbe == nil || containers[0].LivenessProbe.Exec == nil {
		t.Fatalf("expected exec liveness probe to be preserved, got %+v", containers[0].LivenessProbe)
	}
	if containers[0].ReadinessProbe == nil || containers[0].ReadinessProbe.TCPSocket == nil {
		t.Fatalf("expected TCP readiness probe to be preserved, got %+v", containers[0].ReadinessProbe)
	}
}
