package reconciler

import (
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
)

func assertPodSecurityBaseline(t *testing.T, securityContext *corev1.PodSecurityContext) {
	t.Helper()
	if securityContext == nil {
		t.Fatal("pod security context is nil")
	}
	if securityContext.SeccompProfile == nil ||
		securityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("pod seccomp profile = %#v, want RuntimeDefault", securityContext.SeccompProfile)
	}
	if securityContext.RunAsUser != nil || securityContext.RunAsGroup != nil || securityContext.FSGroup != nil {
		t.Fatalf("pod security context pins an identity: %#v", securityContext)
	}
}

func assertContainerSecurityBaseline(t *testing.T, securityContext *corev1.SecurityContext) {
	t.Helper()
	if securityContext == nil {
		t.Fatal("container security context is nil")
	}
	if securityContext.AllowPrivilegeEscalation == nil || *securityContext.AllowPrivilegeEscalation {
		t.Fatalf("allowPrivilegeEscalation = %#v, want false", securityContext.AllowPrivilegeEscalation)
	}
	if securityContext.Capabilities == nil || len(securityContext.Capabilities.Drop) != 1 ||
		securityContext.Capabilities.Drop[0] != appWorkloadCapabilityAll {
		t.Fatalf("capability drop = %#v, want [ALL]", securityContext.Capabilities)
	}
	if securityContext.SeccompProfile == nil ||
		securityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("container seccomp profile = %#v, want RuntimeDefault", securityContext.SeccompProfile)
	}
	if securityContext.RunAsUser != nil || securityContext.RunAsGroup != nil {
		t.Fatalf("container security context pins an identity: %#v", securityContext)
	}
}

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

func TestApplyWorkloadSecurityProfileValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		profile          serverManifest.WorkloadSecurityProfile
		wantRunAsNonRoot *bool
		wantReadOnlyRoot *bool
	}{
		{name: "empty"},
		{
			name: "enabled",
			profile: serverManifest.WorkloadSecurityProfile{
				RunAsNonRoot:           new(true),
				ReadOnlyRootFilesystem: new(true),
			},
			wantRunAsNonRoot: new(true),
			wantReadOnlyRoot: new(true),
		},
		{
			name: "explicitly disabled",
			profile: serverManifest.WorkloadSecurityProfile{
				RunAsNonRoot:           new(false),
				ReadOnlyRootFilesystem: new(false),
			},
			wantRunAsNonRoot: new(false),
			wantReadOnlyRoot: new(false),
		},
		{
			name: "mixed",
			profile: serverManifest.WorkloadSecurityProfile{
				RunAsNonRoot: new(true),
			},
			wantRunAsNonRoot: new(true),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pod := corev1.PodSpec{
				Containers:     []corev1.Container{{Name: "worker"}, {Name: "sidecar"}},
				InitContainers: []corev1.Container{{Name: "prepare"}},
			}
			applyWorkloadSecurityProfile(&pod, test.profile)
			assertPodSecurityBaseline(t, pod.SecurityContext)
			assertOptionalBool(t, "runAsNonRoot", pod.SecurityContext.RunAsNonRoot, test.wantRunAsNonRoot)
			for _, container := range append(pod.Containers, pod.InitContainers...) {
				assertContainerSecurityBaseline(t, container.SecurityContext)
				assertOptionalBool(t, container.Name+" readOnlyRootFilesystem", container.SecurityContext.ReadOnlyRootFilesystem, test.wantReadOnlyRoot)
				if container.SecurityContext.RunAsNonRoot != nil {
					t.Fatalf("container-level runAsNonRoot = %#v, want nil", container.SecurityContext.RunAsNonRoot)
				}
			}
		})
	}
}

func TestApplyApplicationSecurityProfileAndClearSettings(t *testing.T) {
	t.Parallel()

	profile := serverManifest.WorkloadSecurityProfile{
		RunAsNonRoot:           new(true),
		ReadOnlyRootFilesystem: new(true),
	}
	profiledApp := serverManifest.Application{
		Name:            "worker",
		Image:           serverManifest.ImageRef{Repository: "worker", Tag: "test"},
		SecurityProfile: profile,
		Containers: []serverManifest.ContainerSpec{
			{Name: "worker"},
			{Name: "sidecar"},
		},
		InitContainers: []serverManifest.ContainerSpec{
			{Name: "prepare", Image: serverManifest.ImageRef{Repository: "prepare", Tag: "test"}},
		},
	}

	pod := corev1.PodSpec{
		Containers:     resolveContainers(profiledApp, testWeightsAndBiases(), nil, nil),
		InitContainers: resolveInitContainers(profiledApp, testWeightsAndBiases(), nil, nil),
	}
	applyWorkloadSecurityProfile(&pod, profiledApp.SecurityProfile)
	for _, container := range append(pod.Containers, pod.InitContainers...) {
		assertContainerSecurityBaseline(t, container.SecurityContext)
		assertOptionalBool(t, container.Name+" readOnlyRootFilesystem", container.SecurityContext.ReadOnlyRootFilesystem, new(true))
	}
	assertPodSecurityBaseline(t, pod.SecurityContext)
	assertOptionalBool(t, "profiled runAsNonRoot", pod.SecurityContext.RunAsNonRoot, new(true))

	*pod.SecurityContext.RunAsNonRoot = false
	*pod.Containers[0].SecurityContext.ReadOnlyRootFilesystem = false
	assertOptionalBool(t, "manifest runAsNonRoot", profile.RunAsNonRoot, new(true))
	assertOptionalBool(t, "manifest readOnlyRootFilesystem", profile.ReadOnlyRootFilesystem, new(true))
	for _, container := range append(pod.Containers[1:], pod.InitContainers...) {
		assertOptionalBool(t, container.Name+" independent readOnlyRootFilesystem", container.SecurityContext.ReadOnlyRootFilesystem, new(true))
	}

	applyWorkloadSecurityProfile(&pod, serverManifest.WorkloadSecurityProfile{})
	for _, container := range append(pod.Containers, pod.InitContainers...) {
		assertContainerSecurityBaseline(t, container.SecurityContext)
		assertOptionalBool(t, container.Name+" readOnlyRootFilesystem after rollback", container.SecurityContext.ReadOnlyRootFilesystem, nil)
	}
	assertPodSecurityBaseline(t, pod.SecurityContext)
	assertOptionalBool(t, "runAsNonRoot after rollback", pod.SecurityContext.RunAsNonRoot, nil)
}

func TestApplySingleContainerSecurityProfile(t *testing.T) {
	t.Parallel()

	app := serverManifest.Application{
		Name:  "api",
		Image: serverManifest.ImageRef{Repository: "api", Tag: "test"},
		SecurityProfile: serverManifest.WorkloadSecurityProfile{
			ReadOnlyRootFilesystem: new(false),
		},
	}
	pod := corev1.PodSpec{
		Containers: resolveContainers(app, testWeightsAndBiases(), nil, nil),
	}
	applyWorkloadSecurityProfile(&pod, app.SecurityProfile)
	if len(pod.Containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(pod.Containers))
	}
	assertContainerSecurityBaseline(t, pod.Containers[0].SecurityContext)
	assertOptionalBool(t, "readOnlyRootFilesystem", pod.Containers[0].SecurityContext.ReadOnlyRootFilesystem, new(false))
}

func assertOptionalBool(t *testing.T, name string, got, want *bool) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Fatalf("%s = %#v, want %v", name, got, *want)
	}
}
