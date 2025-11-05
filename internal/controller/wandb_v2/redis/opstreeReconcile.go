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

	// DefaultNamespace is the namespace it will deploy in if none is provided.
	DefaultNamespace = "default"
)

type opstree struct {
	redis       *redisv1beta2.Redis
	replication *redisreplicationv1beta2.RedisReplication
	sentinel    *redissentinelv1beta2.RedisSentinel
}

func reconcileOpstreeRedis(
	ctx context.Context,
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler,
	req ctrl.Request,
	wandb *apiv2.WeightsAndBiases,
) common.CtrlState {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var wbRedisSpec *apiv2.WBRedisSpec
	var actual, desired opstree

	if wbRedisSpec, err = initializeWbRedisSpec(wandb.Spec.Profile); err != nil {
		log.Error(err, "failed to initialize wandb redis")
		return common.CtrlError(err)
	}

	namespacedName := desiredOpstreeNamespacedName(req)

	if actual, err = loadActualOpstree(ctx, reconciler, namespacedName); err != nil {
		return common.CtrlError(err)
	}

	if desired, err = buildDesiredOpstree(ctx, wbRedisSpec, namespacedName); err != nil {
		return common.CtrlError(err)
	}

	snapshot := opstreeSnapshot{
		ctx:        ctx,
		desired:    desired,
		actual:     actual,
		status:     &wandb.Status.RedisStatus,
		reconciler: reconciler,
	}

	return reconcileOpstree(snapshot)
}

func loadActualOpstree(
	ctx context.Context,
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler,
	namespacedName types.NamespacedName,
) (
	opstree, error,
) {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var result = opstree{}

	if result.redis, err = actualOpstreeRedis(ctx, reconciler, namespacedName); err != nil {
		log.Error(err, "Failed to get opstree redis")
		return result, err
	}

	if result.replication, err = actualOpstreeRedisReplication(ctx, reconciler, namespacedName); err != nil {
		log.Error(err, "Failed to get opstree replication")
		return result, err
	}

	if result.sentinel, err = actualOpstreeRedisSentinel(ctx, reconciler, namespacedName); err != nil {
		log.Error(err, "Failed to get opstree sentinel")
		return result, err
	}

	return result, nil
}

func buildDesiredOpstree(
	ctx context.Context,
	wbRedisSpec *apiv2.WBRedisSpec,
	namespacedName types.NamespacedName,
) (opstree, error) {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var result = opstree{}

	if result.redis, err = desiredOpstreeRedis(namespacedName, wbRedisSpec); err != nil {
		log.Error(err, "Failed to build desired opstree redis")
		return result, err
	}

	if result.replication, err = desiredOpstreeReplication(namespacedName, wbRedisSpec); err != nil {
		log.Error(err, "Failed to build desired opstree replication")
		return result, err
	}

	if result.sentinel, err = desiredOpstreeSentinel(namespacedName, wbRedisSpec); err != nil {
		log.Error(err, "Failed to build desired opstree sentinel")
		return result, err
	}

	return result, nil
}

func reconcileOpstree(snapshot opstreeSnapshot) common.CtrlState {
	var state common.CtrlState

	for _, h := range allHandlers {
		state = h.reconcile()
		if state.ShouldExit(common.PackageScope) {
			return state
		}
	}

	return common.CtrlContinue()
}
