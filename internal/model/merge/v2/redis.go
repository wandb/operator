package v2

import (
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/model/merge"
)

// Redis will create a new WBRedisSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func Redis(actual v2.WBRedisSpec, defaultValues v2.WBRedisSpec) (v2.WBRedisSpec, error) {
	var redisSpec v2.WBRedisSpec

	/////////////////////////////////////////////
	// Apply defaultValue if not in actual

	if actual.Sentinel == nil {
		redisSpec.Sentinel = defaultValues.Sentinel.DeepCopy()
	} else if defaultValues.Sentinel == nil {
		redisSpec.Sentinel = actual.Sentinel.DeepCopy()
	} else {
		// merge as new Sentinel
		var redisSentinel v2.WBRedisSentinelSpec
		redisSentinel.Enabled = actual.Sentinel.Enabled
		if actual.Sentinel.Config == nil {
			redisSentinel.Config = defaultValues.Sentinel.Config.DeepCopy()
		} else if defaultValues.Sentinel.Config == nil {
			redisSentinel.Config = actual.Sentinel.Config.DeepCopy()
		} else {
			// merge as new Sentinel.Config
			var sentinelConfig v2.WBRedisSentinelConfig
			sentinelConfig.Resources = merge.Resources(
				actual.Sentinel.Config.Resources,
				defaultValues.Sentinel.Config.Resources,
			)
			redisSentinel.Config = &sentinelConfig
		}
		redisSpec.Sentinel = &redisSentinel
	}

	if actual.Config == nil {
		redisSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		redisSpec.Config = actual.Config.DeepCopy()
	} else {
		// merge as new Config
		var redisConfig v2.WBRedisConfig
		redisConfig.Resources = merge.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		redisSpec.Config = &redisConfig
	}

	redisSpec.StorageSize = merge.Coalesce(actual.StorageSize, defaultValues.StorageSize)
	redisSpec.Namespace = merge.Coalesce(actual.Namespace, defaultValues.Namespace)

	///////////////////////////////////////////
	// Values without overrides
	redisSpec.Enabled = actual.Enabled

	return redisSpec, nil
}
