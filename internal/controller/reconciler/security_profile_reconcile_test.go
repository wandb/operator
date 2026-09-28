package reconciler

import (
	"context"
	"reflect"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	servermanifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileApplicationsAddsAndClearsSecurityProfile(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("add W&B API to scheme: %v", err)
	}

	tolerations := []corev1.Toleration{}
	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiv2.GroupVersion.String(),
			Kind:       "WeightsAndBiases",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Tolerations: &tolerations,
		},
		Status: apiv2.WeightsAndBiasesStatus{
			Wandb: apiv2.WandbStatus{
				Applications: map[string]apiv2.ApplicationStatus{},
			},
		},
	}
	initialApplication := &apiv2.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "api",
			Namespace:         "default",
			CreationTimestamp: metav1.Now(),
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initialApplication).
		Build()

	enabled := &servermanifest.WorkloadSecurityProfile{
		RunAsNonRoot: boolPointer(true), ReadOnlyRootFilesystem: boolPointer(true),
	}
	for _, version := range []string{"0.84.9", "0.85.0", "0.85.0-rc.1", "1.0.0", "custom-build"} {
		t.Run(version, func(t *testing.T) {
			wandb.Spec.Wandb.Version = version
			for _, transition := range []struct {
				name    string
				profile *servermanifest.WorkloadSecurityProfile
			}{
				{"false", &servermanifest.WorkloadSecurityProfile{RunAsNonRoot: boolPointer(false), ReadOnlyRootFilesystem: boolPointer(false)}},
				{"empty", &servermanifest.WorkloadSecurityProfile{}},
				{"omitted", nil},
			} {
				t.Run(transition.name, func(t *testing.T) {
					for _, profile := range []*servermanifest.WorkloadSecurityProfile{enabled, transition.profile} {
						manifest := servermanifest.Manifest{Applications: map[string]servermanifest.Application{
							"api": {Name: "api", Image: servermanifest.ImageRef{Repository: "example/api", Tag: "test"},
								SecurityProfile: profile,
								Containers:      []servermanifest.ContainerSpec{{Name: "api"}, {Name: "sidecar"}},
								InitContainers:  []servermanifest.ContainerSpec{{Name: "prepare", Image: servermanifest.ImageRef{Repository: "example/init", Tag: "test"}}},
							},
						}}
						var previous *apiv2.Application
						for repeat := 0; repeat < 2; repeat++ {
							if _, err := reconcileApplications(context.Background(), c, wandb, manifest, telemetry.DefaultTelemetryRuntimeConfig()); err != nil {
								t.Fatalf("reconcile: %v", err)
							}
							actual := &apiv2.Application{}
							if err := c.Get(context.Background(), client.ObjectKey{Name: "api", Namespace: "default"}, actual); err != nil {
								t.Fatal(err)
							}
							var nonRoot, readOnly *bool
							if profile != nil {
								nonRoot, readOnly = profile.RunAsNonRoot, profile.ReadOnlyRootFilesystem
							}
							pod := actual.Spec.PodTemplate.Spec
							assertPodSecurityBaseline(t, pod.SecurityContext)
							assertOptionalBool(t, "runAsNonRoot", pod.SecurityContext.RunAsNonRoot, nonRoot)
							if len(pod.Containers) != 2 || len(pod.InitContainers) != 1 {
								t.Fatalf("unexpected containers: %#v", pod)
							}
							for _, container := range append(pod.Containers, pod.InitContainers...) {
								assertContainerSecurityBaseline(t, container.SecurityContext)
								assertOptionalBool(t, container.Name+" readOnlyRootFilesystem", container.SecurityContext.ReadOnlyRootFilesystem, readOnly)
							}
							if previous != nil && (!reflect.DeepEqual(previous.Spec, actual.Spec) || previous.ResourceVersion != actual.ResourceVersion) {
								t.Fatal("unchanged reconciliation rewrote the Application")
							}
							previous = actual
						}
					}
				})
			}
		})
	}
}
