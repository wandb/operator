package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
)

// BuildMinioConfig will create a new model.MinioConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMinioConfig(actual apiv2.WBMinioSpec, defaultConfig model.MinioConfig) (model.MinioConfig, error) {
	minioConfig := TranslateMinioSpec(actual)

	minioConfig.StorageSize = utils.CoalesceQuantity(minioConfig.StorageSize, defaultConfig.StorageSize)
	minioConfig.Namespace = utils.Coalesce(minioConfig.Namespace, defaultConfig.Namespace)
	minioConfig.Resources = utils.Resources(minioConfig.Resources, defaultConfig.Resources)

	minioConfig.Enabled = actual.Enabled

	return minioConfig, nil
}

func TranslateMinioSpec(spec apiv2.WBMinioSpec) model.MinioConfig {
	config := model.MinioConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractMinioStatus(ctx context.Context, results *model.Results) apiv2.WBMinioStatus {
	return TranslateMinioStatus(
		ctx,
		model.ExtractMinioStatus(ctx, results),
	)
}

func TranslateMinioStatus(ctx context.Context, model model.MinioStatus) apiv2.WBMinioStatus {
	var result apiv2.WBMinioStatus

	return result
}

func (i *InfraConfigBuilder) AddMinioConfig(actual apiv2.WBMinioSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.MinioConfig
	var mergedConfig model.MinioConfig

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

	mergedConfig, err = BuildMinioConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedMinio = mergedConfig
	return i
}
