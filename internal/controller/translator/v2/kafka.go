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

func ExtractKafkaStatus(ctx context.Context, results *common.Results) apiv2.WBKafkaStatus {
	return TranslateKafkaStatus(
		ctx,
		common.ExtractKafkaStatus(ctx, results),
	)
}

func TranslateKafkaStatus(ctx context.Context, m common.KafkaStatus) apiv2.WBKafkaStatus {
	var result apiv2.WBKafkaStatus
	var details []apiv2.WBStatusCondition

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusCondition{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateKafkaStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusCondition{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
		})
	}

	result.Connection = apiv2.WBKafkaConnection{
		KafkaHost: m.Connection.Host,
		KafkaPort: m.Connection.Port,
	}

	result.Ready = m.Ready
	result.Conditions = details
	result.State = computeOverallState(details, m.Ready)
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

	// Get replication config based on replica count
	replicationConfig := common.GetKafkaReplicationConfig(spec.Replicas)

	kafka := &kafkav1beta2.Kafka{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strimzi.KafkaName,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"app": strimzi.KafkaName,
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
					"offsets.topic.replication.factor":         strconv.Itoa(int(replicationConfig.OffsetsTopicRF)),
					"transaction.state.log.replication.factor": strconv.Itoa(int(replicationConfig.TransactionStateRF)),
					"transaction.state.log.min.isr":            strconv.Itoa(int(replicationConfig.TransactionStateISR)),
					"default.replication.factor":               strconv.Itoa(int(replicationConfig.DefaultReplicationFactor)),
					"min.insync.replicas":                      strconv.Itoa(int(replicationConfig.MinInSyncReplicas)),
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

	nodePool := &kafkav1beta2.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strimzi.NodePoolName,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				"strimzi.io/cluster": strimzi.KafkaName,
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

func KafkaNamespacedName(spec apiv2.WBKafkaSpec) types.NamespacedName {
	return types.NamespacedName{
		Name:      spec.Name,
		Namespace: spec.Namespace,
	}
}

func KafkaNodePoolNamespacedName(spec apiv2.WBKafkaSpec) types.NamespacedName {
	return types.NamespacedName{
		Name:      spec.Name + "-node-pool",
		Namespace: spec.Namespace,
	}
}
