package v2

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	merge2 "github.com/wandb/operator/internal/controller/translator/utils"
	corev1 "k8s.io/api/core/v1"
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
func BuildKafkaSpec(actual apiv2.WBKafkaSpec, defaultValues apiv2.WBKafkaSpec) (apiv2.WBKafkaSpec, error) {
	var kafkaSpec apiv2.WBKafkaSpec

	if actual.Config == nil {
		kafkaSpec.Config = defaultValues.Config.DeepCopy()
	} else if defaultValues.Config == nil {
		kafkaSpec.Config = actual.Config.DeepCopy()
	} else {
		var kafkaConfig apiv2.WBKafkaConfig
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

func BuildKafkaDefaults(profile apiv2.WBSize, ownerNamespace string) (apiv2.WBKafkaSpec, error) {
	var err error
	var storageSize string
	var spec apiv2.WBKafkaSpec

	spec = apiv2.WBKafkaSpec{
		Enabled:   true,
		Namespace: ownerNamespace,
	}

	switch profile {
	case apiv2.WBSizeDev:
		storageSize = DevKafkaStorageSize
		spec.StorageSize = storageSize
	case apiv2.WBSizeSmall:
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

		spec.Config = &apiv2.WBKafkaConfig{
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
		return spec, fmt.Errorf("unsupported size for Kafka: %s (only 'dev' and 'small' are supported)", profile)
	}

	return spec, nil
}
