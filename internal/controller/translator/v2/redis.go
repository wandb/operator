package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DefaultSentinelGroup = model.DefaultSentinelGroup
)

// BuildRedisConfig will create a new model.RedisConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildRedisConfig(actual apiv2.WBRedisSpec, defaultConfig model.RedisConfig) (model.RedisConfig, error) {
	redisConfig := TranslateRedisSpec(actual)

	if redisConfig.StorageSize.IsZero() {
		redisConfig.StorageSize = defaultConfig.StorageSize
	}
	redisConfig.Namespace = utils.Coalesce(redisConfig.Namespace, defaultConfig.Namespace)

	mergedResources := utils.Resources(
		corev1.ResourceRequirements{Requests: redisConfig.Requests, Limits: redisConfig.Limits},
		corev1.ResourceRequirements{Requests: defaultConfig.Requests, Limits: defaultConfig.Limits},
	)
	redisConfig.Requests = mergedResources.Requests
	redisConfig.Limits = mergedResources.Limits

	if actual.Sentinel == nil {
		redisConfig.Sentinel = defaultConfig.Sentinel
	} else {
		mergedSentinelResources := utils.Resources(
			corev1.ResourceRequirements{Requests: redisConfig.Sentinel.Requests, Limits: redisConfig.Sentinel.Limits},
			corev1.ResourceRequirements{Requests: defaultConfig.Sentinel.Requests, Limits: defaultConfig.Sentinel.Limits},
		)
		redisConfig.Sentinel.Requests = mergedSentinelResources.Requests
		redisConfig.Sentinel.Limits = mergedSentinelResources.Limits
		redisConfig.Sentinel.MasterGroupName = utils.Coalesce(redisConfig.Sentinel.MasterGroupName, defaultConfig.Sentinel.MasterGroupName)
		redisConfig.Sentinel.Enabled = actual.Sentinel.Enabled
	}

	redisConfig.Enabled = actual.Enabled

	return redisConfig, nil
}

func RedisSentinelEnabled(wbSpec apiv2.WBRedisSpec) bool {
	return wbSpec.Sentinel != nil && wbSpec.Sentinel.Enabled
}

func TranslateRedisSpec(spec apiv2.WBRedisSpec) model.RedisConfig {
	config := model.RedisConfig{
		Enabled:   spec.Enabled,
		Namespace: spec.Namespace,
	}

	if spec.StorageSize != "" {
		config.StorageSize = resource.MustParse(spec.StorageSize)
	}

	if spec.Config != nil {
		config.Requests = spec.Config.Resources.Requests
		config.Limits = spec.Config.Resources.Limits
	}

	if spec.Sentinel != nil {
		config.Sentinel.Enabled = spec.Sentinel.Enabled
		config.Sentinel.ReplicaCount = model.ReplicaSentinelCount
		if spec.Sentinel.Config != nil {
			config.Sentinel.MasterGroupName = spec.Sentinel.Config.MasterName
			config.Sentinel.Requests = spec.Sentinel.Config.Resources.Requests
			config.Sentinel.Limits = spec.Sentinel.Config.Resources.Limits
		}
	}

	return config
}

func ExtractRedisStatus(ctx context.Context, results *model.Results) apiv2.WBRedisStatus {
	return TranslateRedisStatus(
		ctx,
		model.ExtractRedisStatus(ctx, results),
	)
}

func TranslateRedisStatus(ctx context.Context, model model.RedisStatus) apiv2.WBRedisStatus {
	var result apiv2.WBRedisStatus

	return result
}

func (i *InfraConfigBuilder) AddRedisConfig(actual apiv2.WBRedisSpec) *InfraConfigBuilder {
	var err error
	var size model.Size
	var defaultConfig model.RedisConfig
	var mergedConfig model.RedisConfig

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

	mergedConfig, err = BuildRedisConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedRedis = mergedConfig
	return i
}
