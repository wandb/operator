package v2

import (
	"fmt"

	v2 "github.com/wandb/operator/api/v2"
	merge2 "github.com/wandb/operator/internal/controller/translator/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DevMySQLStorageSize   = "1Gi"
	SmallMySQLStorageSize = "10Gi"

	SmallMySQLCpuRequest    = "500m"
	SmallMySQLCpuLimit      = "1000m"
	SmallMySQLMemoryRequest = "1Gi"
	SmallMySQLMemoryLimit   = "2Gi"
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

func BuildMySQLDefaults(profile v2.WBSize, ownerNamespace string) (v2.WBMySQLSpec, error) {
	var err error
	var storageSize string
	spec := v2.WBMySQLSpec{
		Enabled:   true,
		Namespace: ownerNamespace,
	}

	switch profile {
	case v2.WBSizeDev:
		storageSize = DevMySQLStorageSize
		spec.StorageSize = storageSize
	case v2.WBSizeSmall:
		storageSize = SmallMySQLStorageSize
		spec.StorageSize = storageSize

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallMySQLCpuRequest); err != nil {
			return spec, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallMySQLCpuLimit); err != nil {
			return spec, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallMySQLMemoryRequest); err != nil {
			return spec, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallMySQLMemoryLimit); err != nil {
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
