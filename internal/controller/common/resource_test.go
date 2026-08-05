package common

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type updateCountingClient struct {
	client.Client
	updates int
}

func (c *updateCountingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updates++
	return c.Client.Update(ctx, obj, opts...)
}

func TestCrudResourceSkipsUnchangedManagedFields(t *testing.T) {
	t.Parallel()

	actual := testConfigMap("value")
	actual.Labels["co-manager"] = "preserve"
	actual.Finalizers = []string{"co-manager/finalizer"}
	c := newUpdateCountingClient(t, actual)
	desired := testConfigMap("value")

	action, err := CrudResource(context.Background(), c, desired, actual.DeepCopy())
	if err != nil {
		t.Fatalf("reconcile resource: %v", err)
	}
	if action != UnchangedAction {
		t.Fatalf("action = %q, want %q", action, UnchangedAction)
	}
	if c.updates != 0 {
		t.Fatalf("updates = %d, want 0", c.updates)
	}
}

func TestCrudResourcePreservesCoManagedMetadataOnUpdate(t *testing.T) {
	t.Parallel()

	actual := testConfigMap("old")
	actual.Labels["co-manager"] = "preserve"
	actual.Finalizers = []string{"co-manager/finalizer"}
	c := newUpdateCountingClient(t, actual)
	desired := testConfigMap("new")

	action, err := CrudResource(context.Background(), c, desired, actual.DeepCopy())
	if err != nil {
		t.Fatalf("reconcile resource: %v", err)
	}
	if action != UpdateAction {
		t.Fatalf("action = %q, want %q", action, UpdateAction)
	}
	if c.updates != 1 {
		t.Fatalf("updates = %d, want 1", c.updates)
	}

	updated := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(actual), updated); err != nil {
		t.Fatalf("get updated ConfigMap: %v", err)
	}
	if updated.Data["key"] != "new" {
		t.Fatalf("data = %q, want new", updated.Data["key"])
	}
	if updated.Labels["co-manager"] != "preserve" {
		t.Fatal("co-managed label was removed")
	}
	if len(updated.Finalizers) != 1 || updated.Finalizers[0] != "co-manager/finalizer" {
		t.Fatalf("finalizers = %v, want co-manager finalizer", updated.Finalizers)
	}
}

func TestCrudResourceTreatsSecretStringDataAsExistingData(t *testing.T) {
	t.Parallel()

	actual := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "connection", Namespace: "default"},
		Data:       map[string][]byte{"Password": []byte("secret")},
	}
	c := newUpdateCountingClient(t, actual)
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "connection", Namespace: "default"},
		StringData: map[string]string{"Password": "secret"},
	}

	action, err := CrudResource(context.Background(), c, desired, actual.DeepCopy())
	if err != nil {
		t.Fatalf("reconcile resource: %v", err)
	}
	if action != UnchangedAction {
		t.Fatalf("action = %q, want %q", action, UnchangedAction)
	}
	if c.updates != 0 {
		t.Fatalf("updates = %d, want 0", c.updates)
	}
}

func testConfigMap(value string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "default",
			Labels:    map[string]string{"managed-by": "wandb"},
		},
		Data: map[string]string{"key": value},
	}
}

func newUpdateCountingClient(t *testing.T, objects ...client.Object) *updateCountingClient {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core API to scheme: %v", err)
	}
	return &updateCountingClient{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()}
}
