package redis

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/wandb_v2"
	"github.com/wandb/operator/internal/controller/wandb_v2/common"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	redisv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redissentinel/v1beta2"
)

const (
	// NamePrefix is the prefix name for redis resources
	NamePrefix = "wandb-redis"
)

var opstreeHandlers = []OpstreeSnapshotHandler{
	handleCreateOpstreeRedisStandalone,
	handleCreateOpstreeRedisSentinel,
	handleCreateOpstreeRedisReplication,
	handleDeleteOpstreeRedisStandalone,
	handleDeleteOpstreeRedisSentinel,
	handleDeleteOpstreeRedisReplication,
}

type opstree struct {
	standalone  *redisv1beta2.Redis
	replication *redisreplicationv1beta2.RedisReplication
	sentinel    *redissentinelv1beta2.RedisSentinel
}

func ReconcileOpstreeRedis(
	ctx context.Context,
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler,
	req ctrl.Request,
	wandb *apiv2.WeightsAndBiases,
) common.CtrlState {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var defaultRedis, actualRedis, mergedRedis apiv2.WBRedisSpec
	var actual, desired opstree

	actualRedis = wandb.Spec.Redis
	if defaultRedis, err = wbRedisSpecDefaults(wandb.Spec.Profile); err != nil {
		log.Error(err, "failed to initialize wandb redis")
		return common.CtrlError(err)
	}
	if mergedRedis, err = wbRedisSpecsMerge(actualRedis, defaultRedis); err != nil {
		log.Error(err, "failed to merge wandb redis")
		return common.CtrlError(err)
	}

	namespacedName := types.NamespacedName{Namespace: mergedRedis.Namespace, Name: NamePrefix}

	if actual, err = actualOpstree(ctx, reconciler, namespacedName); err != nil {
		return common.CtrlError(err)
	}

	if desired, err = desiredOpstree(ctx, mergedRedis, namespacedName); err != nil {
		return common.CtrlError(err)
	}

	snapshot := opstreeSnapshot{
		desired:    desired,
		actual:     actual,
		wandb:      wandb,
		reconciler: reconciler,
	}

	return reconcileOpstree(snapshot)
}

func reconcileOpstree(snapshot opstreeSnapshot) common.CtrlState {
	return common.CtrlContinue()
}
