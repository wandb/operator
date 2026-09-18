package reconciler

import (
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
)

func boolPointer(value bool) *bool {
	return &value
}

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

func TestResolveSecurityProfileValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		profile          *serverManifest.WorkloadSecurityProfile
		wantRunAsNonRoot *bool
		wantReadOnlyRoot *bool
	}{
		{name: "absent"},
		{name: "empty", profile: &serverManifest.WorkloadSecurityProfile{}},
		{
			name: "enabled",
			profile: &serverManifest.WorkloadSecurityProfile{
				RunAsNonRoot:           boolPointer(true),
				ReadOnlyRootFilesystem: boolPointer(true),
			},
			wantRunAsNonRoot: boolPointer(true),
			wantReadOnlyRoot: boolPointer(true),
		},
		{
			name: "explicitly disabled",
			profile: &serverManifest.WorkloadSecurityProfile{
				RunAsNonRoot:           boolPointer(false),
				ReadOnlyRootFilesystem: boolPointer(false),
			},
			wantRunAsNonRoot: boolPointer(false),
			wantReadOnlyRoot: boolPointer(false),
		},
		{
			name: "mixed",
			profile: &serverManifest.WorkloadSecurityProfile{
				RunAsNonRoot: boolPointer(true),
			},
			wantRunAsNonRoot: boolPointer(true),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			podSecurityContext := resolvePodSecurityContext(test.profile)
			containerSecurityContext := resolveContainerSecurityContext(test.profile)
			assertPodSecurityBaseline(t, podSecurityContext)
			assertContainerSecurityBaseline(t, containerSecurityContext)
			assertOptionalBool(t, "runAsNonRoot", podSecurityContext.RunAsNonRoot, test.wantRunAsNonRoot)
			assertOptionalBool(t, "readOnlyRootFilesystem", containerSecurityContext.ReadOnlyRootFilesystem, test.wantReadOnlyRoot)
			if containerSecurityContext.RunAsNonRoot != nil {
				t.Fatalf("container-level runAsNonRoot = %#v, want nil", containerSecurityContext.RunAsNonRoot)
			}
		})
	}
}

func TestResolveApplicationContainersApplyAndClearSecurityProfile(t *testing.T) {
	t.Parallel()

	profile := &serverManifest.WorkloadSecurityProfile{
		RunAsNonRoot:           boolPointer(true),
		ReadOnlyRootFilesystem: boolPointer(true),
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

	profiledContainers := resolveContainers(profiledApp, testWeightsAndBiases(), nil, nil)
	profiledInitContainers := resolveInitContainers(profiledApp, testWeightsAndBiases(), nil, nil)
	for _, container := range append(profiledContainers, profiledInitContainers...) {
		assertContainerSecurityBaseline(t, container.SecurityContext)
		assertOptionalBool(t, container.Name+" readOnlyRootFilesystem", container.SecurityContext.ReadOnlyRootFilesystem, boolPointer(true))
	}
	profiledPodSecurityContext := resolvePodSecurityContext(profiledApp.SecurityProfile)
	assertOptionalBool(t, "profiled runAsNonRoot", profiledPodSecurityContext.RunAsNonRoot, boolPointer(true))

	legacyApp := profiledApp
	legacyApp.SecurityProfile = nil
	legacyContainers := resolveContainers(legacyApp, testWeightsAndBiases(), nil, nil)
	legacyInitContainers := resolveInitContainers(legacyApp, testWeightsAndBiases(), nil, nil)
	for _, container := range append(legacyContainers, legacyInitContainers...) {
		assertContainerSecurityBaseline(t, container.SecurityContext)
		assertOptionalBool(t, container.Name+" readOnlyRootFilesystem after rollback", container.SecurityContext.ReadOnlyRootFilesystem, nil)
	}
	legacyPodSecurityContext := resolvePodSecurityContext(legacyApp.SecurityProfile)
	assertPodSecurityBaseline(t, legacyPodSecurityContext)
	assertOptionalBool(t, "runAsNonRoot after rollback", legacyPodSecurityContext.RunAsNonRoot, nil)
}

func TestResolveSingleContainerAppliesSecurityProfile(t *testing.T) {
	t.Parallel()

	containers := resolveContainers(serverManifest.Application{
		Name:  "api",
		Image: serverManifest.ImageRef{Repository: "api", Tag: "test"},
		SecurityProfile: &serverManifest.WorkloadSecurityProfile{
			ReadOnlyRootFilesystem: boolPointer(false),
		},
	}, testWeightsAndBiases(), nil, nil)
	if len(containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(containers))
	}
	assertContainerSecurityBaseline(t, containers[0].SecurityContext)
	assertOptionalBool(t, "readOnlyRootFilesystem", containers[0].SecurityContext.ReadOnlyRootFilesystem, boolPointer(false))
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
