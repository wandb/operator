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
	DevMinioStorageSize   = "10Gi"
	SmallMinioStorageSize = "10Gi"

	// Resource requests/limits for small size
	SmallMinioCpuRequest    = "500m"
	SmallMinioCpuLimit      = "1000m"
	SmallMinioMemoryRequest = "1Gi"
	SmallMinioMemoryLimit   = "2Gi"
)

// BuildMinioSpec will create a new WBMinioSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMinioSpec(actual v2.WBMinioSpec, defaultValues v2.WBMinioSpec) (v2.WBMinioSpec, error) {
	var minioSpec v2.WBMinioSpec

	if actual.Config == nil {
		minioSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		minioSpec.Config = actual.Config.DeepCopy()
	} else {
		var minioConfig v2.WBMinioConfig
		minioConfig.Resources = merge2.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		minioSpec.Config = &minioConfig
	}

	minioSpec.StorageSize = merge2.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	minioSpec.Namespace = merge2.Coalesce(actual.Namespace, defaultValues.Namespace)

	minioSpec.Enabled = actual.Enabled
	minioSpec.Replicas = actual.Replicas

	return minioSpec, nil
}

func BuildMinioDefaults(profile v2.WBSize, ownerNamespace string) (v2.WBMinioSpec, error) {
	var err error
	var storageSize string
	spec := v2.WBMinioSpec{
		Enabled:   true,
		Namespace: ownerNamespace,
	}

	switch profile {
	case v2.WBSizeDev:
		storageSize = DevMinioStorageSize
		spec.StorageSize = storageSize
	case v2.WBSizeSmall:
		storageSize = SmallMinioStorageSize
		spec.StorageSize = storageSize

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallMinioCpuRequest); err != nil {
			return spec, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallMinioCpuLimit); err != nil {
			return spec, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallMinioMemoryRequest); err != nil {
			return spec, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallMinioMemoryLimit); err != nil {
			return spec, err
		}

		spec.Config = &v2.WBMinioConfig{
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
		return spec, fmt.Errorf("unsupported size for Minio: %s (only 'dev' and 'small' are supported)", profile)
	}

	return spec, nil
}
