package reconciler

import (
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func testWeightsAndBiases() *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{}
}

func TestResolveContainersDefaultsStartupProbeFromLiveness(t *testing.T) {
	containers := resolveContainers(serverManifest.Application{
		Name: "api",
		Image: serverManifest.ImageRef{
			Repository: "api",
			Tag:        "test",
		},
		Containers: []serverManifest.ContainerSpec{
			{
				Name: "api",
				Ports: []serverManifest.ContainerPort{
					{Name: "api", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/healthz",
							Host:   "127.0.0.1",
							Scheme: corev1.URISchemeHTTPS,
							HTTPHeaders: []corev1.HTTPHeader{
								{Name: "X-Probe", Value: "liveness"},
							},
						},
					},
				},
			},
		},
	}, testWeightsAndBiases(), nil, nil)

	if len(containers) != 1 {
		t.Fatalf("expected one container, got %d", len(containers))
	}
	startupProbe := containers[0].StartupProbe
	if startupProbe == nil || startupProbe.HTTPGet == nil {
		t.Fatalf("expected startup probe copied from liveness probe")
	}
	if startupProbe.HTTPGet.Path != "/healthz" {
		t.Fatalf("unexpected startup probe path: %s", startupProbe.HTTPGet.Path)
	}
	if startupProbe.HTTPGet.Host != "127.0.0.1" {
		t.Fatalf("unexpected startup probe host: %s", startupProbe.HTTPGet.Host)
	}
	if startupProbe.HTTPGet.Scheme != corev1.URISchemeHTTPS {
		t.Fatalf("unexpected startup probe scheme: %s", startupProbe.HTTPGet.Scheme)
	}
	if len(startupProbe.HTTPGet.HTTPHeaders) != 1 || startupProbe.HTTPGet.HTTPHeaders[0].Name != "X-Probe" {
		t.Fatalf("expected startup probe to preserve liveness HTTP headers: %+v", startupProbe.HTTPGet.HTTPHeaders)
	}
	if startupProbe.HTTPGet.Port != intstr.FromString("api") {
		t.Fatalf("expected startup probe port to default to named port, got %#v", startupProbe.HTTPGet.Port)
	}
	if startupProbe.PeriodSeconds != defaultStartupProbePeriodSeconds {
		t.Fatalf("unexpected startup probe period: %d", startupProbe.PeriodSeconds)
	}
	if startupProbe.TimeoutSeconds != defaultStartupProbeTimeoutSeconds {
		t.Fatalf("unexpected startup probe timeout: %d", startupProbe.TimeoutSeconds)
	}
	if startupProbe.FailureThreshold != defaultStartupProbeFailureThreshold {
		t.Fatalf("unexpected startup probe failure threshold: %d", startupProbe.FailureThreshold)
	}
	if startupProbe.SuccessThreshold != defaultStartupProbeSuccessThreshold {
		t.Fatalf("unexpected startup probe success threshold: %d", startupProbe.SuccessThreshold)
	}
	if containers[0].LivenessProbe == nil || containers[0].LivenessProbe.HTTPGet.Port != intstr.FromString("api") {
		t.Fatalf("expected liveness probe port to default to named port, got %+v", containers[0].LivenessProbe)
	}
}

func TestResolveContainersDefaultStartupProbeKeepsLargerLivenessTimeout(t *testing.T) {
	containers := resolveContainers(serverManifest.Application{
		Name: "api",
		Image: serverManifest.ImageRef{
			Repository: "api",
			Tag:        "test",
		},
		Containers: []serverManifest.ContainerSpec{
			{
				Name:  "api",
				Ports: []serverManifest.ContainerPort{{Name: "api", ContainerPort: 8080}},
				LivenessProbe: &corev1.Probe{
					TimeoutSeconds: 5,
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/healthz"},
					},
				},
			},
		},
	}, testWeightsAndBiases(), nil, nil)

	if got := containers[0].StartupProbe.TimeoutSeconds; got != 5 {
		t.Fatalf("expected startup probe to preserve larger liveness timeout, got %d", got)
	}
}

func TestResolveContainersPreservesExplicitStartupProbe(t *testing.T) {
	containers := resolveContainers(serverManifest.Application{
		Name: "api",
		Image: serverManifest.ImageRef{
			Repository: "api",
			Tag:        "test",
		},
		Containers: []serverManifest.ContainerSpec{
			{
				Name:  "api",
				Ports: []serverManifest.ContainerPort{{Name: "api", ContainerPort: 8080}},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/healthz"},
					},
				},
				StartupProbe: &corev1.Probe{
					PeriodSeconds:    3,
					TimeoutSeconds:   7,
					FailureThreshold: 9,
					SuccessThreshold: 1,
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/startup"},
					},
				},
			},
		},
	}, testWeightsAndBiases(), nil, nil)

	startupProbe := containers[0].StartupProbe
	if startupProbe == nil || startupProbe.HTTPGet == nil {
		t.Fatalf("expected explicit startup probe")
	}
	if startupProbe.HTTPGet.Path != "/startup" {
		t.Fatalf("expected explicit startup probe path to be preserved, got %s", startupProbe.HTTPGet.Path)
	}
	if startupProbe.HTTPGet.Port != intstr.FromString("api") {
		t.Fatalf("expected explicit startup probe port to default to named port, got %#v", startupProbe.HTTPGet.Port)
	}
	if startupProbe.PeriodSeconds != 3 || startupProbe.TimeoutSeconds != 7 || startupProbe.FailureThreshold != 9 {
		t.Fatalf("expected explicit startup probe timings to be preserved, got %+v", startupProbe)
	}
}

func TestResolveContainersDoesNotDefaultStartupProbeWithoutHTTPGet(t *testing.T) {
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
			},
		},
	}, testWeightsAndBiases(), nil, nil)

	if containers[0].StartupProbe != nil {
		t.Fatalf("expected no default startup probe for non-HTTP liveness probe, got %+v", containers[0].StartupProbe)
	}
}
