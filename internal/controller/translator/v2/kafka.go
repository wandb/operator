package v2

import (
	"context"
	"fmt"
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator/common"
	kafkav1beta2 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func ExtractKafkaStatus(ctx context.Context, conditions []common.KafkaCondition) apiv2.WBKafkaStatus {
	return TranslateKafkaStatus(
		ctx,
		common.ExtractKafkaStatus(ctx, conditions),
	)
}

func TranslateKafkaStatus(ctx context.Context, m common.KafkaStatus) apiv2.WBKafkaStatus {
	var result apiv2.WBKafkaStatus
	var conditions []apiv2.WBStatusCondition

	for _, condition := range m.Conditions {
		state := translateKafkaStatusCode(condition.Code())
		conditions = append(conditions, apiv2.WBStatusCondition{
			State:   state,
			Code:    condition.Code(),
			Message: condition.Message(),
		})
	}

	result.Connection = apiv2.WBKafkaConnection{
		KafkaHost: m.Connection.Host,
		KafkaPort: m.Connection.Port,
	}

	result.Ready = m.Ready
	result.Conditions = conditions
	result.State = computeOverallState(conditions, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateKafkaStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(common.KafkaCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.KafkaUpdatedCode):
		return apiv2.WBStateUpdating
	case string(common.KafkaDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.KafkaNodePoolCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.KafkaNodePoolUpdatedCode):
		return apiv2.WBStateUpdating
	case string(common.KafkaNodePoolDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.KafkaConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
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
) (*kafkav1beta2.Kafka, error) {
	log := ctrl.LoggerFrom(ctx)

	if !spec.Enabled {
		return nil, nil
	}

	nsNameBldr := strimzi.CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	kafka := &kafkav1beta2.Kafka{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsNameBldr.KafkaName(),
			Namespace: nsNameBldr.Namespace(),
			Labels: map[string]string{
				"app": nsNameBldr.KafkaName(),
			},
			Annotations: map[string]string{
				"strimzi.io/node-pools": "enabled",
			},
		},
		Spec: kafkav1beta2.KafkaSpec{
			Kafka: kafkav1beta2.KafkaClusterSpec{
				Version:         common.KafkaVersion,
				MetadataVersion: common.KafkaMetadataVersion,
				Replicas:        0, // CRITICAL: Must be 0 when using node pools in KRaft mode
				Listeners: []kafkav1beta2.GenericKafkaListener{
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
) (*kafkav1beta2.KafkaNodePool, error) {
	log := ctrl.LoggerFrom(ctx)

	nsNameBldr := strimzi.CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	nodePool := &kafkav1beta2.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsNameBldr.NodePoolName(),
			Namespace: nsNameBldr.Namespace(),
			Labels: map[string]string{
				"strimzi.io/cluster": nsNameBldr.KafkaName(),
			},
		},
		Spec: kafkav1beta2.KafkaNodePoolSpec{
			Replicas: spec.Replicas,
			Roles:    []string{strimzi.RoleBroker, strimzi.RoleController},
			Storage: kafkav1beta2.KafkaStorage{
				Type: "jbod",
				Volumes: []kafkav1beta2.StorageVolume{
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
