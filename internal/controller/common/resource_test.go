package common

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCrudResourceNoActionForSubsetDesired(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 scheme: %v", err)
	}

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "cm",
			Namespace:   "default",
			Annotations: map[string]string{"controller-added": "true"},
		},
		Data: map[string]string{"key": "value"},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	actual := &corev1.ConfigMap{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "cm", Namespace: "default"}, actual); err != nil {
		t.Fatalf("failed to get configmap: %v", err)
	}

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value"},
	}

	action, err := CrudResource(context.Background(), client, desired, actual)
	if err != nil {
		t.Fatalf("CrudResource returned error: %v", err)
	}
	if action != NoAction {
		t.Fatalf("expected no action, got %s", action)
	}
}

func TestCrudResourceUpdateWhenDesiredDiffers(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 scheme: %v", err)
	}

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "old"},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	actual := &corev1.ConfigMap{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "cm", Namespace: "default"}, actual); err != nil {
		t.Fatalf("failed to get configmap: %v", err)
	}

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "default",
		},
		Data: map[string]string{"key": "new"},
	}

	action, err := CrudResource(context.Background(), client, desired, actual)
	if err != nil {
		t.Fatalf("CrudResource returned error: %v", err)
	}
	if action != UpdateAction {
		t.Fatalf("expected update action, got %s", action)
	}

	updated := &corev1.ConfigMap{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "cm", Namespace: "default"}, updated); err != nil {
		t.Fatalf("failed to get updated configmap: %v", err)
	}
	if updated.Data["key"] != "new" {
		t.Fatalf("expected updated data, got %q", updated.Data["key"])
	}
}
