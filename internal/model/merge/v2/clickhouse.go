package v2

import (
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model/merge"
)

// ClickHouse will create a new WBClickHouseSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func ClickHouse(actual v2.WBClickHouseSpec, defaultValues v2.WBClickHouseSpec) (v2.WBClickHouseSpec, error) {
	var clickhouseSpec v2.WBClickHouseSpec

	/////////////////////////////////////////////
	// Apply defaultValue if not in actual

	if actual.Config == nil {
		clickhouseSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		clickhouseSpec.Config = actual.Config.DeepCopy()
	} else {
		// merge as new Config
		var clickhouseConfig v2.WBClickHouseConfig
		clickhouseConfig.Resources = merge.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		clickhouseSpec.Config = &clickhouseConfig
	}

	clickhouseSpec.StorageSize = merge.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	clickhouseSpec.Namespace = merge.Coalesce(actual.Namespace, defaultValues.Namespace)
	clickhouseSpec.Version = merge.Coalesce(actual.Version, defaultValues.Version)

	///////////////////////////////////////////
	// Values without overrides
	clickhouseSpec.Enabled = actual.Enabled
	clickhouseSpec.Replicas = actual.Replicas

	return clickhouseSpec, nil
}
