package kafka

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator/common"
)

type Kafka interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualKafka interface {
	Upsert(ctx context.Context, kafkaConfig common.KafkaConfig) *common.Results
	Delete(ctx context.Context) *common.Results
}

type DesiredKafka interface{}
