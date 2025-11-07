package redis

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/wandb_v2/common"
	ctrl "sigs.k8s.io/controller-runtime"
)

// handleDeleteOpstreeRedisStandalone, if a RedisStandalone is present but not desired, will:
//   - delete the RedisStandalone
//   - update the status of Redis on WandB CR accordingly
func handleDeleteOpstreeRedisStandalone(
	ctx context.Context, snapshot opstreeSnapshot,
) common.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	wandb := snapshot.wandb
	reconciler := snapshot.reconciler

	wantsStandalone := snapshot.desired.standalone != nil
	hasStandalone := snapshot.actual.standalone != nil

	if !wantsStandalone && hasStandalone {
		infraObj := snapshot.actual.standalone
		if err = reconciler.Delete(ctx, infraObj); err != nil {
			log.Error(err, "Failed to delete Redis Standalone")
			return common.CtrlError(err)
		}
		wandb.Status.State = apiv2.WBStateInfraUpdate
		wandb.Status.Message = "Deleting Redis Standalone"
		wandb.Status.RedisStatus.State = "deleting"
		if err = reconciler.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to update status after deleting Redis Standalone")
			return common.CtrlError(err)
		}
		return common.CtrlDone(common.PackageScope)
	}
	return common.CtrlContinue()
}

// handleDeleteOpstreeRedisReplication, if a RedisReplication is present but not desired, will:
//   - delete the RedisReplication
//   - update the status of Redis on WandB CR accordingly
func handleDeleteOpstreeRedisReplication(
	ctx context.Context, snapshot opstreeSnapshot,
) common.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	wandb := snapshot.wandb
	reconciler := snapshot.reconciler

	wantsReplication := snapshot.desired.replication != nil
	hasReplication := snapshot.actual.replication != nil

	if !wantsReplication && hasReplication {
		infraObj := snapshot.actual.replication
		if err = reconciler.Delete(ctx, infraObj); err != nil {
			log.Error(err, "Failed to delete Redis Replication")
			return common.CtrlError(err)
		}
		wandb.Status.State = apiv2.WBStateInfraUpdate
		wandb.Status.Message = "Deleting Redis Replication"
		wandb.Status.RedisStatus.State = "deleting"
		if err = reconciler.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to update status after deleting Redis Replication")
			return common.CtrlError(err)
		}
		return common.CtrlDone(common.PackageScope)
	}
	return common.CtrlContinue()
}

// handleDeleteOpstreeRedisSentinel, if a RedisSentinel is present but not desired, will:
//   - delete the RedisSentinel
//   - update the status of Redis on WandB CR accordingly
func handleDeleteOpstreeRedisSentinel(
	ctx context.Context, snapshot opstreeSnapshot,
) common.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	wandb := snapshot.wandb
	reconciler := snapshot.reconciler

	wantsSentinel := snapshot.desired.sentinel != nil
	hasSentinel := snapshot.actual.sentinel != nil

	if !wantsSentinel && hasSentinel {
		infraObj := snapshot.actual.sentinel
		if err = reconciler.Delete(ctx, infraObj); err != nil {
			log.Error(err, "Failed to delete Redis Sentinel")
			return common.CtrlError(err)
		}
		wandb.Status.State = apiv2.WBStateInfraUpdate
		wandb.Status.Message = "Deleting Redis Sentinel"
		wandb.Status.RedisStatus.State = "deleting"
		if err = reconciler.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to update status after deleting Redis Sentinel")
			return common.CtrlError(err)
		}
		return common.CtrlDone(common.PackageScope)
	}
	return common.CtrlContinue()
}
