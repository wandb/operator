package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
)

// BuildClickHouseSpec will create a new WBClickHouseSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildClickHouseSpec(actual apiv2.WBClickHouseSpec, defaultValues apiv2.WBClickHouseSpec) (apiv2.WBClickHouseSpec, error) {
	var clickhouseSpec apiv2.WBClickHouseSpec

	if actual.Config == nil {
		clickhouseSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		clickhouseSpec.Config = actual.Config.DeepCopy()
	} else {
		var clickhouseConfig apiv2.WBClickHouseConfig
		clickhouseConfig.Resources = utils.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		clickhouseSpec.Config = &clickhouseConfig
	}

	clickhouseSpec.StorageSize = utils.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	clickhouseSpec.Namespace = utils.Coalesce(actual.Namespace, defaultValues.Namespace)
	clickhouseSpec.Version = utils.Coalesce(actual.Version, defaultValues.Version)

	clickhouseSpec.Enabled = actual.Enabled
	clickhouseSpec.Replicas = actual.Replicas

	return clickhouseSpec, nil
}

func TranslateClickHouseConfig(config model.ClickHouseConfig) apiv2.WBClickHouseSpec {
	spec := apiv2.WBClickHouseSpec{
		Enabled:     config.Enabled,
		Namespace:   config.Namespace,
		StorageSize: config.StorageSize,
		Replicas:    config.Replicas,
		Version:     config.Version,
		Config: &apiv2.WBClickHouseConfig{
			Resources: config.Resources,
		},
	}

	return spec
}

func (i *InfraConfigBuilder) AddClickHouseSpec(actual apiv2.WBClickHouseSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.ClickHouseConfig
	var spec apiv2.WBClickHouseSpec

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = model.BuildClickHouseDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	defaultSpec := TranslateClickHouseConfig(defaultConfig)

	spec, err = BuildClickHouseSpec(actual, defaultSpec)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedClickHouse = &spec
	return i
}
