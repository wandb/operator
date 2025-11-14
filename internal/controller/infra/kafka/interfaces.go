package kafka

import (
	"context"

	"github.com/wandb/operator/internal/model"
)

type Kafka interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualKafka interface {
	Upsert(ctx context.Context, kafkaConfig model.KafkaConfig) *model.Results
	Delete(ctx context.Context) *model.Results
}

type DesiredKafka interface{}
