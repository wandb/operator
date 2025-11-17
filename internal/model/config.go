package model

import (
	apiv2 "github.com/wandb/operator/api/v2"
)

type InfraConfig interface {
	GetRedisConfig() (RedisConfig, error)
	GetKafkaConfig() (KafkaConfig, error)
	GetMySQLConfig() (MySQLConfig, error)
	GetMinioConfig() (MinioConfig, error)
	GetClickHouseConfig() (ClickHouseConfig, error)
}

func BuildInfraConfig(ownerNamespace string) *InfraConfigBuilder {
	return &InfraConfigBuilder{
		ownerNamespace: ownerNamespace,
	}
}

type InfraConfigBuilder struct {
	errors           []error
	ownerNamespace   string
	size             apiv2.WBSize
	mergedRedis      *apiv2.WBRedisSpec
	mergedKafka      *apiv2.WBKafkaSpec
	mergedMySQL      *apiv2.WBMySQLSpec
	mergedMinio      *apiv2.WBMinioSpec
	mergedClickHouse *apiv2.WBClickHouseSpec
}
