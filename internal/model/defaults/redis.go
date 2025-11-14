package defaults

import (
	"fmt"

	v2 "github.com/wandb/operator/api/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// ReplicaSentinelCount is applicable when sentinel-mode is ON -- pod count of redis replicas and sentinels
	ReplicaSentinelCount = 3

	DefaultSentinelGroup = "gorilla"

	DevStorageRequest = "100Mi"

	DefaultNamespace = "default"

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

func Redis(profile v2.WBSize) (v2.WBRedisSpec, error) {
	var err error
	var storageRequest, cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
	var spec v2.WBRedisSpec
	var sentinelSpec *v2.WBRedisSentinelSpec

	if sentinelSpec, err = _wbRedisSentinelSpecDefaults(profile); err != nil {
		return spec, err
	}
	spec = v2.WBRedisSpec{
		Enabled: true,
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
		break
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
		break
	default:
		return spec, fmt.Errorf("invalid profile: %v", profile)
	}

	if !storageRequest.IsZero() {
		spec.Config.Resources.Requests[v1.ResourceStorage] = storageRequest
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

func _wbRedisSentinelSpecDefaults(profile v2.WBSize) (*v2.WBRedisSentinelSpec, error) {
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
		break
	default:
		return nil, fmt.Errorf("invalid profile: %v", profile)
	}

	sentinelSpec := v2.WBRedisSentinelSpec{
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
