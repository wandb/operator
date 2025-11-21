package defaults

import (
	"fmt"

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

	DefaultKafkaName = "wandb-kafka"
)

type KafkaConfig struct {
	Enabled           bool
	Namespace         string
	Name              string
	StorageSize       string
	Replicas          int32
	Resources         corev1.ResourceRequirements
	ReplicationConfig KafkaReplicationConfig
}

type KafkaReplicationConfig struct {
	DefaultReplicationFactor int32
	MinInSyncReplicas        int32
	OffsetsTopicRF           int32
	TransactionStateRF       int32
	TransactionStateISR      int32
}

// GetKafkaReplicationConfig returns replication settings based on replica count.
// For single replica (dev mode), all factors are 1.
// For multi-replica (HA mode), uses standard HA settings.
func GetKafkaReplicationConfig(replicas int32) KafkaReplicationConfig {
	if replicas == 1 {
		return KafkaReplicationConfig{
			DefaultReplicationFactor: 1,
			MinInSyncReplicas:        1,
			OffsetsTopicRF:           1,
			TransactionStateRF:       1,
			TransactionStateISR:      1,
		}
	}
	// Multi-replica HA configuration
	minISR := int32(2)
	if replicas < 3 {
		minISR = 1
	}
	return KafkaReplicationConfig{
		DefaultReplicationFactor: min32(replicas, 3),
		MinInSyncReplicas:        minISR,
		OffsetsTopicRF:           min32(replicas, 3),
		TransactionStateRF:       min32(replicas, 3),
		TransactionStateISR:      minISR,
	}
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func BuildKafkaDefaults(size Size, ownerNamespace string) (KafkaConfig, error) {
	var err error
	var storageSize string
	config := KafkaConfig{
		Enabled:   true,
		Namespace: ownerNamespace,
		Name:      DefaultKafkaName,
	}

	switch size {
	case SizeDev:
		storageSize = DevKafkaStorageSize
		config.StorageSize = storageSize
		config.Replicas = 1
		config.ReplicationConfig = KafkaReplicationConfig{
			DefaultReplicationFactor: 1,
			MinInSyncReplicas:        1,
			OffsetsTopicRF:           1,
			TransactionStateRF:       1,
			TransactionStateISR:      1,
		}
	case SizeSmall:
		storageSize = SmallKafkaStorageSize
		config.StorageSize = storageSize
		config.Replicas = 3
		config.ReplicationConfig = KafkaReplicationConfig{
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
