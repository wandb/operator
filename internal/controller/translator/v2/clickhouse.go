package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
)

// BuildClickHouseConfig will create a new WBClickHouseSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildClickHouseConfig(actual apiv2.WBClickHouseSpec, defaultConfig model.ClickHouseConfig) (model.ClickHouseConfig, error) {
	clickhouseConfig := TranslateClickHouseSpec(actual)

	clickhouseConfig.StorageSize = utils.CoalesceQuantity(clickhouseConfig.StorageSize, defaultConfig.StorageSize)
	clickhouseConfig.Namespace = utils.Coalesce(clickhouseConfig.Namespace, defaultConfig.Namespace)
	clickhouseConfig.Version = utils.Coalesce(clickhouseConfig.Version, defaultConfig.Version)
	clickhouseConfig.Resources = utils.Resources(clickhouseConfig.Resources, defaultConfig.Resources)

	clickhouseConfig.Enabled = actual.Enabled
	clickhouseConfig.Replicas = actual.Replicas

	return clickhouseConfig, nil
}

func TranslateClickHouseSpec(spec apiv2.WBClickHouseSpec) model.ClickHouseConfig {
	config := model.ClickHouseConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
		Replicas:    spec.Replicas,
		Version:     spec.Version,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func ExtractClickHouseStatus(ctx context.Context, results *model.Results) apiv2.WBClickHouseStatus {
	return TranslateClickHouseStatus(
		ctx,
		model.ExtractClickHouseStatus(ctx, results),
	)
}

func TranslateClickHouseStatus(ctx context.Context, model model.ClickHouseStatus) apiv2.WBClickHouseStatus {
	var result apiv2.WBClickHouseStatus

	return result
}

func (i *InfraConfigBuilder) AddClickHouseConfig(actual apiv2.WBClickHouseSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.ClickHouseConfig
	var mergedConfig model.ClickHouseConfig

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

	mergedConfig, err = BuildClickHouseConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedClickHouse = mergedConfig
	return i
}
