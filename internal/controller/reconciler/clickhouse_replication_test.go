package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func replicationTestCR(replicas int32) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		TypeMeta:   metav1.TypeMeta{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb"},
		Spec: apiv2.WeightsAndBiasesSpec{
			ClickHouse: map[string]apiv2.ClickHouseSpec{
				apiv2.DefaultInstanceName: {
					ManagedClickHouse: &apiv2.ManagedClickHouseSpec{
						Name:      "wandb-chi",
						Namespace: "wandb",
						Replicas:  replicas,
					},
				},
			},
		},
	}
}

// inferManagedClickHouseStatus runs the managed status path and returns the
// published ClickHouse status for the default instance.
func inferManagedClickHouseStatus(t *testing.T, replicas int32) apiv2.ClickHouseInfraStatus {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiv2 to scheme: %v", err)
	}

	wandb := replicationTestCR(replicas)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(wandb).
		WithStatusSubresource(wandb).
		Build()

	wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{}
	if _, err := managedClickHouseInferStatus(
		context.Background(), client, record.NewFakeRecorder(10),
		wandb, apiv2.DefaultInstanceName, nil, &apiv2.ClickHouseConnection{},
	); err != nil {
		t.Fatalf("managedClickHouseInferStatus: %v", err)
	}
	return wandb.Status.ClickHouseStatus[apiv2.DefaultInstanceName]
}

// Weave creates ReplicatedMergeTree tables only when told the cluster is
// replicated; without this the operator provisions replicas that never receive
// writes.
func TestManagedClickHouseStatusPublishesReplication(t *testing.T) {
	for _, tc := range []struct {
		name       string
		replicas   int32
		replicated bool
	}{
		{"single replica is not replicated", 1, false},
		{"two replicas are replicated", 2, true},
		{"three replicas are replicated", 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status := inferManagedClickHouseStatus(t, tc.replicas)
			if status.Connection.Replicated != tc.replicated {
				t.Fatalf("expected Replicated=%t for %d replicas, got %t",
					tc.replicated, tc.replicas, status.Connection.Replicated)
			}
		})
	}
}

// The published name is used verbatim as `ON CLUSTER <name>`, so it has to be
// the cluster the CHI declares — not the Service-name derivation.
func TestManagedClickHouseStatusPublishesCHIClusterName(t *testing.T) {
	status := inferManagedClickHouseStatus(t, 2)

	if status.Connection.ClusterName != altinity.CHIClusterName() {
		t.Fatalf("expected ClusterName=%q, got %q", altinity.CHIClusterName(), status.Connection.ClusterName)
	}
	if status.Connection.ClusterName == altinity.ClusterName("wandb-chi") {
		t.Fatal("published the Service-name derivation instead of the CHI cluster name")
	}
}

// inferExternalClickHouseStatus runs the external status path for a declared
// connection and returns the published status.
func inferExternalClickHouseStatus(t *testing.T, declared *apiv2.ClickHouseConnection) apiv2.ClickHouseInfraStatus {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiv2 to scheme: %v", err)
	}

	wandb := &apiv2.WeightsAndBiases{
		TypeMeta:   metav1.TypeMeta{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb"},
		Spec: apiv2.WeightsAndBiasesSpec{
			ClickHouse: map[string]apiv2.ClickHouseSpec{
				apiv2.DefaultInstanceName: {ExternalClickHouse: declared},
			},
		},
	}
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(wandb).
		WithStatusSubresource(wandb).
		Build()

	wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{}
	if _, err := externalClickHouseInferStatus(
		context.Background(), client, wandb, apiv2.DefaultInstanceName,
		nil, &apiv2.ClickHouseConnection{},
	); err != nil {
		t.Fatalf("externalClickHouseInferStatus: %v", err)
	}
	return wandb.Status.ClickHouseStatus[apiv2.DefaultInstanceName]
}

// ReadState rebuilds the connection from the Secret, which carries no topology,
// so the user-declared values have to be carried across from the spec.
func TestExternalClickHouseStatusCarriesDeclaredReplication(t *testing.T) {
	status := inferExternalClickHouseStatus(t, &apiv2.ClickHouseConnection{
		Replicated:  true,
		ClusterName: "weavecluster",
	})

	if !status.Connection.Replicated {
		t.Error("declared Replicated=true did not reach the published status")
	}
	if status.Connection.ClusterName != "weavecluster" {
		t.Errorf("expected ClusterName=%q, got %q", "weavecluster", status.Connection.ClusterName)
	}
}

// Undeclared external topology must stay zero so the manifest's defaultValue
// applies rather than the operator asserting something it cannot know.
func TestExternalClickHouseStatusLeavesReplicationUnset(t *testing.T) {
	status := inferExternalClickHouseStatus(t, &apiv2.ClickHouseConnection{})

	if status.Connection.Replicated {
		t.Error("external ClickHouse must not be reported as replicated")
	}
	if status.Connection.ClusterName != "" {
		t.Errorf("expected an empty ClusterName for external ClickHouse, got %q", status.Connection.ClusterName)
	}
}
