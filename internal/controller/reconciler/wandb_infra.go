package reconciler

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/objectstore/seaweedfs"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cleanupWandbLegacyMinio preserves the W&B migration behavior for the default
// object-store instance. Other deployments must not run this legacy cleanup.
func cleanupWandbLegacyMinio(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) {
	spec := wandb.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore
	if spec == nil {
		return
	}
	policy := wandb.GetRetentionPolicy(spec.ManagedInfraSpec)
	if !wandb.GetDeletionTimestamp().IsZero() && policy.OnDelete != apiv2.PurgeOnDelete && policy.OnDelete != apiv2.DetachOnDelete {
		return
	}
	rule := seaweedfs.ToObjectStoreOnDeleteRule(wandb, policy)
	if err := seaweedfs.CleanupLegacyMinio(ctx, c, wandb.Name, wandb.Namespace, wandb.UID, rule.Policy == common.Purge, rule.Selector); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to clean up legacy MinIO resources")
	}
}
