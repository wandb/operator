package redis

import (
	"context"

	redisv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redissentinel/v1beta2"
	"github.com/wandb/operator/internal/controller/wandb_v2"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// actualOpstree delegates to *get* all OpsTree Redis CRs we work with:
// Redis, RedisSentinel and RedisReplication
func actualOpstree(
	ctx context.Context,
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler,
	namespacedName types.NamespacedName,
) (
	opstree, error,
) {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var result = opstree{}

	if result.standalone, err = actualOpstreeRedis(ctx, reconciler, namespacedName); err != nil {
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

func actualOpstreeRedis(
	ctx context.Context,
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler,
	namespacedName types.NamespacedName,
) (
	*redisv1beta2.Redis, error,
) {
	result := &redisv1beta2.Redis{}
	err := reconciler.Get(ctx, namespacedName, result)
	if err != nil {
		if machErrors.IsNotFound(err) {
			return result, nil
		}
		return result, err
	}
	return result, nil
}

func actualOpstreeRedisReplication(
	ctx context.Context,
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler,
	namespacedName types.NamespacedName,
) (
	*redisreplicationv1beta2.RedisReplication, error,
) {
	result := &redisreplicationv1beta2.RedisReplication{}
	err := reconciler.Get(ctx, namespacedName, result)
	if err != nil {
		if machErrors.IsNotFound(err) {
			return result, nil
		}
		return result, err
	}
	return result, nil
}

func actualOpstreeRedisSentinel(
	ctx context.Context,
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler,
	namespacedName types.NamespacedName,
) (
	*redissentinelv1beta2.RedisSentinel, error,
) {
	result := &redissentinelv1beta2.RedisSentinel{}
	err := reconciler.Get(ctx, namespacedName, result)
	if err != nil {
		if machErrors.IsNotFound(err) {
			return result, nil
		}
		return result, err
	}
	return result, nil
}
