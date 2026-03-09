package mysql

import (
	"context"
	"testing"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/pkg/vendored/mysql-operator/v2"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestComputeMySQLReportedReadyConditionOnline(t *testing.T) {
	cluster := &v2.InnoDBCluster{
		Status: v2.InnoDBClusterStatus{
			RawExtension: runtime.RawExtension{
				Raw: []byte(`{"cluster":{"status":"ONLINE"}}`),
			},
		},
	}

	conditions := computeMySQLReportedReadyCondition(context.Background(), cluster)
	if len(conditions) != 1 {
		t.Fatalf("expected one condition, got %d", len(conditions))
	}
	if conditions[0].Status != "True" {
		t.Fatalf("expected ready condition true, got %s", conditions[0].Status)
	}
	if conditions[0].Reason != "Online" {
		t.Fatalf("expected reason Online, got %s", conditions[0].Reason)
	}
}

func TestComputeMySQLReportedReadyConditionPending(t *testing.T) {
	cluster := &v2.InnoDBCluster{
		Status: v2.InnoDBClusterStatus{
			RawExtension: runtime.RawExtension{
				Raw: []byte(`{"cluster":{"status":"PENDING"}}`),
			},
		},
	}

	conditions := computeMySQLReportedReadyCondition(context.Background(), cluster)
	if len(conditions) != 1 {
		t.Fatalf("expected one condition, got %d", len(conditions))
	}
	if conditions[0].Status != "False" {
		t.Fatalf("expected ready condition false, got %s", conditions[0].Status)
	}
	if conditions[0].Reason != "PENDING" {
		t.Fatalf("expected reason PENDING, got %s", conditions[0].Reason)
	}
}

func TestComputeMySQLReportedReadyConditionMissingStatus(t *testing.T) {
	cluster := &v2.InnoDBCluster{
		Status: v2.InnoDBClusterStatus{
			RawExtension: runtime.RawExtension{
				Raw: []byte(`{"kopf":{"progress":{}}}`),
			},
		},
	}

	conditions := computeMySQLReportedReadyCondition(context.Background(), cluster)
	if len(conditions) != 1 {
		t.Fatalf("expected one condition, got %d", len(conditions))
	}
	if conditions[0].Status != "False" {
		t.Fatalf("expected ready condition false, got %s", conditions[0].Status)
	}
	if conditions[0].Reason != common.NoResourceReason {
		t.Fatalf("expected reason %s, got %s", common.NoResourceReason, conditions[0].Reason)
	}
}
