package v2

import (
	"fmt"

	v2 "github.com/wandb/operator/api/v2"
	merge2 "github.com/wandb/operator/internal/controller/translator/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// Storage sizes
	DevClickHouseStorageSize   = "10Gi"
	SmallClickHouseStorageSize = "10Gi"

	// Resource requests/limits for small size
	SmallClickHouseCpuRequest    = "500m"
	SmallClickHouseCpuLimit      = "1000m"
	SmallClickHouseMemoryRequest = "1Gi"
	SmallClickHouseMemoryLimit   = "2Gi"

	// ClickHouse version
	ClickHouseVersion = "23.8"
)

// BuildClickHouseSpec will create a new WBClickHouseSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildClickHouseSpec(actual v2.WBClickHouseSpec, defaultValues v2.WBClickHouseSpec) (v2.WBClickHouseSpec, error) {
	var clickhouseSpec v2.WBClickHouseSpec

	if actual.Config == nil {
		clickhouseSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		clickhouseSpec.Config = actual.Config.DeepCopy()
	} else {
		var clickhouseConfig v2.WBClickHouseConfig
		clickhouseConfig.Resources = merge2.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		clickhouseSpec.Config = &clickhouseConfig
	}

	clickhouseSpec.StorageSize = merge2.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	clickhouseSpec.Namespace = merge2.Coalesce(actual.Namespace, defaultValues.Namespace)
	clickhouseSpec.Version = merge2.Coalesce(actual.Version, defaultValues.Version)

	clickhouseSpec.Enabled = actual.Enabled
	clickhouseSpec.Replicas = actual.Replicas

	return clickhouseSpec, nil
}

func BuildClickHouseDefaults(profile v2.WBSize, ownerNamespace string) (v2.WBClickHouseSpec, error) {
	var err error
	var storageSize string
	spec := v2.WBClickHouseSpec{
		Enabled:   true,
		Namespace: ownerNamespace,
		Version:   ClickHouseVersion,
	}

	switch profile {
	case v2.WBSizeDev:
		storageSize = DevClickHouseStorageSize
		spec.StorageSize = storageSize
		spec.Replicas = 1
	case v2.WBSizeSmall:
		storageSize = SmallClickHouseStorageSize
		spec.StorageSize = storageSize
		spec.Replicas = 3

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallClickHouseCpuRequest); err != nil {
			return spec, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallClickHouseCpuLimit); err != nil {
			return spec, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallClickHouseMemoryRequest); err != nil {
			return spec, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallClickHouseMemoryLimit); err != nil {
			return spec, err
		}

		spec.Config = &v2.WBClickHouseConfig{
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    cpuRequest,
					v1.ResourceMemory: memoryRequest,
				},
				Limits: v1.ResourceList{
					v1.ResourceCPU:    cpuLimit,
					v1.ResourceMemory: memoryLimit,
				},
			},
		}
	default:
		return spec, fmt.Errorf("unsupported size for ClickHouse: %s (only 'dev' and 'small' are supported)", profile)
	}

	return spec, nil
}
