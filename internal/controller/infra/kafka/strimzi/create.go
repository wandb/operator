package strimzi

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (a *strimziKafka) createKafka(
	ctx context.Context, desiredKafka *v1beta3.Kafka,
) *common.Results {
	log := ctrl.LoggerFrom(ctx)
	results := common.InitResults()

	if a.kafka != nil {
		msg := "cannot create Kafka CR when it already exists"
		err := common.NewKafkaError(common.KafkaErrFailedToCreateCode, msg)
		log.Error(err, msg)
		results.AddErrors(err)
		return results
	}

	if err := a.client.Create(ctx, desiredKafka); err != nil {
		log.Error(err, "Failed to create Kafka CR")
		results.AddErrors(common.NewKafkaError(
			common.KafkaErrFailedToCreateCode,
			fmt.Sprintf("failed to create Kafka CR: %v", err),
		))
		return results
	}

	results.AddStatuses(
		common.NewKafkaStatusDetail(common.KafkaCreatedCode, fmt.Sprintf("Created Kafka CR: %s", KafkaName)),
	)

	return results
}

func (a *strimziKafka) createNodePool(
	ctx context.Context, desiredNodePool *v1beta3.KafkaNodePool,
) *common.Results {
	log := ctrl.LoggerFrom(ctx)
	results := common.InitResults()

	if a.nodePool != nil {
		msg := "cannot create KafkaNodePool CR when it already exists"
		err := common.NewKafkaError(common.KafkaErrFailedToCreateCode, msg)
		log.Error(err, msg)
		results.AddErrors(err)
		return results
	}

	if err := a.client.Create(ctx, desiredNodePool); err != nil {
		log.Error(err, "Failed to create KafkaNodePool CR")
		results.AddErrors(common.NewKafkaError(
			common.KafkaErrFailedToCreateCode,
			fmt.Sprintf("failed to create KafkaNodePool CR: %v", err),
		))
		return results
	}

	results.AddStatuses(
		common.NewKafkaStatusDetail(common.KafkaNodePoolCreatedCode, fmt.Sprintf("Created KafkaNodePool CR: %s", NodePoolName)),
	)

	return results
}
