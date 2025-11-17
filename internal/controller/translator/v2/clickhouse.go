package v2

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	merge2 "github.com/wandb/operator/internal/controller/translator/utils"
	corev1 "k8s.io/api/core/v1"
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
func BuildClickHouseSpec(actual apiv2.WBClickHouseSpec, defaultValues apiv2.WBClickHouseSpec) (apiv2.WBClickHouseSpec, error) {
	var clickhouseSpec apiv2.WBClickHouseSpec

	if actual.Config == nil {
		clickhouseSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		clickhouseSpec.Config = actual.Config.DeepCopy()
	} else {
		var clickhouseConfig apiv2.WBClickHouseConfig
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

func BuildClickHouseDefaults(profile apiv2.WBSize, ownerNamespace string) (apiv2.WBClickHouseSpec, error) {
	var err error
	var storageSize string
	spec := apiv2.WBClickHouseSpec{
		Enabled:   true,
		Namespace: ownerNamespace,
		Version:   ClickHouseVersion,
	}

	switch profile {
	case apiv2.WBSizeDev:
		storageSize = DevClickHouseStorageSize
		spec.StorageSize = storageSize
		spec.Replicas = 1
	case apiv2.WBSizeSmall:
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

		spec.Config = &apiv2.WBClickHouseConfig{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    cpuRequest,
					corev1.ResourceMemory: memoryRequest,
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    cpuLimit,
					corev1.ResourceMemory: memoryLimit,
				},
			},
		}
	default:
		return spec, fmt.Errorf("unsupported size for ClickHouse: %s (only 'dev' and 'small' are supported)", profile)
	}

	return spec, nil
}
