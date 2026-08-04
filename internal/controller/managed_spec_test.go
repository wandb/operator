package controller

import (
	"context"
	"errors"
	"reflect"
	"testing"

	appsv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagedSpecSelection(t *testing.T) {
	ctx := context.Background()
	namespace := "default"
	deployerSpec := testManagedSpec(map[string]interface{}{
		"global": map[string]interface{}{"enabled": true},
	})

	t.Run("uses Deployer when the managed spec does not exist", func(t *testing.T) {
		reconciler := testManagedSpecReconciler(t)
		calls := 0

		selected, pendingCutover, err := reconciler.selectBaseSpec(ctx, namespace, func() (*spec.Spec, error) {
			calls++
			return deployerSpec, nil
		})

		if err != nil {
			t.Fatalf("selectBaseSpec returned an error: %v", err)
		}
		if selected != deployerSpec {
			t.Fatal("selectBaseSpec did not return the Deployer spec")
		}
		if pendingCutover {
			t.Fatal("cutover must not be pending without a managed spec")
		}
		if calls != 1 {
			t.Fatalf("Deployer was called %d times, want 1", calls)
		}
	})

	t.Run("uses Deployer when the managed spec differs", func(t *testing.T) {
		reconciler := testManagedSpecReconciler(t, testManagedSpecConfigMap(namespace, map[string]interface{}{
			"global": map[string]interface{}{"enabled": false},
		}))

		selected, pendingCutover, err := reconciler.selectBaseSpec(ctx, namespace, func() (*spec.Spec, error) {
			return deployerSpec, nil
		})

		if err != nil {
			t.Fatalf("selectBaseSpec returned an error: %v", err)
		}
		if selected != deployerSpec {
			t.Fatal("selectBaseSpec did not return the Deployer spec")
		}
		if pendingCutover {
			t.Fatal("cutover must not be pending for a mismatched managed spec")
		}
	})

	t.Run("selects matching managed-owned configuration and requests cutover", func(t *testing.T) {
		reconciler := testManagedSpecReconciler(t, testManagedSpecConfigMap(namespace, deployerSpec.Values))

		selected, pendingCutover, err := reconciler.selectBaseSpec(ctx, namespace, func() (*spec.Spec, error) {
			withMetadata := *testManagedSpec(map[string]interface{}{
				"global": map[string]interface{}{
					"enabled": true,
					"image":   map[string]interface{}{"tag": "deployer-only"},
				},
				"legacy": map[string]interface{}{"enabled": true},
			})
			metadata := spec.Metadata{"releaseId": "release-1"}
			withMetadata.Metadata = &metadata
			withMetadata.Chart.(*charts.RepoRelease).Debug = true
			return &withMetadata, nil
		})

		if err != nil {
			t.Fatalf("selectBaseSpec returned an error: %v", err)
		}
		if !pendingCutover {
			t.Fatal("matching managed configuration must request cutover")
		}
		if selected == nil || selected.Metadata != nil {
			t.Fatal("selectBaseSpec did not return the managed ConfigMap spec")
		}
		if !reflect.DeepEqual(selected.Values, deployerSpec.Values) {
			t.Fatal("selectBaseSpec did not preserve the managed-owned values")
		}
	})

	t.Run("uses managed configuration without calling Deployer after cutover", func(t *testing.T) {
		reconciler := testManagedSpecReconciler(
			t,
			testManagedSpecConfigMap(namespace, deployerSpec.Values),
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: managedSpecStateConfigMapName, Namespace: namespace},
				Data:       map[string]string{managedSpecStateKey: "true"},
			},
		)
		calls := 0

		selected, pendingCutover, err := reconciler.selectBaseSpec(ctx, namespace, func() (*spec.Spec, error) {
			calls++
			return nil, errors.New("Deployer must not be called")
		})

		if err != nil {
			t.Fatalf("selectBaseSpec returned an error: %v", err)
		}
		if pendingCutover {
			t.Fatal("cutover cannot be pending after it is active")
		}
		if selected == nil || !selected.IsEqual(deployerSpec) {
			t.Fatal("selectBaseSpec did not return the managed spec")
		}
		if calls != 0 {
			t.Fatalf("Deployer was called %d times after cutover, want 0", calls)
		}
	})

	t.Run("fails closed when managed configuration is missing after cutover", func(t *testing.T) {
		reconciler := testManagedSpecReconciler(t, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: managedSpecStateConfigMapName, Namespace: namespace},
			Data:       map[string]string{managedSpecStateKey: "true"},
		})
		calls := 0

		selected, pendingCutover, err := reconciler.selectBaseSpec(ctx, namespace, func() (*spec.Spec, error) {
			calls++
			return deployerSpec, nil
		})

		if err == nil {
			t.Fatal("selectBaseSpec succeeded without the managed spec after cutover")
		}
		if selected != nil || pendingCutover {
			t.Fatal("selectBaseSpec returned a spec while failing closed")
		}
		if calls != 0 {
			t.Fatalf("Deployer was called %d times after cutover, want 0", calls)
		}
	})

	for _, value := range []string{"false", "1"} {
		t.Run("rejects managed state "+value+" without calling Deployer", func(t *testing.T) {
			reconciler := testManagedSpecReconciler(t, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: managedSpecStateConfigMapName, Namespace: namespace},
				Data:       map[string]string{managedSpecStateKey: value},
			})
			calls := 0

			_, _, err := reconciler.selectBaseSpec(ctx, namespace, func() (*spec.Spec, error) {
				calls++
				return deployerSpec, nil
			})

			if err == nil {
				t.Fatalf("selectBaseSpec accepted managed state %q", value)
			}
			if calls != 0 {
				t.Fatalf("Deployer was called %d times with invalid managed state, want 0", calls)
			}
		})
	}
}

