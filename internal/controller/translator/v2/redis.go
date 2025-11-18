package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultSentinelGroup = model.DefaultSentinelGroup
)

// BuildRedisSpec will create a new WBRedisSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildRedisSpec(actual apiv2.WBRedisSpec, defaultValues apiv2.WBRedisSpec) (apiv2.WBRedisSpec, error) {
	var redisSpec apiv2.WBRedisSpec

	if actual.Sentinel == nil {
		redisSpec.Sentinel = defaultValues.Sentinel.DeepCopy()
	} else if defaultValues.Sentinel == nil {
		redisSpec.Sentinel = actual.Sentinel.DeepCopy()
	} else {
		var redisSentinel apiv2.WBRedisSentinelSpec
		redisSentinel.Enabled = actual.Sentinel.Enabled
		if actual.Sentinel.Config == nil {
			redisSentinel.Config = defaultValues.Sentinel.Config.DeepCopy()
		} else if defaultValues.Sentinel.Config == nil {
			redisSentinel.Config = actual.Sentinel.Config.DeepCopy()
		} else {
			var sentinelConfig apiv2.WBRedisSentinelConfig
			sentinelConfig.Resources = utils.Resources(
				actual.Sentinel.Config.Resources,
				defaultValues.Sentinel.Config.Resources,
			)
			sentinelConfig.MasterName = utils.Coalesce(
				actual.Sentinel.Config.MasterName,
				defaultValues.Sentinel.Config.MasterName,
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
		var redisConfig apiv2.WBRedisConfig
		redisConfig.Resources = utils.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		redisSpec.Config = &redisConfig
	}

	redisSpec.StorageSize = utils.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	redisSpec.Namespace = utils.Coalesce(actual.Namespace, defaultValues.Namespace)
	redisSpec.Enabled = actual.Enabled

	return redisSpec, nil
}

func RedisSentinelEnabled(wbSpec apiv2.WBRedisSpec) bool {
	return wbSpec.Sentinel != nil && wbSpec.Sentinel.Enabled
}

func TranslateRedisConfig(config model.RedisConfig) apiv2.WBRedisSpec {
	spec := apiv2.WBRedisSpec{
		Enabled:     config.Enabled,
		Namespace:   config.Namespace,
		StorageSize: config.StorageSize.String(),
		Config: &apiv2.WBRedisConfig{
			Resources: corev1.ResourceRequirements{
				Requests: config.Requests,
				Limits:   config.Limits,
			},
		},
	}

	if config.Sentinel.Enabled {
		spec.Sentinel = &apiv2.WBRedisSentinelSpec{
			Enabled: true,
			Config: &apiv2.WBRedisSentinelConfig{
				MasterName: config.Sentinel.MasterGroupName,
				Resources: corev1.ResourceRequirements{
					Requests: config.Sentinel.Requests,
					Limits:   config.Sentinel.Limits,
				},
			},
		}
	}

	return spec
}

func (i *InfraConfigBuilder) AddRedisSpec(actual apiv2.WBRedisSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.RedisConfig
	var spec apiv2.WBRedisSpec

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = model.BuildRedisDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	defaultSpec := TranslateRedisConfig(defaultConfig)

	spec, err = BuildRedisSpec(actual, defaultSpec)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedRedis = &spec
	return i
}
