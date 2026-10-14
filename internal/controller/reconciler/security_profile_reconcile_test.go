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
		WithObjects(initialApplication, &apiv2.Application{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "default", CreationTimestamp: metav1.Now()}}).
		Build()

	enabled := apiv2.WorkloadSecurityProfile{RunAsNonRoot: new(true), ReadOnlyRootFilesystem: new(true)}
	disabled := apiv2.WorkloadSecurityProfile{RunAsNonRoot: new(false), ReadOnlyRootFilesystem: new(false)}
	for _, step := range []struct {
		name                       string
		manifest, shared, workload apiv2.WorkloadSecurityProfile
		nonRoot, readOnly          *bool
	}{
		{name: "unconfigured"},
		{name: "manifest defaults", manifest: enabled, nonRoot: new(true), readOnly: new(true)},
		{name: "shared CR overrides manifest", manifest: enabled, shared: disabled, nonRoot: new(false), readOnly: new(false)},
		{name: "workload CR overrides one field", manifest: enabled, shared: disabled, workload: apiv2.WorkloadSecurityProfile{RunAsNonRoot: new(true)}, nonRoot: new(true), readOnly: new(false)},
		{name: "remove workload override", manifest: enabled, shared: disabled, nonRoot: new(false), readOnly: new(false)},
		{name: "remove shared override", manifest: enabled, nonRoot: new(true), readOnly: new(true)},
		{name: "remove manifest profile"},
		{name: "shared CR without manifest profile", shared: enabled, nonRoot: new(true), readOnly: new(true)},
		{name: "workload CR false without manifest profile", shared: enabled, workload: disabled, nonRoot: new(false), readOnly: new(false)},
		{name: "workload CR alone", workload: enabled, nonRoot: new(true), readOnly: new(true)},
		{name: "remove all overrides"},
	} {
		t.Run(step.name, func(t *testing.T) {
			wandb.Spec.Wandb.SecurityProfile = step.shared
			wandb.Spec.Wandb.Applications = map[string]apiv2.WandbApplicationOverride{
				"api":     {SecurityProfile: step.workload},
				"unknown": {SecurityProfile: enabled},
			}
			manifest := servermanifest.Manifest{Applications: map[string]servermanifest.Application{
				"api": {Name: "api", Image: servermanifest.ImageRef{Repository: "example/api", Tag: "test"},
					SecurityProfile: step.manifest,
					Containers:      []servermanifest.ContainerSpec{{Name: "api"}, {Name: "sidecar"}},
					InitContainers:  []servermanifest.ContainerSpec{{Name: "prepare", Image: servermanifest.ImageRef{Repository: "example/init", Tag: "test"}}},
				},
				"worker": {Name: "worker", Image: servermanifest.ImageRef{Repository: "example/worker", Tag: "test"}},
			}}
			var previous *apiv2.Application
			for repeat := 0; repeat < 2; repeat++ {
				if _, err := reconcileApplications(context.Background(), c, wandb, manifest, telemetry.DefaultTelemetryRuntimeConfig()); err != nil {
					t.Fatal(err)
				}
				actual := &apiv2.Application{}
				if err := c.Get(context.Background(), client.ObjectKey{Name: "api", Namespace: "default"}, actual); err != nil {
					t.Fatal(err)
				}
				pod := actual.Spec.PodTemplate.Spec
				assertPodSecurityBaseline(t, pod.SecurityContext)
				assertOptionalBool(t, "runAsNonRoot", pod.SecurityContext.RunAsNonRoot, step.nonRoot)
				if len(pod.Containers) != 2 || len(pod.InitContainers) != 1 {
					t.Fatalf("unexpected containers: %#v", pod)
				}
				for _, container := range append(pod.Containers, pod.InitContainers...) {
					assertContainerSecurityBaseline(t, container.SecurityContext)
					assertOptionalBool(t, container.Name+" readOnlyRootFilesystem", container.SecurityContext.ReadOnlyRootFilesystem, step.readOnly)
				}
				if previous != nil && (!reflect.DeepEqual(previous.Spec, actual.Spec) || previous.ResourceVersion != actual.ResourceVersion) {
					t.Fatal("unchanged reconciliation rewrote the Application")
				}
				previous = actual
				worker := &apiv2.Application{}
				if err := c.Get(context.Background(), client.ObjectKey{Name: "worker", Namespace: "default"}, worker); err != nil {
					t.Fatal(err)
				}
				assertOptionalBool(t, "worker runAsNonRoot", worker.Spec.PodTemplate.Spec.SecurityContext.RunAsNonRoot, step.shared.RunAsNonRoot)
				assertOptionalBool(t, "worker readOnlyRootFilesystem", worker.Spec.PodTemplate.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem, step.shared.ReadOnlyRootFilesystem)
			}
			if !reflect.DeepEqual(manifest.Applications["api"].SecurityProfile, step.manifest) {
				t.Fatal("CR override mutated the manifest")
			}
		})
	}
}
