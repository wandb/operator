package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
	merge2 "github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
)

// BuildMySQLSpec will create a new WBMySQLSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMySQLSpec(actual apiv2.WBMySQLSpec, defaultValues apiv2.WBMySQLSpec) (apiv2.WBMySQLSpec, error) {
	var mysqlSpec apiv2.WBMySQLSpec

	if actual.Config == nil {
		mysqlSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		mysqlSpec.Config = actual.Config.DeepCopy()
	} else {
		var mysqlConfig apiv2.WBMySQLConfig
		mysqlConfig.Resources = merge2.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		mysqlSpec.Config = &mysqlConfig
	}

	mysqlSpec.StorageSize = merge2.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	mysqlSpec.Namespace = merge2.Coalesce(actual.Namespace, defaultValues.Namespace)

	mysqlSpec.Enabled = actual.Enabled

	return mysqlSpec, nil
}

func TranslateMySQLConfig(config model.MySQLConfig) apiv2.WBMySQLSpec {
	spec := apiv2.WBMySQLSpec{
		Enabled:     config.Enabled,
		Namespace:   config.Namespace,
		StorageSize: config.StorageSize,
		Config: &apiv2.WBMySQLConfig{
			Resources: config.Resources,
		},
	}

	return spec
}

func (i *InfraConfigBuilder) AddMySQLSpec(actual apiv2.WBMySQLSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.MySQLConfig
	var spec apiv2.WBMySQLSpec

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

	defaultSpec := TranslateMySQLConfig(defaultConfig)

	spec, err = BuildMySQLSpec(actual, defaultSpec)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedMySQL = &spec
	return i
}
