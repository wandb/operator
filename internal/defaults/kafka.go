package defaults

import (
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
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

func BuildKafkaDefaults(size common.Size, ownerNamespace string) (common.KafkaConfig, error) {
	var err error
	var storageSize string
	config := common.KafkaConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
	}

	switch size {
	case common.SizeDev:
		storageSize = DevKafkaStorageSize
		config.StorageSize = storageSize
		config.Replicas = 1
		config.ReplicationConfig = common.KafkaReplicationConfig{
			DefaultReplicationFactor: 1,
			MinInSyncReplicas:        1,
			OffsetsTopicRF:           1,
			TransactionStateRF:       1,
			TransactionStateISR:      1,
		}
	case common.SizeSmall:
		storageSize = SmallKafkaStorageSize
		config.StorageSize = storageSize
		config.Replicas = 3
		config.ReplicationConfig = common.KafkaReplicationConfig{
			DefaultReplicationFactor: 3,
			MinInSyncReplicas:        2,
			OffsetsTopicRF:           3,
			TransactionStateRF:       3,
			TransactionStateISR:      2,
		}

		var cpuRequest, cpuLimit, memoryRequest, memoryLimit resource.Quantity
		if cpuRequest, err = resource.ParseQuantity(SmallKafkaCpuRequest); err != nil {
			return config, err
		}
		if cpuLimit, err = resource.ParseQuantity(SmallKafkaCpuLimit); err != nil {
			return config, err
		}
		if memoryRequest, err = resource.ParseQuantity(SmallKafkaMemoryRequest); err != nil {
			return config, err
		}
		if memoryLimit, err = resource.ParseQuantity(SmallKafkaMemoryLimit); err != nil {
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
		return config, fmt.Errorf("unsupported size for Kafka: %s (only 'dev' and 'small' are supported)", size)
	}

	return config, nil
}
