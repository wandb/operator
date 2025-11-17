package v2

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	merge2 "github.com/wandb/operator/internal/controller/translator/utils"
	corev1 "k8s.io/api/core/v1"
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
func BuildMySQLSpec(actual apiv2.WBMySQLSpec, defaultValues apiv2.WBMySQLSpec) (apiv2.WBMySQLSpec, error) {
	var mysqlSpec apiv2.WBMySQLSpec

	if actual.Config == nil {
		mysqlSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		mysqlSpec.Config = actual.Config.DeepCopy()
	} else {
		var mysqlConfig apiv2.WBMySQLConfig
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

func BuildMySQLDefaults(profile apiv2.WBSize, ownerNamespace string) (apiv2.WBMySQLSpec, error) {
	var err error
	var storageSize string
	spec := apiv2.WBMySQLSpec{
		Enabled:   true,
		Namespace: ownerNamespace,
	}

	switch profile {
	case apiv2.WBSizeDev:
		storageSize = DevMySQLStorageSize
		spec.StorageSize = storageSize
	case apiv2.WBSizeSmall:
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

		spec.Config = &apiv2.WBMySQLConfig{
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
		return spec, fmt.Errorf("unsupported size for MySQL: %s (only 'dev' and 'small' are supported)", profile)
	}

	return spec, nil
}
