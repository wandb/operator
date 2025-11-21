package defaults

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DevClickHouseStorageSize   = "10Gi"
	SmallClickHouseStorageSize = "10Gi"

	SmallClickHouseCpuRequest    = "500m"
	SmallClickHouseCpuLimit      = "1000m"
	SmallClickHouseMemoryRequest = "1Gi"
	SmallClickHouseMemoryLimit   = "2Gi"

	ClickHouseVersion     = "23.8"
	DefaultClickHouseName = "wandb-clickhouse"
)

type ClickHouseConfig struct {
	Enabled   bool
	Namespace string
	Name      string

	// Storage and resources
	StorageSize string
	Replicas    int32
	Version     string
	Resources   corev1.ResourceRequirements
}

func BuildClickHouseDefaults(size Size, ownerNamespace string) (ClickHouseConfig, error) {
	var err error
	var storageSize string
	config := ClickHouseConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
		Name:      DefaultClickHouseName,
		Version:   ClickHouseVersion,
	}

	switch size {
	case SizeDev:
		storageSize = DevClickHouseStorageSize
		config.StorageSize = storageSize
		config.Replicas = 1
	case SizeSmall:
		storageSize = SmallClickHouseStorageSize
		config.StorageSize = storageSize
		config.Replicas = 3

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallClickHouseCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallClickHouseCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallClickHouseMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallClickHouseMemoryLimit); err != nil {
			return config, err
		}

		config.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuRequest,
				corev1.ResourceMemory: memoryRequest,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memoryLimit,
			},
		}
	default:
		return config, fmt.Errorf("unsupported size for ClickHouse: %s (only 'dev' and 'small' are supported)", size)
	}

	return config, nil
}
