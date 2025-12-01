package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/redis/opstree"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *WeightsAndBiasesV2Reconciler) redisResourceReconcile(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var standaloneDesired *redisv1beta2.Redis
	var sentinelDesired *redissentinelv1beta2.RedisSentinel
	var replicationDesired *redisreplicationv1beta2.RedisReplication
	var specNamespacedName = redisSpecNamespacedName(wandb.Spec.Redis)

	if standaloneDesired, err = translatorv2.ToRedisStandaloneVendorSpec(ctx, wandb.Spec.Redis, wandb, r.Scheme); err != nil {
		return err
	}
	if err = opstree.CrudStandaloneResource(ctx, r.Client, specNamespacedName, standaloneDesired); err != nil {
		return err
	}

	if sentinelDesired, err = translatorv2.ToRedisSentinelVendorSpec(ctx, wandb.Spec.Redis, wandb, r.Scheme); err != nil {
		return err
	}
	if err = opstree.CrudSentinelResource(ctx, r.Client, specNamespacedName, sentinelDesired); err != nil {
		return err
	}

	if replicationDesired, err = translatorv2.ToRedisReplicationVendorSpec(ctx, wandb.Spec.Redis, wandb, r.Scheme); err != nil {
		return err
	}
	if err = opstree.CrudReplicationResource(ctx, r.Client, specNamespacedName, replicationDesired); err != nil {
		return err
	}

	//wandb.Status.RedisStatus = translatorv2.ExtractRedisStatus(ctx, results)
	//if err = r.Status().Update(ctx, wandb); err != nil {
	//	results.AddErrors(err)
	//}

	return nil

}

func (r *WeightsAndBiasesV2Reconciler) redisStatusUpdate(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var conditions []common.RedisCondition
	var specNamespacedName = redisSpecNamespacedName(wandb.Spec.Redis)

	if conditions, err = opstree.GetConditions(ctx, r.Client, specNamespacedName); err != nil {
		return err
	}
	wandb.Status.RedisStatus = translatorv2.ExtractRedisStatus(ctx, conditions)
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "failed to update status")
		return err
	}

	return nil
}

func redisSpecNamespacedName(redis apiv2.WBRedisSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: redis.Namespace,
		Name:      redis.Name,
	}

}
