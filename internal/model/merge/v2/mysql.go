package v2

import (
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model/merge"
)

// MySQL will create a new WBMySQLSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func MySQL(actual v2.WBMySQLSpec, defaultValues v2.WBMySQLSpec) (v2.WBMySQLSpec, error) {
	var mysqlSpec v2.WBMySQLSpec

	/////////////////////////////////////////////
	// Apply defaultValue if not in actual

	if actual.Config == nil {
		mysqlSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		mysqlSpec.Config = actual.Config.DeepCopy()
	} else {
		// merge as new Config
		var mysqlConfig v2.WBMySQLConfig
		mysqlConfig.Resources = merge.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		mysqlSpec.Config = &mysqlConfig
	}

	mysqlSpec.StorageSize = merge.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	mysqlSpec.Namespace = merge.Coalesce(actual.Namespace, defaultValues.Namespace)

	///////////////////////////////////////////
	// Values without overrides
	mysqlSpec.Enabled = actual.Enabled

	return mysqlSpec, nil
}
