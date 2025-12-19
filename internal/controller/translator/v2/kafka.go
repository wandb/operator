package v2

import (
	"context"
	"fmt"
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func ToWBKafkaStatus(ctx context.Context, status translator.KafkaStatus) apiv2.WBKafkaStatus {
	return apiv2.WBKafkaStatus{
		Ready:          status.Ready,
		State:          status.State,
		Conditions:     status.Conditions,
		LastReconciled: metav1.Now(),
		Connection: apiv2.WBInfraConnection{
			URL: status.Connection.URL,
		},
	}
}

// ToKafkaVendorSpec converts a WBKafkaSpec to a Kafka CR.
// This function translates the high-level Kafka spec into the vendor-specific
// Kafka format used by the Strimzi operator.
// Note: In KRaft mode with node pools, the Kafka.Spec.Kafka.Replicas MUST be 0.
func ToKafkaVendorSpec(
	ctx context.Context,
	spec apiv2.WBKafkaSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*strimziv1.Kafka, error) {
	log := ctrl.LoggerFrom(ctx)

	if !spec.Enabled {
		return nil, nil
	}

	nsnBuilder := strimzi.CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	kafka := &strimziv1.Kafka{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.KafkaName(),
			Namespace: nsnBuilder.Namespace(),
			Labels: map[string]string{
				"app": nsnBuilder.KafkaName(),
			},
			Annotations: map[string]string{
				"strimzi.io/node-pools": "enabled",
			},
		},
		Spec: strimziv1.KafkaSpec{
			Kafka: strimziv1.KafkaClusterSpec{
				Version:         translator.KafkaVersion,
				MetadataVersion: translator.KafkaMetadataVersion,
				Replicas:        0, // CRITICAL: Must be 0 when using node pools in KRaft mode
				Listeners: []strimziv1.GenericKafkaListener{
					{
						Name: strimzi.PlainListenerName,
						Port: strimzi.PlainListenerPort,
						Type: strimzi.ListenerType,
						Tls:  false,
					},
					{
						Name: strimzi.TLSListenerName,
						Port: strimzi.TLSListenerPort,
						Type: strimzi.ListenerType,
						Tls:  true,
					},
				},
				Config: map[string]string{
					"offsets.topic.replication.factor":         strconv.Itoa(int(spec.Config.ReplicationConfig.OffsetsTopicRF)),
					"transaction.state.log.replication.factor": strconv.Itoa(int(spec.Config.ReplicationConfig.TransactionStateRF)),
					"transaction.state.log.min.isr":            strconv.Itoa(int(spec.Config.ReplicationConfig.TransactionStateISR)),
					"default.replication.factor":               strconv.Itoa(int(spec.Config.ReplicationConfig.DefaultReplicationFactor)),
					"min.insync.replicas":                      strconv.Itoa(int(spec.Config.ReplicationConfig.MinInSyncReplicas)),
				},
			},
			EntityOperator: &strimziv1.EntityOperatorSpec{
				TopicOperator: &strimziv1.EntityTopicOperatorSpec{WatchedNamespace: nsnBuilder.Namespace()},
				UserOperator:  &strimziv1.EntityUserOperatorSpec{WatchedNamespace: nsnBuilder.Namespace()},
			},
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, kafka, scheme); err != nil {
		log.Error(err, "failed to set owner reference on Kafka CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return kafka, nil
}

// ToKafkaNodePoolVendorSpec converts a WBKafkaSpec to a KafkaNodePool CR.
// This function creates the node pool that contains the actual replica count
// and runs in KRaft mode with broker and controller roles.
func ToKafkaNodePoolVendorSpec(
	ctx context.Context,
	spec apiv2.WBKafkaSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*strimziv1.KafkaNodePool, error) {
	log := ctrl.LoggerFrom(ctx)

	if !spec.Enabled {
		return nil, nil
	}

	nsnBuilder := strimzi.CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	nodePool := &strimziv1.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.NodePoolName(),
			Namespace: nsnBuilder.Namespace(),
			Labels: map[string]string{
				"strimzi.io/cluster": nsnBuilder.KafkaName(),
			},
		},
		Spec: strimziv1.KafkaNodePoolSpec{
			Replicas: spec.Replicas,
			Roles:    []string{strimzi.RoleBroker, strimzi.RoleController},
			Storage: strimziv1.KafkaStorage{
				Type: "jbod",
				Volumes: []strimziv1.StorageVolume{
					{
						ID:          0,
						Type:        strimzi.StorageType,
						Size:        spec.StorageSize,
						DeleteClaim: strimzi.StorageDeleteClaim,
					},
				},
			},
		},
	}

	// Add resources if specified
	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		nodePool.Spec.Resources = &spec.Config.Resources
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, nodePool, scheme); err != nil {
		log.Error(err, "failed to set owner reference on KafkaNodePool CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return nodePool, nil
}
