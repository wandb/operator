package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	servermanifest "github.com/wandb/operator/pkg/wandb/manifest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInferStateBlocksOnExternalInfrastructure(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("add W&B API to scheme: %v", err)
	}
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default", Generation: 2},
		Spec: apiv2.WeightsAndBiasesSpec{
			Redis: map[string]apiv2.RedisSpec{
				apiv2.DefaultInstanceName: {ExternalRedis: &apiv2.RedisConnection{}},
			},
		},
		Status: apiv2.WeightsAndBiasesStatus{
			Ready:              true,
			ObservedGeneration: 2,
			RedisStatus: map[string]apiv2.RedisInfraStatus{
				apiv2.DefaultInstanceName: {},
			},
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv2.WeightsAndBiases{}).
		WithObjects(wandb).
		Build()

	if err := inferState(context.Background(), c, wandb); err != nil {
		t.Fatalf("infer state: %v", err)
	}

	actual := &apiv2.WeightsAndBiases{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(wandb), actual); err != nil {
		t.Fatalf("get updated W&B resource: %v", err)
	}
	if actual.Status.Ready {
		t.Fatal("external Redis must block overall readiness")
	}
	condition := apimeta.FindStatusCondition(actual.Status.Conditions, readyConditionType)
	if condition == nil {
		t.Fatal("Ready condition was not written")
	}
	if condition.Status != metav1.ConditionFalse || condition.Reason != "DependenciesNotReady" {
		t.Fatalf("unexpected Ready condition: %#v", condition)
	}
	if condition.ObservedGeneration != 2 {
		t.Fatalf("observed generation = %d, want 2", condition.ObservedGeneration)
	}
}

func TestSetReadyStatusKeepsBooleanAndConditionConsistent(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Generation: 4},
	}

	setReadyStatus(wandb, true, "ReconciliationSucceeded", "ready")

	if !wandb.Status.Ready {
		t.Fatal("status.ready was not set")
	}
	condition := apimeta.FindStatusCondition(wandb.Status.Conditions, readyConditionType)
	if condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("unexpected Ready condition: %#v", condition)
	}
	if condition.ObservedGeneration != 4 {
		t.Fatalf("observed generation = %d, want 4", condition.ObservedGeneration)
	}
}

func TestRunMigrationsSurfacesFailedJobPhaseAndReason(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatalf("add W&B API to scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add batch API to scheme: %v", err)
	}
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Wandb: apiv2.WandbAppSpec{Version: "0.82.2"},
		},
		Status: apiv2.WeightsAndBiasesStatus{
			Wandb: apiv2.WandbStatus{
				Migration: apiv2.WandbMigrationStatus{
					Version: "0.82.2",
					Jobs:    map[string]apiv2.MigrationJobStatus{},
				},
			},
		},
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-weave-trace", Namespace: "default"},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{{
				Type:    batchv1.JobFailed,
				Status:  corev1.ConditionTrue,
				Reason:  "BackoffLimitExceeded",
				Message: "migration exited after detecting a partial version",
			}},
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&apiv2.WeightsAndBiases{}).
		WithObjects(wandb, job).
		Build()
	manifest := servermanifest.Manifest{
		Migrations: map[string]servermanifest.MigrationJob{
			"weave-trace": {},
		},
	}

	result, err := runMigrations(context.Background(), c, wandb, manifest)
	if err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("failed migration should requeue")
	}
	if wandb.Status.Wandb.Migration.Phase != migrationPhaseFailed {
		t.Fatalf("migration phase = %q, want %q", wandb.Status.Wandb.Migration.Phase, migrationPhaseFailed)
	}
	jobStatus := wandb.Status.Wandb.Migration.Jobs["weave-trace"]
	if jobStatus.Phase != migrationPhaseFailed || jobStatus.Reason != "BackoffLimitExceeded" {
		t.Fatalf("unexpected migration job status: %#v", jobStatus)
	}
	if jobStatus.Message == "" {
		t.Fatal("migration failure message was not surfaced")
	}

	reason, message := migrationReadiness(wandb)
	if reason != "MigrationFailed" {
		t.Fatalf("readiness reason = %q, want MigrationFailed", reason)
	}
	if message == "" {
		t.Fatal("readiness message should identify the failed migration")
	}
}
