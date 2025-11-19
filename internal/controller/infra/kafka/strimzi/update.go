package strimzi

import (
	"context"
	"fmt"
	"strconv"

	"github.com/wandb/operator/internal/controller/translator/common"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
)

func (a *strimziKafka) updateKafka(
	ctx context.Context, desiredKafka *v1beta3.Kafka,
) *common.Results {
	results := common.InitResults()

	// Extract connection info from Kafka CR status
	// Connection format: wandb-kafka.{namespace}.svc.cluster.local:9092
	namespace := a.kafka.Namespace
	kafkaHost := fmt.Sprintf("%s.%s.svc.cluster.local", KafkaName, namespace)
	kafkaPort := strconv.Itoa(PlainListenerPort)

	connInfo := common.KafkaConnInfo{
		Host: kafkaHost,
		Port: kafkaPort,
	}
	results.AddStatuses(common.NewKafkaConnDetail(connInfo))

	return results
}

func (a *strimziKafka) updateNodePool(
	ctx context.Context, desiredNodePool *v1beta3.KafkaNodePool,
) *common.Results {
	results := common.InitResults()

	// Minimal update - just acknowledge the NodePool exists
	// Strimzi operator handles the actual reconciliation
	return results
}
