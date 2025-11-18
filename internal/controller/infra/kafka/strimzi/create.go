package strimzi

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/model"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (a *strimziKafka) createKafka(
	ctx context.Context, desiredKafka *v1beta3.Kafka,
) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	if a.kafka != nil {
		msg := "cannot create Kafka CR when it already exists"
		err := model.NewKafkaError(model.KafkaErrFailedToCreateCode, msg)
		log.Error(err, msg)
		results.AddErrors(err)
		return results
	}

	if err := a.client.Create(ctx, desiredKafka); err != nil {
		log.Error(err, "Failed to create Kafka CR")
		results.AddErrors(model.NewKafkaError(
			model.KafkaErrFailedToCreateCode,
			fmt.Sprintf("failed to create Kafka CR: %v", err),
		))
		return results
	}

	results.AddStatuses(
		model.NewKafkaStatusDetail(model.KafkaCreatedCode, fmt.Sprintf("Created Kafka CR: %s", KafkaName)),
	)

	return results
}

func (a *strimziKafka) createNodePool(
	ctx context.Context, desiredNodePool *v1beta3.KafkaNodePool,
) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	if a.nodePool != nil {
		msg := "cannot create KafkaNodePool CR when it already exists"
		err := model.NewKafkaError(model.KafkaErrFailedToCreateCode, msg)
		log.Error(err, msg)
		results.AddErrors(err)
		return results
	}

	if err := a.client.Create(ctx, desiredNodePool); err != nil {
		log.Error(err, "Failed to create KafkaNodePool CR")
		results.AddErrors(model.NewKafkaError(
			model.KafkaErrFailedToCreateCode,
			fmt.Sprintf("failed to create KafkaNodePool CR: %v", err),
		))
		return results
	}

	results.AddStatuses(
		model.NewKafkaStatusDetail(model.KafkaNodePoolCreatedCode, fmt.Sprintf("Created KafkaNodePool CR: %s", NodePoolName)),
	)

	return results
}
