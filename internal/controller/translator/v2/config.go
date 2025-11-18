package v2

import (
	"errors"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model"
)

type InfraConfig interface {
	GetRedisConfig() (model.RedisConfig, error)
	GetKafkaConfig() (model.KafkaConfig, error)
	GetMySQLConfig() (model.MySQLConfig, error)
	GetMinioConfig() (model.MinioConfig, error)
	GetClickHouseConfig() (model.ClickHouseConfig, error)
}

func BuildInfraConfig(ownerNamespace string, size apiv2.WBSize) *InfraConfigBuilder {
	return &InfraConfigBuilder{
		ownerNamespace: ownerNamespace,
		size:           size,
	}
}

type InfraConfigBuilder struct {
	errors           []error
	ownerNamespace   string
	size             apiv2.WBSize
	mergedRedis      model.RedisConfig
	mergedKafka      model.KafkaConfig
	mergedMySQL      model.MySQLConfig
	mergedMinio      model.MinioConfig
	mergedClickHouse model.ClickHouseConfig
}

func ToModelSize(wbSize apiv2.WBSize) (model.Size, error) {
	switch wbSize {
	case apiv2.WBSizeDev:
		return model.SizeDev, nil
	case apiv2.WBSizeSmall:
		return model.SizeSmall, nil
	default:
		return "", errors.New(fmt.Sprintf("unsupported size: %s", string(wbSize)))
	}
}

func (i *InfraConfigBuilder) GetClickHouseConfig() (model.ClickHouseConfig, error) {
	return i.mergedClickHouse, nil
}

func (i *InfraConfigBuilder) GetRedisConfig() (model.RedisConfig, error) {
	return i.mergedRedis, nil
}

func (i *InfraConfigBuilder) GetKafkaConfig() (model.KafkaConfig, error) {
	return i.mergedKafka, nil
}

func (i *InfraConfigBuilder) GetMySQLConfig() (model.MySQLConfig, error) {
	return i.mergedMySQL, nil
}

func (i *InfraConfigBuilder) GetMinioConfig() (model.MinioConfig, error) {
	return i.mergedMinio, nil
}