func TestSetManagedSpecEnabled(t *testing.T) {
	ctx := context.Background()
	namespace := "default"
	reconciler := testManagedSpecReconciler(t)

	if err := reconciler.setManagedSpecEnabled(ctx, namespace); err != nil {
		t.Fatalf("setManagedSpecEnabled returned an error: %v", err)
	}

	state := &corev1.ConfigMap{}
	key := client.ObjectKey{Name: managedSpecStateConfigMapName, Namespace: namespace}
	if err := reconciler.Get(ctx, key, state); err != nil {
		t.Fatalf("could not read managed spec state: %v", err)
	}
	if state.Data[managedSpecStateKey] != "true" {
		t.Fatalf("managed state is %q, want true", state.Data[managedSpecStateKey])
	}
}

func TestManagedJSONSubsetEqual(t *testing.T) {
	tests := []struct {
		name     string
		managed  interface{}
		deployer interface{}
		matches  bool
	}{
		{
			name:     "ignores Deployer-only object keys",
			managed:  map[string]interface{}{"api": map[string]interface{}{"enabled": true}},
			deployer: map[string]interface{}{"api": map[string]interface{}{"enabled": true, "tag": "latest"}},
			matches:  true,
		},
		{
			name:     "rejects a different managed-owned value",
			managed:  map[string]interface{}{"api": map[string]interface{}{"enabled": true}},
			deployer: map[string]interface{}{"api": map[string]interface{}{"enabled": false}},
			matches:  false,
		},
		{
			name:     "requires arrays to have the same length",
			managed:  []interface{}{map[string]interface{}{"name": "first"}},
			deployer: []interface{}{map[string]interface{}{"name": "first"}, map[string]interface{}{"name": "second"}},
			matches:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := managedJSONSubsetEqual(test.managed, test.deployer); got != test.matches {
				t.Fatalf("managedJSONSubsetEqual() = %t, want %t", got, test.matches)
			}
		})
	}
}

func TestManagedSpecConfigMapRequests(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("could not register core Kubernetes types: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("could not register WeightsAndBiases types: %v", err)
	}

	reconciler := &WeightsAndBiasesReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&appsv1.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "first", Namespace: "default"}},
			&appsv1.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "other"}},
		).Build(),
		Scheme: scheme,
	}

	requests := reconciler.managedSpecConfigMapRequests(context.Background(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: managedSpecConfigMapName, Namespace: "default"},
	})
	if len(requests) != 1 {
		t.Fatalf("managedSpecConfigMapRequests returned %d requests, want 1", len(requests))
	}
	want := types.NamespacedName{Name: "first", Namespace: "default"}
	if requests[0].NamespacedName != want {
		t.Fatalf("managedSpecConfigMapRequests returned %v, want %v", requests[0].NamespacedName, want)
	}
}

func testManagedSpecReconciler(t *testing.T, objects ...client.Object) *WeightsAndBiasesReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("could not register core Kubernetes types: %v", err)
	}
	return &WeightsAndBiasesReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
		Scheme: scheme,
	}
}

func testManagedSpecConfigMap(namespace string, values map[string]interface{}) *corev1.ConfigMap {
	valuesJSON := `{"global":{"enabled":true}}`
	if enabled, ok := values["global"].(map[string]interface{})["enabled"].(bool); ok && !enabled {
		valuesJSON = `{"global":{"enabled":false}}`
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: managedSpecConfigMapName, Namespace: namespace},
		Data: map[string]string{
			"chart":  `{"name":"operator-wandb","url":"https://charts.wandb.ai","version":"0.43.5"}`,
			"values": valuesJSON,
		},
	}
}

func testManagedSpec(values map[string]interface{}) *spec.Spec {
	return &spec.Spec{
		Chart: &charts.RepoRelease{
			Name:    "operator-wandb",
			URL:     "https://charts.wandb.ai",
			Version: "0.43.5",
		},
		Values: values,
	}
}
