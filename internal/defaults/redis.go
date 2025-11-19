package defaults

import (
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
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

func BuildRedisDefaults(size common.Size, ownerNamespace string) (common.RedisConfig, error) {
	var err error
	var storageRequest, cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
	config := common.RedisConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
		Requests:  corev1.ResourceList{},
		Limits:    corev1.ResourceList{},
	}

	switch size {
	case common.SizeDev:
		if storageRequest, err = resource.ParseQuantity(DevStorageRequest); err != nil {
			return config, err
		}
		config.StorageSize = storageRequest
	case common.SizeSmall:
		if storageRequest, err = resource.ParseQuantity(SmallStorageRequest); err != nil {
			return config, err
		}
		if cpuRequest, err = resource.ParseQuantity(SmallReplicaCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallReplicaCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallReplicaMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallReplicaMemoryLimit); err != nil {
			return config, err
		}

		config.StorageSize = storageRequest
		config.Requests[corev1.ResourceCPU] = cpuRequest
		config.Limits[corev1.ResourceCPU] = cpuLimit
		config.Requests[corev1.ResourceMemory] = memoryRequest
		config.Limits[corev1.ResourceMemory] = memoryLimit

		var sentinelCpuRequest, sentinelCpuLimit, sentinelMemoryRequest, sentinelMemoryLimit resource.Quantity
		if sentinelCpuRequest, err = resource.ParseQuantity(SmallSentinelCpuRequest); err != nil {
			return config, err
		}
		if sentinelCpuLimit, err = resource.ParseQuantity(SmallSentinelCpuLimit); err != nil {
			return config, err
		}
		if sentinelMemoryRequest, err = resource.ParseQuantity(SmallSentinelMemoryRequest); err != nil {
			return config, err
		}
		if sentinelMemoryLimit, err = resource.ParseQuantity(SmallSentinelMemoryLimit); err != nil {
			return config, err
		}

		config.Sentinel = common.SentinelConfig{
			Enabled:         true,
			MasterGroupName: DefaultSentinelGroup,
			ReplicaCount:    ReplicaSentinelCount,
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    sentinelCpuRequest,
				corev1.ResourceMemory: sentinelMemoryRequest,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    sentinelCpuLimit,
				corev1.ResourceMemory: sentinelMemoryLimit,
			},
		}
	default:
		return config, fmt.Errorf("invalid profile: %v", size)
	}

	return config, nil
}
