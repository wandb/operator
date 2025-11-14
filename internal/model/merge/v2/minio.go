package v2

import (
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model/merge"
)

// Minio will create a new WBMinioSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func Minio(actual v2.WBMinioSpec, defaultValues v2.WBMinioSpec) (v2.WBMinioSpec, error) {
	var minioSpec v2.WBMinioSpec

	/////////////////////////////////////////////
	// Apply defaultValue if not in actual

	if actual.Config == nil {
		minioSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		minioSpec.Config = actual.Config.DeepCopy()
	} else {
		// merge as new Config
		var minioConfig v2.WBMinioConfig
		minioConfig.Resources = merge.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		minioSpec.Config = &minioConfig
	}

	minioSpec.StorageSize = merge.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	minioSpec.Namespace = merge.Coalesce(actual.Namespace, defaultValues.Namespace)

	///////////////////////////////////////////
	// Values without overrides
	minioSpec.Enabled = actual.Enabled
	minioSpec.Replicas = actual.Replicas

	return minioSpec, nil
}
