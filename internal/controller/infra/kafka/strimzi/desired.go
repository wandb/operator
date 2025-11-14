package strimzi

import (
	"context"
	"fmt"
	"strconv"

	strimziv1beta2 "github.com/wandb/operator/api/strimzi-kafka-vendored/v1beta2"
	"github.com/wandb/operator/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// buildDesiredKafka creates a Kafka CR for Strimzi.
// In KRaft mode with node pools, the Kafka.Spec.Kafka.Replicas MUST be 0.
func buildDesiredKafka(
	ctx context.Context,
	kafkaConfig model.KafkaConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*strimziv1beta2.Kafka, *model.Results) {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	kafka := &strimziv1beta2.Kafka{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KafkaName,
			Namespace: kafkaConfig.Namespace,
			Labels: map[string]string{
				"app": KafkaName,
			},
			Annotations: map[string]string{
				"strimzi.io/node-pools": "enabled",
			},
		},
		Spec: strimziv1beta2.KafkaSpec{
			Kafka: strimziv1beta2.KafkaClusterSpec{
				Version:         KafkaVersion,
				MetadataVersion: KafkaMetadataVersion,
				Replicas:        0, // CRITICAL: Must be 0 when using node pools in KRaft mode
				Listeners: []strimziv1beta2.GenericKafkaListener{
					{
						Name: PlainListenerName,
						Port: PlainListenerPort,
						Type: ListenerType,
						Tls:  false,
					},
					{
						Name: TLSListenerName,
						Port: TLSListenerPort,
						Type: ListenerType,
						Tls:  true,
					},
				},
				Config: buildKafkaConfig(kafkaConfig),
			},
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, kafka, scheme); err != nil {
		log.Error(err, "failed to set owner reference on Kafka CR")
		results.AddErrors(model.NewKafkaError(model.KafkaErrFailedToCreate, fmt.Sprintf("failed to set owner reference: %v", err)))
		return nil, results
	}

	return kafka, results
}

// buildKafkaConfig generates the Kafka configuration map based on replication settings
func buildKafkaConfig(kafkaConfig model.KafkaConfig) map[string]string {
	return map[string]string{
		"offsets.topic.replication.factor":         strconv.Itoa(int(kafkaConfig.ReplicationConfig.OffsetsTopicRF)),
		"transaction.state.log.replication.factor": strconv.Itoa(int(kafkaConfig.ReplicationConfig.TransactionStateRF)),
		"transaction.state.log.min.isr":            strconv.Itoa(int(kafkaConfig.ReplicationConfig.TransactionStateISR)),
		"default.replication.factor":               strconv.Itoa(int(kafkaConfig.ReplicationConfig.DefaultReplicationFactor)),
		"min.insync.replicas":                      strconv.Itoa(int(kafkaConfig.ReplicationConfig.MinInSyncReplicas)),
	}
}

// buildDesiredNodePool creates a KafkaNodePool CR for Strimzi.
// The node pool contains the actual replica count and runs in KRaft mode with broker and controller roles.
func buildDesiredNodePool(
	ctx context.Context,
	kafkaConfig model.KafkaConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*strimziv1beta2.KafkaNodePool, *model.Results) {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	nodePool := &strimziv1beta2.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NodePoolName,
			Namespace: kafkaConfig.Namespace,
			Labels: map[string]string{
				"strimzi.io/cluster": KafkaName,
			},
		},
		Spec: strimziv1beta2.KafkaNodePoolSpec{
			Replicas: kafkaConfig.Replicas,
			Roles:    []string{RoleBroker, RoleController},
			Storage: strimziv1beta2.KafkaStorage{
				Type: "jbod",
				Volumes: []strimziv1beta2.StorageVolume{
					{
						ID:          0,
						Type:        StorageType,
						Size:        kafkaConfig.StorageSize,
						DeleteClaim: StorageDeleteClaim,
					},
				},
			},
		},
	}

	// Add resources if specified
	if len(kafkaConfig.Resources.Requests) > 0 || len(kafkaConfig.Resources.Limits) > 0 {
		nodePool.Spec.Resources = &kafkaConfig.Resources
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, nodePool, scheme); err != nil {
		log.Error(err, "failed to set owner reference on KafkaNodePool CR")
		results.AddErrors(model.NewKafkaError(model.KafkaErrFailedToCreate, fmt.Sprintf("failed to set owner reference: %v", err)))
		return nil, results
	}

	return nodePool, results
}
