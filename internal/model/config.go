package model

import (
	apiv2 "github.com/wandb/operator/api/v2"
)

type InfraConfig interface {
	GetRedisConfig() (RedisConfig, error)
	GetKafkaConfig() (KafkaConfig, error)
	GetMySQLConfig() (MySQLConfig, error)
	GetMinioConfig() (MinioConfig, error)
}

func BuildInfraConfig() *InfraConfigBuilder {
	return &InfraConfigBuilder{}
}

type InfraConfigBuilder struct {
	errors      []error
	size        apiv2.WBSize
	mergedRedis *apiv2.WBRedisSpec
	mergedKafka *apiv2.WBKafkaSpec
	mergedMySQL *apiv2.WBMySQLSpec
	mergedMinio *apiv2.WBMinioSpec
}
