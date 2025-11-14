package redis

import (
	"context"

	"github.com/wandb/operator/internal/model"
)

type Redis interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualRedis interface {
	Upsert(ctx context.Context, redisConfig model.RedisConfig) *model.Results
	Delete(ctx context.Context) *model.Results
}

type DesiredRedis interface{}
