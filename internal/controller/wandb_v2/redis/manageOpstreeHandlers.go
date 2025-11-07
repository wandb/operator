package redis

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/wandb_v2/common"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// handleOpstreeRedisStandaloneStatus ensures the RedisStatus of Wandb
// is aligned with the Redis CR
func handleOpstreeRedisStandaloneManageStatus(
	ctx context.Context, snapshot opstreeSnapshot,
) common.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	wandb := snapshot.wandb
	reconciler := snapshot.reconciler

	wantsStandalone := snapshot.desired.standalone != nil
	hasStandalone := snapshot.actual.standalone != nil

	// only bother if it desires and actually has standalone
	if wantsStandalone && hasStandalone {
		infraObj := snapshot.actual.standalone
		//infraObj.Status
		//wbRedisState := wandb.Status.RedisStatus

		wandb.Status.State = apiv2.WBStateInfraUpdate
		wandb.Status.Message = "Creating Redis Standalone"
		wandb.Status.RedisStatus.State = "pending"
		if err = reconciler.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to update status after creating Redis Standalone")
			return common.CtrlError(err)
		}
		return common.CtrlDone(common.PackageScope)
	}
	return common.CtrlContinue()
}
