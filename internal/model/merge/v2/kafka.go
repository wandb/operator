package v2

import (
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model/merge"
)

// Kafka will create a new WBKafkaSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func Kafka(actual v2.WBKafkaSpec, defaultValues v2.WBKafkaSpec) (v2.WBKafkaSpec, error) {
	var kafkaSpec v2.WBKafkaSpec

	/////////////////////////////////////////////
	// Apply defaultValue if not in actual

	if actual.Config == nil {
		kafkaSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		kafkaSpec.Config = actual.Config.DeepCopy()
	} else {
		// merge as new Config
		var kafkaConfig v2.WBKafkaConfig
		kafkaConfig.Resources = merge.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		kafkaSpec.Config = &kafkaConfig
	}

	kafkaSpec.StorageSize = merge.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	kafkaSpec.Namespace = merge.Coalesce(actual.Namespace, defaultValues.Namespace)

	///////////////////////////////////////////
	// Values without overrides
	kafkaSpec.Enabled = actual.Enabled

	return kafkaSpec, nil
}
