package redis

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/wandb_v2/common"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// handleCreateOpstreeRedisStandalone, if a RedisStandalone is desired but not present, will:
//   - set WandB CR as the owner of RedisStandalone
//   - create a RedisStandalone
//   - update the status of Redis on WandB CR accordingly
func handleCreateOpstreeRedisStandalone(
	ctx context.Context, snapshot opstreeSnapshot,
) common.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	wandb := snapshot.wandb
	reconciler := snapshot.reconciler
	scheme := reconciler.Scheme

	wantsStandalone := snapshot.desired.standalone != nil
	hasStandalone := snapshot.actual.standalone != nil

	if wantsStandalone && !hasStandalone {
		infraObj := snapshot.desired.standalone
		if err = controllerutil.SetOwnerReference(wandb, infraObj, scheme); err != nil {
			log.Error(err, "Failed to set owner reference for Redis Standalone")
			return common.CtrlError(err)
		}
		if err = reconciler.Create(ctx, infraObj); err != nil {
			log.Error(err, "Failed to create Redis Standalone")
			return common.CtrlError(err)
		}
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

// handleCreateOpstreeRedisReplication, if a RedisReplication is desired but not present, will:
//   - set WandB CR as the owner of RedisReplication
//   - create a RedisReplication
//   - update the status of Redis on WandB CR accordingly
func handleCreateOpstreeRedisReplication(
	ctx context.Context, snapshot opstreeSnapshot,
) common.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	wandb := snapshot.wandb
	reconciler := snapshot.reconciler
	scheme := reconciler.Scheme

	wantsReplication := snapshot.desired.replication != nil
	hasReplication := snapshot.actual.replication != nil

	if wantsReplication && !hasReplication {
		infraObj := snapshot.desired.replication
		if err = controllerutil.SetOwnerReference(wandb, infraObj, scheme); err != nil {
			log.Error(err, "Failed to set owner reference for Redis Replication")
			return common.CtrlError(err)
		}
		if err = reconciler.Create(ctx, infraObj); err != nil {
			log.Error(err, "Failed to create Redis Replication")
			return common.CtrlError(err)
		}
		wandb.Status.State = apiv2.WBStateInfraUpdate
		wandb.Status.Message = "Creating Redis Standalone"
		wandb.Status.RedisStatus.State = "pending"
		if err = reconciler.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to update status after creating Redis Replication")
			return common.CtrlError(err)
		}
		return common.CtrlDone(common.PackageScope)
	}
	return common.CtrlContinue()
}

// handleCreateOpstreeRedisSentinel, if a RedisSentinel is desired but not present, will:
//   - set WandB CR as the owner of RedisSentinel
//   - create a RedisSentinel
//   - update the status of Redis on WandB CR accordingly
func handleCreateOpstreeRedisSentinel(
	ctx context.Context, snapshot opstreeSnapshot,
) common.CtrlState {
	var err error
	log := ctrl.LoggerFrom(ctx)
	wandb := snapshot.wandb
	reconciler := snapshot.reconciler
	scheme := reconciler.Scheme

	wantsSentinel := snapshot.desired.sentinel != nil
	hasSentinel := snapshot.actual.sentinel != nil

	if wantsSentinel && !hasSentinel {
		infraObj := snapshot.desired.sentinel
		if err = controllerutil.SetOwnerReference(wandb, infraObj, scheme); err != nil {
			log.Error(err, "Failed to set owner reference for Redis Sentinel")
			return common.CtrlError(err)
		}
		if err = reconciler.Create(ctx, infraObj); err != nil {
			log.Error(err, "Failed to create Redis Sentinel")
			return common.CtrlError(err)
		}
		wandb.Status.State = apiv2.WBStateInfraUpdate
		wandb.Status.Message = "Creating Redis Sentinel"
		wandb.Status.RedisStatus.State = "pending"
		if err = reconciler.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to update status after creating Redis Sentinel")
			return common.CtrlError(err)
		}
		return common.CtrlDone(common.PackageScope)
	}
	return common.CtrlContinue()
}
