package v2

import (
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
		return "", fmt.Errorf("unsupported size: %s", string(wbSize))
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

func computeOverallState(details []apiv2.WBStatusDetail, ready bool) apiv2.WBStateType {
	if len(details) == 0 {
		if ready {
			return apiv2.WBStateReady
		}
		return apiv2.WBStateUnknown
	}

	worst := details[0].State
	for _, detail := range details[1:] {
		if detail.State.WorseThan(worst) {
			worst = detail.State
		}
	}
	return worst
}
