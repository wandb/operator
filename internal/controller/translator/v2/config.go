package v2

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/common"
)

type InfraConfig interface {
	GetRedisConfig() (common.RedisConfig, error)
	GetKafkaConfig() (common.KafkaConfig, error)
	GetMySQLConfig() (common.MySQLConfig, error)
	GetMinioConfig() (common.MinioConfig, error)
	GetClickHouseConfig() (common.ClickHouseConfig, error)
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
	mergedRedis      common.RedisConfig
	mergedKafka      common.KafkaConfig
	mergedMySQL      common.MySQLConfig
	mergedMinio      common.MinioConfig
	mergedClickHouse common.ClickHouseConfig
}

func ToModelSize(wbSize apiv2.WBSize) (common.Size, error) {
	switch wbSize {
	case apiv2.WBSizeDev:
		return common.SizeDev, nil
	case apiv2.WBSizeSmall:
		return common.SizeSmall, nil
	default:
		return "", fmt.Errorf("unsupported size: %s", string(wbSize))
	}
}

func (i *InfraConfigBuilder) GetClickHouseConfig() (common.ClickHouseConfig, error) {
	return i.mergedClickHouse, nil
}

func (i *InfraConfigBuilder) GetRedisConfig() (common.RedisConfig, error) {
	return i.mergedRedis, nil
}

func (i *InfraConfigBuilder) GetKafkaConfig() (common.KafkaConfig, error) {
	return i.mergedKafka, nil
}

func (i *InfraConfigBuilder) GetMySQLConfig() (common.MySQLConfig, error) {
	return i.mergedMySQL, nil
}

func (i *InfraConfigBuilder) GetMinioConfig() (common.MinioConfig, error) {
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
