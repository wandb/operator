package v2

import (
	"errors"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model"
)

type InfraConfig interface {
	GetRedisConfig() (*apiv2.WBRedisSpec, error)
	GetKafkaConfig() (*apiv2.WBKafkaSpec, error)
	GetMySQLConfig() (*apiv2.WBMySQLSpec, error)
	GetMinioConfig() (*apiv2.WBMinioSpec, error)
	GetClickHouseConfig() (*apiv2.WBClickHouseSpec, error)
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
