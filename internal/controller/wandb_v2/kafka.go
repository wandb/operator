package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/kafka"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
)

func (r *WeightsAndBiasesV2Reconciler) reconcileKafka(
	ctx context.Context,
	infraDetails translatorv2.InfraConfig,
	wandb *apiv2.WeightsAndBiases,
) *common.Results {
	var err error
	var results = &common.Results{}
	var nextResults *common.Results
	var kafkaConfig common.KafkaConfig
	var actual kafka.ActualKafka

	if kafkaConfig, err = infraDetails.GetKafkaConfig(); err != nil {
		results.AddErrors(err)
		return results
	}

	if actual, err = strimzi.Initialize(ctx, r.Client, kafkaConfig, wandb, r.Scheme); err != nil {
		results.AddErrors(err)
		return results
	}

	if kafkaConfig.Enabled {
		nextResults = actual.Upsert(ctx, kafkaConfig)
	} else {
		nextResults = actual.Delete(ctx)
	}
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	wandb.Status.KafkaStatus = translatorv2.ExtractKafkaStatus(ctx, results)
	if err = r.Status().Update(ctx, wandb); err != nil {
		results.AddErrors(err)
	}

	return results
}
