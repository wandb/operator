package reconciler

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func updateWandbStatusIfChanged(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	statusBefore apiv2.WeightsAndBiasesStatus,
) error {
	if apiequality.Semantic.DeepEqual(statusBefore, wandb.Status) {
		return nil
	}
	return c.Status().Update(ctx, wandb)
}

// updateDeploymentStatusIfChanged persists the concrete resource only when its
// shared status changes. Application-specific status stays on that same object.
func updateDeploymentStatusIfChanged(ctx context.Context, c client.Client, deployment common.DeploymentResource, before *apiv2.BaseDeploymentStatus) error {
	if apiequality.Semantic.DeepEqual(before, deployment.GetBaseDeploymentStatus()) {
		return nil
	}
	return c.Status().Update(ctx, deployment)
}
