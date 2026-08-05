package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateWandbStatusIfChangedSkipsEqualStatus(t *testing.T) {
	t.Parallel()

	wandb := &apiv2.WeightsAndBiases{Status: apiv2.WeightsAndBiasesStatus{Ready: true}}
	if err := updateWandbStatusIfChanged(context.Background(), nil, wandb, wandb.DeepCopy().Status); err != nil {
		t.Fatalf("unchanged status returned an error: %v", err)
	}
}

func TestUpdateWandbStatusIfChangedPersistsChange(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("add API to scheme: %v", err)
	}
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv2.WeightsAndBiases{}).
		WithObjects(wandb).
		Build()

	statusBefore := wandb.DeepCopy().Status
	wandb.Status.Ready = true
	if err := updateWandbStatusIfChanged(context.Background(), c, wandb, statusBefore); err != nil {
		t.Fatalf("update status: %v", err)
	}

	actual := &apiv2.WeightsAndBiases{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(wandb), actual); err != nil {
		t.Fatalf("get updated resource: %v", err)
	}
	if !actual.Status.Ready {
		t.Fatal("status change was not persisted")
	}
}

func TestApplicationManagedFieldsEqual(t *testing.T) {
	t.Parallel()

	before := &apiv2.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec:       apiv2.ApplicationSpec{Kind: "Deployment"},
	}
	after := before.DeepCopy()
	after.Status.Ready = true
	if !applicationManagedFieldsEqual(before, after) {
		t.Fatal("status-only changes should not rewrite the Application")
	}

	after.Spec.Kind = "StatefulSet"
	if applicationManagedFieldsEqual(before, after) {
		t.Fatal("spec changes must update the Application")
	}
}
