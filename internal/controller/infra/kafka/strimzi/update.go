package strimzi

import (
	"context"
	"fmt"
	"strconv"

	strimziv1beta2 "github.com/wandb/operator/api/strimzi-kafka-vendored/v1beta2"
	"github.com/wandb/operator/internal/model"
)

func (a *strimziKafka) updateKafka(
	ctx context.Context, desiredKafka *strimziv1beta2.Kafka,
) *model.Results {
	results := model.InitResults()

	// Extract connection info from Kafka CR status
	// Connection format: wandb-kafka.{namespace}.svc.cluster.local:9092
	namespace := a.kafka.Namespace
	kafkaHost := fmt.Sprintf("%s.%s.svc.cluster.local", KafkaName, namespace)
	kafkaPort := strconv.Itoa(PlainListenerPort)

	connInfo := model.KafkaConnInfo{
		Host: kafkaHost,
		Port: kafkaPort,
	}
	results.AddStatuses(model.NewKafkaConnDetail(connInfo))

	return results
}

func (a *strimziKafka) updateNodePool(
	ctx context.Context, desiredNodePool *strimziv1beta2.KafkaNodePool,
) *model.Results {
	results := model.InitResults()

	// Minimal update - just acknowledge the NodePool exists
	// Strimzi operator handles the actual reconciliation
	return results
}
