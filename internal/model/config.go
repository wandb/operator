package model

import (
	apiv2 "github.com/wandb/operator/api/v2"
)

type InfraConfig interface {
	GetRedisConfig() (RedisConfig, error)
}

func BuildInfraConfig() *InfraConfigBuilder {
	return &InfraConfigBuilder{}
}

type InfraConfigBuilder struct {
	errors      []error
	mergedRedis *apiv2.WBRedisSpec
}
