package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/utils"
	"github.com/wandb/operator/internal/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TranslateRedisStatus(ctx context.Context, m model.RedisStatus) apiv2.WBRedisStatus {
	var result apiv2.WBRedisStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateRedisStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBRedisConnection{
		RedisHost:         m.Connection.RedisHost,
		RedisPort:         m.Connection.RedisPort,
		RedisSentinelHost: m.Connection.SentinelHost,
		RedisSentinelPort: m.Connection.SentinelPort,
		RedisMasterName:   m.Connection.SentinelMaster,
	}

	result.Ready = m.Ready
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateRedisStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(model.RedisSentinelCreatedCode):
		return apiv2.WBStateUpdating
	case string(model.RedisReplicationCreatedCode):
		return apiv2.WBStateUpdating
	case string(model.RedisStandaloneCreatedCode):
		return apiv2.WBStateUpdating
	case string(model.RedisSentinelDeletedCode):
		return apiv2.WBStateDeleting
	case string(model.RedisReplicationDeletedCode):
		return apiv2.WBStateDeleting
	case string(model.RedisStandaloneDeletedCode):
		return apiv2.WBStateDeleting
	case string(model.RedisSentinelConnectionCode):
		return apiv2.WBStateReady
	case string(model.RedisStandaloneConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
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
