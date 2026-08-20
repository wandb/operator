package reconciler

import (
	"context"
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

	profiledManifest := servermanifest.Manifest{
		Applications: map[string]servermanifest.Application{
			"api": {
				Name:  "api",
				Image: servermanifest.ImageRef{Repository: "example/api", Tag: "test"},
				SecurityProfile: &servermanifest.WorkloadSecurityProfile{
					RunAsNonRoot:           boolPointer(true),
					ReadOnlyRootFilesystem: boolPointer(true),
				},
			},
		},
	}
	if _, err := reconcileApplications(
		context.Background(),
		c,
		wandb,
		profiledManifest,
		telemetry.DefaultTelemetryRuntimeConfig(),
	); err != nil {
		t.Fatalf("reconcile profiled application: %v", err)
	}

	actual := &apiv2.Application{}
	key := client.ObjectKey{Name: "api", Namespace: "default"}
	if err := c.Get(context.Background(), key, actual); err != nil {
		t.Fatalf("get profiled application: %v", err)
	}
	assertOptionalBool(t, "profiled runAsNonRoot", actual.Spec.PodTemplate.Spec.SecurityContext.RunAsNonRoot, boolPointer(true))
	if len(actual.Spec.PodTemplate.Spec.Containers) != 1 {
		t.Fatalf("profiled containers = %d, want 1", len(actual.Spec.PodTemplate.Spec.Containers))
	}
	assertOptionalBool(
		t,
		"profiled readOnlyRootFilesystem",
		actual.Spec.PodTemplate.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem,
		boolPointer(true),
	)

	legacyManifest := profiledManifest
	legacyApp := legacyManifest.Applications["api"]
	legacyApp.SecurityProfile = nil
	legacyManifest.Applications["api"] = legacyApp
	if _, err := reconcileApplications(
		context.Background(),
		c,
		wandb,
		legacyManifest,
		telemetry.DefaultTelemetryRuntimeConfig(),
	); err != nil {
		t.Fatalf("reconcile legacy application: %v", err)
	}

	actual = &apiv2.Application{}
	if err := c.Get(context.Background(), key, actual); err != nil {
		t.Fatalf("get rolled-back application: %v", err)
	}
	assertPodSecurityBaseline(t, actual.Spec.PodTemplate.Spec.SecurityContext)
	assertOptionalBool(t, "runAsNonRoot after rollback", actual.Spec.PodTemplate.Spec.SecurityContext.RunAsNonRoot, nil)
	if len(actual.Spec.PodTemplate.Spec.Containers) != 1 {
		t.Fatalf("legacy containers = %d, want 1", len(actual.Spec.PodTemplate.Spec.Containers))
	}
	assertContainerSecurityBaseline(t, actual.Spec.PodTemplate.Spec.Containers[0].SecurityContext)
	assertOptionalBool(
		t,
		"readOnlyRootFilesystem after rollback",
		actual.Spec.PodTemplate.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem,
		nil,
	)
}
