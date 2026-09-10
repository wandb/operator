package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestSelectBaseSpec(t *testing.T) {
	const namespace = "default"
	for _, tc := range []struct {
		name         string
		enabled      bool
		missing      bool
		mutate       func(map[string]string)
		wantManaged  bool
		wantErrorLog bool
	}{
		{name: "disabled with valid managed spec"},
		{name: "enabled with different managed values", enabled: true, wantManaged: true},
		{name: "missing managed spec", enabled: true, missing: true},
		{name: "missing values", enabled: true, mutate: func(d map[string]string) { delete(d, "values") }, wantErrorLog: true},
		{name: "invalid values JSON", enabled: true, mutate: func(d map[string]string) { d["values"] = "{" }, wantErrorLog: true},
		{name: "null values", enabled: true, mutate: func(d map[string]string) { d["values"] = "null" }, wantErrorLog: true},
		{name: "non-object values", enabled: true, mutate: func(d map[string]string) { d["values"] = "[]" }, wantErrorLog: true},
		{name: "missing chart", enabled: true, mutate: func(d map[string]string) { delete(d, "chart") }, wantErrorLog: true},
		{name: "invalid chart JSON", enabled: true, mutate: func(d map[string]string) { d["chart"] = "{" }, wantErrorLog: true},
		{name: "unsupported chart", enabled: true, mutate: func(d map[string]string) { d["chart"] = "{}" }, wantErrorLog: true},
		{name: "disabled ignores invalid managed spec", mutate: func(d map[string]string) { d["values"] = "{" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cm := testManagedSpecConfigMap(namespace, testManagedValues(true))
			if tc.mutate != nil {
				tc.mutate(cm.Data)
			}
			objects := []client.Object{
				// Old cutover state must have no effect on selection.
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "wandb-managed-spec-state", Namespace: namespace}, Data: map[string]string{"managed": "true"}},
			}
			if !tc.missing {
				objects = append(objects, cm)
			}
			r := testManagedSpecReconciler(t, objects...)
			r.ManagedSpecEnabled = tc.enabled
			deployerSpec := testManagedSpec(testManagedValues(false))
			var logs bytes.Buffer
			ctx := ctrllog.IntoContext(context.Background(), zap.New(zap.WriteTo(&logs)))
			calls := 0
			for i := 0; i < 2; i++ {
				selected, err := r.selectBaseSpec(ctx, namespace, func() (*spec.Spec, error) {
					calls++
					return deployerSpec, nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if tc.wantManaged {
					if selected.Values["global"].(map[string]interface{})["enabled"] != true {
						t.Fatal("expected managed values despite Deployer mismatch")
					}
				} else if selected != deployerSpec {
					t.Fatal("expected Deployer fallback")
				}
			}
			if calls != 2 {
				t.Fatalf("Deployer called %d times, want 2", calls)
			}
			if got := strings.Contains(logs.String(), "\"level\":\"error\""); got != tc.wantErrorLog {
				t.Fatalf("error log = %t, want %t; logs: %s", got, tc.wantErrorLog, logs.String())
			}
		})
	}
}

func TestSelectBaseSpecPropagatesDeployerCacheError(t *testing.T) {
	r := testManagedSpecReconciler(t, testManagedSpecConfigMap("default", testManagedValues(true)))
	want := errors.New("cache write failed")
	selected, err := r.selectBaseSpec(context.Background(), "default", func() (*spec.Spec, error) {
		return nil, want
	})
	if selected != nil || !errors.Is(err, want) {
		t.Fatalf("got %v, %v; want cache error", selected, err)
	}
}

func TestSelectBaseSpecFallsBackAfterManagedSpecRemoval(t *testing.T) {
	cm := testManagedSpecConfigMap("default", testManagedValues(true))
	r := testManagedSpecReconciler(t, cm)
	deployerSpec := testManagedSpec(testManagedValues(false))
	getDeployer := func() (*spec.Spec, error) { return deployerSpec, nil }
	selected, err := r.selectBaseSpec(context.Background(), "default", getDeployer)
	if err != nil || selected == deployerSpec {
		t.Fatalf("managed selection failed: %v", err)
	}
	if err := r.Client.Delete(context.Background(), cm); err != nil {
		t.Fatal(err)
	}
	selected, err = r.selectBaseSpec(context.Background(), "default", getDeployer)
	if err != nil || selected != deployerSpec {
		t.Fatalf("fallback failed: %v", err)
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
		Client:             fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
		Scheme:             scheme,
		ManagedSpecEnabled: true,
	}
}

func testManagedSpecConfigMap(namespace string, values map[string]interface{}) *corev1.ConfigMap {
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: managedSpecConfigMapName, Namespace: namespace},
		Data: map[string]string{
			"chart":  `{"name":"operator-wandb","url":"https://charts.wandb.ai","version":"0.43.5"}`,
			"values": string(valuesJSON),
		},
	}
}

func testManagedValues(enabled bool) map[string]interface{} {
	return map[string]interface{}{
		"global": map[string]interface{}{
			"cloudProvider": "gcp",
			"enabled":       enabled,
			"extraEnv": map[string]interface{}{
				"TAG_CLOUD":       "GCP",
				"TAG_CUSTOMER_NS": "wandb-test",
			},
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
