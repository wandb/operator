package v2

import (
	"fmt"

	v2 "github.com/wandb/operator/api/v2"
	merge2 "github.com/wandb/operator/internal/controller/translator/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DevKafkaStorageSize   = "1Gi"
	SmallKafkaStorageSize = "5Gi"

	SmallKafkaCpuRequest    = "500m"
	SmallKafkaCpuLimit      = "1000m"
	SmallKafkaMemoryRequest = "1Gi"
	SmallKafkaMemoryLimit   = "2Gi"
)

// BuildKafkaSpec will create a new WBKafkaSpec with defaultValues applied if not
// present in actual. It should *never* be saved into the CR!
func BuildKafkaSpec(actual v2.WBKafkaSpec, defaultValues v2.WBKafkaSpec) (v2.WBKafkaSpec, error) {
	var kafkaSpec v2.WBKafkaSpec

	if actual.Config == nil {
		kafkaSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		kafkaSpec.Config = actual.Config.DeepCopy()
	} else {
		var kafkaConfig v2.WBKafkaConfig
		kafkaConfig.Resources = merge2.Resources(
			actual.Config.Resources,
			defaultValues.Config.Resources,
		)
		kafkaSpec.Config = &kafkaConfig
	}

	kafkaSpec.StorageSize = merge2.CoalesceQuantity(actual.StorageSize, defaultValues.StorageSize)
	kafkaSpec.Namespace = merge2.Coalesce(actual.Namespace, defaultValues.Namespace)

	kafkaSpec.Enabled = actual.Enabled

	return kafkaSpec, nil
}

func BuildKafkaDefaults(profile v2.WBSize, ownerNamespace string) (v2.WBKafkaSpec, error) {
	var err error
	var storageSize string
	var spec v2.WBKafkaSpec

	spec = v2.WBKafkaSpec{
		Enabled:   true,
		Namespace: ownerNamespace,
	}

	switch profile {
	case v2.WBSizeDev:
		storageSize = DevKafkaStorageSize
		spec.StorageSize = storageSize
	case v2.WBSizeSmall:
		storageSize = SmallKafkaStorageSize
		spec.StorageSize = storageSize

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallKafkaCpuRequest); err != nil {
			return spec, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallKafkaCpuLimit); err != nil {
			return spec, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallKafkaMemoryRequest); err != nil {
			return spec, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallKafkaMemoryLimit); err != nil {
			return spec, err
		}

		spec.Config = &v2.WBKafkaConfig{
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
		return spec, fmt.Errorf("unsupported size for Kafka: %s (only 'dev' and 'small' are supported)", profile)
	}

	return spec, nil
}
