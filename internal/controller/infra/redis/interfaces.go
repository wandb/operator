package redis

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator/common"
)

type Redis interface {
	IsStandalone() bool
	IsHighAvailability() bool
}

type ActualRedis interface {
	Upsert(ctx context.Context, redisConfig common.RedisConfig) *common.Results
	Delete(ctx context.Context) *common.Results
}

type DesiredRedis interface{}
