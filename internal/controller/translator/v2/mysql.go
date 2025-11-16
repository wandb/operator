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
	devMySQLStorageSize   = "1Gi"
	smallMySQLStorageSize = "10Gi"

	// Resource requests/limits for small size
	smallMySQLCpuRequest    = "500m"
	smallMySQLCpuLimit      = "1000m"
	smallMySQLMemoryRequest = "1Gi"
	smallMySQLMemoryLimit   = "2Gi"
)

// BuildMySQLSpec will create a new WBMySQLSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildMySQLSpec(actual v2.WBMySQLSpec, defaultValues v2.WBMySQLSpec) (v2.WBMySQLSpec, error) {
	var mysqlSpec v2.WBMySQLSpec

	if actual.Config == nil {
		mysqlSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		mysqlSpec.Config = actual.Config.DeepCopy()
	} else {
		var mysqlConfig v2.WBMySQLConfig
		mysqlConfig.Resources = merge2.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		mysqlSpec.Config = &mysqlConfig
	}

	mysqlSpec.StorageSize = merge2.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	mysqlSpec.Namespace = merge2.Coalesce(actual.Namespace, defaultValues.Namespace)

	mysqlSpec.Enabled = actual.Enabled

	return mysqlSpec, nil
}

func BuildMySQLDefaults(profile v2.WBSize) (v2.WBMySQLSpec, error) {
	var err error
	var storageSize string
	spec := v2.WBMySQLSpec{
		Enabled:   true,
		Namespace: defaultNamespace,
	}

	switch profile {
	case v2.WBSizeDev:
		storageSize = devMySQLStorageSize
		spec.StorageSize = storageSize
	case v2.WBSizeSmall:
		storageSize = smallMySQLStorageSize
		spec.StorageSize = storageSize

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(smallMySQLCpuRequest); err != nil {
			return spec, err
		}
		if cpuLimit, err = resource.ParseQuantity(smallMySQLCpuLimit); err != nil {
			return spec, err
		}
		if memoryRequest, err = resource.ParseQuantity(smallMySQLMemoryRequest); err != nil {
			return spec, err
		}
		if memoryLimit, err = resource.ParseQuantity(smallMySQLMemoryLimit); err != nil {
			return spec, err
		}

		spec.Config = &v2.WBMySQLConfig{
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
		return spec, fmt.Errorf("unsupported size for MySQL: %s (only 'dev' and 'small' are supported)", profile)
	}

	return spec, nil
}
