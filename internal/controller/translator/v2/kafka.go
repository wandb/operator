package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
)

// BuildKafkaConfig will create a new model.KafkaConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildKafkaConfig(actual apiv2.WBKafkaSpec, defaultConfig model.KafkaConfig) (model.KafkaConfig, error) {
	kafkaConfig := TranslateKafkaSpec(actual)

	kafkaConfig.StorageSize = utils.CoalesceQuantity(kafkaConfig.StorageSize, defaultConfig.StorageSize)
	kafkaConfig.Namespace = utils.Coalesce(kafkaConfig.Namespace, defaultConfig.Namespace)
	kafkaConfig.Resources = utils.Resources(kafkaConfig.Resources, defaultConfig.Resources)

	kafkaConfig.Enabled = actual.Enabled

	return kafkaConfig, nil
}

func TranslateKafkaSpec(spec apiv2.WBKafkaSpec) model.KafkaConfig {
	config := model.KafkaConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func (i *InfraConfigBuilder) AddKafkaConfig(actual apiv2.WBKafkaSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.KafkaConfig
	var mergedConfig model.KafkaConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = model.BuildKafkaDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildKafkaConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedKafka = mergedConfig
	return i
}
