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
	devMinioStorageSize   = "10Gi"
	smallMinioStorageSize = "10Gi"

	// Resource requests/limits for small size
	smallMinioCpuRequest    = "500m"
	smallMinioCpuLimit      = "1000m"
	smallMinioMemoryRequest = "1Gi"
	smallMinioMemoryLimit   = "2Gi"
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

func BuildMinioDefaults(profile v2.WBSize) (v2.WBMinioSpec, error) {
	var err error
	var storageSize string
	spec := v2.WBMinioSpec{
		Enabled:   true,
		Namespace: defaultNamespace,
	}

	switch profile {
	case v2.WBSizeDev:
		storageSize = devMinioStorageSize
		spec.StorageSize = storageSize
	case v2.WBSizeSmall:
		storageSize = smallMinioStorageSize
		spec.StorageSize = storageSize

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(smallMinioCpuRequest); err != nil {
			return spec, err
		}
		if cpuLimit, err = resource.ParseQuantity(smallMinioCpuLimit); err != nil {
			return spec, err
		}
		if memoryRequest, err = resource.ParseQuantity(smallMinioMemoryRequest); err != nil {
			return spec, err
		}
		if memoryLimit, err = resource.ParseQuantity(smallMinioMemoryLimit); err != nil {
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
