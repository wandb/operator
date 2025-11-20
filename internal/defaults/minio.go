package defaults

import (
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DevMinioStorageSize   = "10Gi"
	SmallMinioStorageSize = "10Gi"

	SmallMinioCpuRequest    = "500m"
	SmallMinioCpuLimit      = "1000m"
	SmallMinioMemoryRequest = "1Gi"
	SmallMinioMemoryLimit   = "2Gi"

	MinioImage = "quay.io/minio/minio:latest"
	MinioName  = "wandb-minio"
)

func BuildMinioDefaults(size Size, ownerNamespace string) (common.MinioConfig, error) {
	var err error
	var storageSize string
	config := common.MinioConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
		Name:      MinioName,
		Image:     MinioImage,
	}

	switch size {
	case SizeDev:
		storageSize = DevMinioStorageSize
		config.StorageSize = storageSize
		config.Servers = 1
		config.VolumesPerServer = 1
	case SizeSmall:
		storageSize = SmallMinioStorageSize
		config.StorageSize = storageSize
		config.Servers = 3
		config.VolumesPerServer = 4

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallMinioCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallMinioCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallMinioMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallMinioMemoryLimit); err != nil {
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
		return config, fmt.Errorf("unsupported size for Minio: %s (only 'dev' and 'small' are supported)", size)
	}

	return config, nil
}
