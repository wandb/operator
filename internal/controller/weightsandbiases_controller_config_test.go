package controller

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMapTelemetryConfigToWandb(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 to scheme: %v", err)
	}
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding appsv2 to scheme: %v", err)
	}

	wandbOne := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-a", Namespace: "default"},
	}
	wandbTwo := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-b", Namespace: "other"},
	}

	reconciler := &WeightsAndBiasesReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandbOne, wandbTwo).Build(),
		TelemetryConfigRef: types.NamespacedName{
			Name:      "wandb-operator-telemetry-config",
			Namespace: "wandb-operator",
		},
	}

	matching := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-operator-telemetry-config",
			Namespace: "wandb-operator",
		},
	}

	requests := reconciler.mapTelemetryConfigToWandb(context.Background(), matching)
	if len(requests) != 2 {
		t.Fatalf("expected 2 reconcile requests, got %d", len(requests))
	}

	nonMatching := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-config",
			Namespace: "wandb-operator",
		},
	}

	requests = reconciler.mapTelemetryConfigToWandb(context.Background(), nonMatching)
	if len(requests) != 0 {
		t.Fatalf("expected no reconcile requests for non-matching configmap, got %d", len(requests))
	}
}
