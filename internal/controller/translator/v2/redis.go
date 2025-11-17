package v2

import (
	"fmt"

	v2 "github.com/wandb/operator/api/v2"
	merge2 "github.com/wandb/operator/internal/controller/translator/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// ReplicaSentinelCount is applicable when sentinel-mode is ON -- pod count of redis replicas and sentinels
	ReplicaSentinelCount = 3

	DefaultSentinelGroup = "gorilla"

	DevStorageRequest = "100Mi"

	SmallStorageRequest        = "2Gi"
	SmallReplicaCpuRequest     = "250m"
	SmallReplicaCpuLimit       = "500m"
	SmallReplicaMemoryRequest  = "256Mi"
	SmallReplicaMemoryLimit    = "512Mi"
	SmallSentinelCpuRequest    = "125m"
	SmallSentinelCpuLimit      = "256m"
	SmallSentinelMemoryRequest = "128Mi"
	SmallSentinelMemoryLimit   = "256Mi"
)

// BuildRedisSpec will create a new WBRedisSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildRedisSpec(actual v2.WBRedisSpec, defaultValues v2.WBRedisSpec) (v2.WBRedisSpec, error) {
	var redisSpec v2.WBRedisSpec

	if actual.Sentinel == nil {
		redisSpec.Sentinel = defaultValues.Sentinel.DeepCopy()
	} else if defaultValues.Sentinel == nil {
		redisSpec.Sentinel = actual.Sentinel.DeepCopy()
	} else {
		var redisSentinel v2.WBRedisSentinelSpec
		redisSentinel.Enabled = actual.Sentinel.Enabled
		if actual.Sentinel.Config == nil {
			redisSentinel.Config = defaultValues.Sentinel.Config.DeepCopy()
		} else if defaultValues.Sentinel.Config == nil {
			redisSentinel.Config = actual.Sentinel.Config.DeepCopy()
		} else {
			var sentinelConfig v2.WBRedisSentinelConfig
			sentinelConfig.Resources = merge2.Resources(
				actual.Sentinel.Config.Resources,
				defaultValues.Sentinel.Config.Resources,
			)
			sentinelConfig.MasterName = merge2.Coalesce(
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
		var redisConfig v2.WBRedisConfig
		redisConfig.Resources = merge2.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		redisSpec.Config = &redisConfig
	}

	redisSpec.StorageSize = merge2.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	redisSpec.Namespace = merge2.Coalesce(actual.Namespace, defaultValues.Namespace)
	redisSpec.Enabled = actual.Enabled

	return redisSpec, nil
}

func BuildRedisDefaults(profile v2.WBSize, ownerNamespace string) (v2.WBRedisSpec, error) {
	var err error
	var storageRequest, cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
	var spec v2.WBRedisSpec
	var sentinelSpec *v2.WBRedisSentinelSpec

	if sentinelSpec, err = buildRedisSentinelSpecDefaults(profile); err != nil {
		return spec, err
	}
	spec = v2.WBRedisSpec{
		Enabled:   true,
		Namespace: ownerNamespace,
		Config: &v2.WBRedisConfig{
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{},
				Limits:   v1.ResourceList{},
			},
		},
		Sentinel: sentinelSpec,
	}
	switch profile {
	case v2.WBSizeDev:
		if storageRequest, err = resource.ParseQuantity(DevStorageRequest); err != nil {
			return spec, err
		}
	case v2.WBSizeSmall:
		if storageRequest, err = resource.ParseQuantity(SmallStorageRequest); err != nil {
			return spec, err
		}
		if cpuRequest, err = resource.ParseQuantity(SmallReplicaCpuRequest); err != nil {
			return spec, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallReplicaCpuLimit); err != nil {
			return spec, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallReplicaMemoryRequest); err != nil {
			return spec, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallReplicaMemoryLimit); err != nil {
			return spec, err
		}
	default:
		return spec, fmt.Errorf("invalid profile: %v", profile)
	}

	if !storageRequest.IsZero() {
		spec.StorageSize = storageRequest.String()
	}
	if !cpuRequest.IsZero() {
		spec.Config.Resources.Requests[v1.ResourceCPU] = cpuRequest
	}
	if !cpuLimit.IsZero() {
		spec.Config.Resources.Limits[v1.ResourceCPU] = cpuLimit
	}
	if !memoryRequest.IsZero() {
		spec.Config.Resources.Requests[v1.ResourceMemory] = memoryRequest
	}
	if !memoryLimit.IsZero() {
		spec.Config.Resources.Limits[v1.ResourceMemory] = memoryLimit
	}

	return spec, nil
}

func buildRedisSentinelSpecDefaults(profile v2.WBSize) (*v2.WBRedisSentinelSpec, error) {
	var err error
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity

	switch profile {
	case v2.WBSizeDev:
		return nil, nil
	case v2.WBSizeSmall:
		if cpuRequest, err = resource.ParseQuantity(SmallSentinelCpuRequest); err != nil {
			return nil, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallSentinelCpuLimit); err != nil {
			return nil, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallSentinelMemoryRequest); err != nil {
			return nil, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallSentinelMemoryLimit); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid profile: %v", profile)
	}

	sentinelSpec := v2.WBRedisSentinelSpec{
		Enabled: true,
		Config: &v2.WBRedisSentinelConfig{
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{},
				Limits:   v1.ResourceList{},
			},
		},
	}

	if !cpuRequest.IsZero() {
		sentinelSpec.Config.Resources.Requests[v1.ResourceCPU] = cpuRequest
	}
	if !cpuLimit.IsZero() {
		sentinelSpec.Config.Resources.Limits[v1.ResourceCPU] = cpuLimit
	}
	if !memoryRequest.IsZero() {
		sentinelSpec.Config.Resources.Requests[v1.ResourceMemory] = memoryRequest
	}
	if !memoryLimit.IsZero() {
		sentinelSpec.Config.Resources.Limits[v1.ResourceMemory] = memoryLimit
	}

	return &sentinelSpec, nil
}

func RedisSentinelEnabled(wbSpec v2.WBRedisSpec) bool {
	return wbSpec.Sentinel != nil && wbSpec.Sentinel.Enabled
}
