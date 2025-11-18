package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
)

// BuildMySQLConfig will create a new model.MySQLConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMySQLConfig(actual apiv2.WBMySQLSpec, defaultConfig model.MySQLConfig) (model.MySQLConfig, error) {
	mysqlConfig := TranslateMySQLSpec(actual)

	mysqlConfig.StorageSize = utils.CoalesceQuantity(mysqlConfig.StorageSize, defaultConfig.StorageSize)
	mysqlConfig.Namespace = utils.Coalesce(mysqlConfig.Namespace, defaultConfig.Namespace)
	mysqlConfig.Resources = utils.Resources(mysqlConfig.Resources, defaultConfig.Resources)

	mysqlConfig.Enabled = actual.Enabled

	return mysqlConfig, nil
}

func TranslateMySQLSpec(spec apiv2.WBMySQLSpec) model.MySQLConfig {
	config := model.MySQLConfig{
		Enabled:     spec.Enabled,
		Namespace:   spec.Namespace,
		StorageSize: spec.StorageSize,
	}
	if spec.Config != nil {
		config.Resources = spec.Config.Resources
	}

	return config
}

func (i *InfraConfigBuilder) AddMySQLConfig(actual apiv2.WBMySQLSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.MySQLConfig
	var mergedConfig model.MySQLConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = model.BuildMySQLDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildMySQLConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedMySQL = mergedConfig
	return i
}
