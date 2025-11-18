package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
)

// BuildKafkaSpec will create a new WBKafkaSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildKafkaSpec(actual apiv2.WBKafkaSpec, defaultValues apiv2.WBKafkaSpec) (apiv2.WBKafkaSpec, error) {
	var kafkaSpec apiv2.WBKafkaSpec

	if actual.Config == nil {
		kafkaSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		kafkaSpec.Config = actual.Config.DeepCopy()
	} else {
		var kafkaConfig apiv2.WBKafkaConfig
		kafkaConfig.Resources = utils.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		kafkaSpec.Config = &kafkaConfig
	}

	kafkaSpec.StorageSize = utils.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	kafkaSpec.Namespace = utils.Coalesce(actual.Namespace, defaultValues.Namespace)

	kafkaSpec.Enabled = actual.Enabled

	return kafkaSpec, nil
}

func TranslateKafkaConfig(config model.KafkaConfig) apiv2.WBKafkaSpec {
	spec := apiv2.WBKafkaSpec{
		Enabled:     config.Enabled,
		Namespace:   config.Namespace,
		StorageSize: config.StorageSize,
		Config: &apiv2.WBKafkaConfig{
			Resources: config.Resources,
		},
	}

	return spec
}

func (i *InfraConfigBuilder) AddKafkaSpec(actual apiv2.WBKafkaSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.KafkaConfig
	var spec apiv2.WBKafkaSpec

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

	defaultSpec := TranslateKafkaConfig(defaultConfig)

	spec, err = BuildKafkaSpec(actual, defaultSpec)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedKafka = &spec
	return i
}
