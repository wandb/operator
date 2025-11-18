package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/redis"
	"github.com/wandb/operator/internal/controller/infra/redis/opstree"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/internal/model"
)

func (r *WeightsAndBiasesV2Reconciler) reconcileRedis(
	ctx context.Context,
	infraDetails translatorv2.InfraConfig,
	wandb *apiv2.WeightsAndBiases,
) *model.Results {
	var err error
	var results = &model.Results{}
	var nextResults *model.Results
	var redisConfig model.RedisConfig
	var actual redis.ActualRedis

	if redisConfig, err = infraDetails.GetRedisConfig(); err != nil {
		results.AddErrors(err)
		return results
	}

	if actual, err = opstree.Initialize(ctx, r.Client, redisConfig, wandb, r.Scheme); err != nil {
		results.AddErrors(err)
		return results
	}

	if redisConfig.Enabled {
		nextResults = actual.Upsert(ctx, redisConfig)
	} else {
		nextResults = actual.Delete(ctx)
	}
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	wandb.Status.RedisStatus = model.ExtractRedisStatus(ctx, results)
	if err = r.Status().Update(ctx, wandb); err != nil {
		results.AddErrors(err)
	}

	return results

}
