package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
)

// BuildMinioSpec will create a new WBMinioSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMinioSpec(actual apiv2.WBMinioSpec, defaultValues apiv2.WBMinioSpec) (apiv2.WBMinioSpec, error) {
	var minioSpec apiv2.WBMinioSpec

	if actual.Config == nil {
		minioSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		minioSpec.Config = actual.Config.DeepCopy()
	} else {
		var minioConfig apiv2.WBMinioConfig
		minioConfig.Resources = utils.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		minioSpec.Config = &minioConfig
	}

	minioSpec.StorageSize = utils.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	minioSpec.Namespace = utils.Coalesce(actual.Namespace, defaultValues.Namespace)

	minioSpec.Enabled = actual.Enabled
	minioSpec.Replicas = actual.Replicas

	return minioSpec, nil
}

func TranslateMinioConfig(config model.MinioConfig) apiv2.WBMinioSpec {
	spec := apiv2.WBMinioSpec{
		Enabled:     config.Enabled,
		Namespace:   config.Namespace,
		StorageSize: config.StorageSize,
		Config: &apiv2.WBMinioConfig{
			Resources: config.Resources,
		},
	}

	return spec
}

func (i *InfraConfigBuilder) AddMinioSpec(actual apiv2.WBMinioSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.MinioConfig
	var spec apiv2.WBMinioSpec

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = model.BuildMinioDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	defaultSpec := TranslateMinioConfig(defaultConfig)

	spec, err = BuildMinioSpec(actual, defaultSpec)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedMinio = &spec
	return i
}
