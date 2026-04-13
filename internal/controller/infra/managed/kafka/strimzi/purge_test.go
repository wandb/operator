package strimzi

import (
	"context"
	"testing"

	"github.com/wandb/operator/internal/controller/translator"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPurgeFinalizerDeletesKafkaWorkloadArtifacts(t *testing.T) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add apps scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add core scheme: %v", err)
	}

	podSetGVK := schema.GroupVersionKind{Group: "core.strimzi.io", Version: "v1", Kind: "StrimziPodSet"}
	podSetListGVK := schema.GroupVersionKind{Group: "core.strimzi.io", Version: "v1", Kind: "StrimziPodSetList"}
	scheme.AddKnownTypeWithName(podSetGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(podSetListGVK, &unstructured.UnstructuredList{})

	labelsMap := map[string]string{
		"weightsandbiases.apps.wandb.com/component": "kafka",
		"weightsandbiases.apps.wandb.com/name":      "wandb",
		"weightsandbiases.apps.wandb.com/namespace": "default",
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-kafka-entity-operator",
			Namespace: "default",
			Labels:    labelsMap,
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-kafka-wandb-kafka-node-pool-0",
			Namespace: "default",
			Labels:    labelsMap,
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-0-wandb-kafka-wandb-kafka-node-pool-0",
			Namespace: "default",
			Labels:    labelsMap,
		},
	}
	podSet := &unstructured.Unstructured{}
	podSet.SetGroupVersionKind(podSetGVK)
	podSet.SetNamespace("default")
	podSet.SetName("wandb-kafka-wandb-kafka-node-pool")
	podSet.SetLabels(labelsMap)

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(deployment, pod, pvc, podSet).
		Build()

	err := PurgeFinalizer(
		context.Background(),
		cl,
		types.NamespacedName{Namespace: "default", Name: "wandb-kafka"},
		translator.OnDeleteRule{
			Policy:   translator.Purge,
			Selector: labels.SelectorFromSet(labelsMap),
		},
	)
	if err != nil {
		t.Fatalf("PurgeFinalizer returned error: %v", err)
	}

	assertNotFound(t, cl, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})
	assertNotFound(t, cl, client.ObjectKeyFromObject(pod), &corev1.Pod{})
	assertNotFound(t, cl, client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})

	actualPodSet := &unstructured.Unstructured{}
	actualPodSet.SetGroupVersionKind(podSetGVK)
	assertNotFound(t, cl, client.ObjectKeyFromObject(podSet), actualPodSet)
}

func assertNotFound(t *testing.T, cl client.Client, key types.NamespacedName, obj client.Object) {
	t.Helper()

	err := cl.Get(context.Background(), key, obj)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected %T %s/%s to be deleted, got err=%v", obj, key.Namespace, key.Name, err)
	}
}
