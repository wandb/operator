package v2

import (
	"context"
	"fmt"
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	MetricsReporterType = "strimziMetricsReporter"
)

// createKafkaMetricsConfig creates a MetricsConfig for Kafka if telemetry is enabled.
// Uses the Strimzi Metrics Reporter which exposes metrics in Prometheus format.
// Returns nil if telemetry is disabled.
func createKafkaMetricsConfig(telemetry apiv2.Telemetry) *v1.MetricsConfig {
	if !telemetry.Enabled {
		return nil
	}

	return &v1.MetricsConfig{
		Type: MetricsReporterType,
		Values: &v1.MetricsReporterValues{
			AllowList: []string{".*"},
		},
	}
}

// ToKafkaVendorSpec converts a WBKafkaSpec to a Kafka CR.
// This function translates the high-level Kafka spec into the vendor-specific
// Kafka format used by the Strimzi operator.
// Note: In KRaft mode with node pools, the Kafka.Spec.Kafka.Replicas MUST be 0.
func ToKafkaVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*v1.Kafka, error) {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)

	infraSpec := wandb.Spec.Kafka
	if !infraSpec.Enabled {
		log.Debug("Kafka is disabled, no vendor spec")
		return nil, nil
	}

	nsnBuilder := strimzi.CreateNsNameBuilder(types.NamespacedName{
		Namespace: infraSpec.Namespace, Name: infraSpec.Name,
	})

	kafka := &v1.Kafka{
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
		Spec: v1.KafkaSpec{
			Kafka: v1.KafkaClusterSpec{
				Version:         translator.KafkaVersion,
				MetadataVersion: translator.KafkaMetadataVersion,
				Replicas:        0, // CRITICAL: Must be 0 when using node pools in KRaft mode
				Listeners: []v1.GenericKafkaListener{
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
					"offsets.topic.replication.factor":         strconv.Itoa(int(infraSpec.Config.ReplicationConfig.OffsetsTopicRF)),
					"transaction.state.log.replication.factor": strconv.Itoa(int(infraSpec.Config.ReplicationConfig.TransactionStateRF)),
					"transaction.state.log.min.isr":            strconv.Itoa(int(infraSpec.Config.ReplicationConfig.TransactionStateISR)),
					"default.replication.factor":               strconv.Itoa(int(infraSpec.Config.ReplicationConfig.DefaultReplicationFactor)),
					"min.insync.replicas":                      strconv.Itoa(int(infraSpec.Config.ReplicationConfig.MinInSyncReplicas)),
				},
				Template: &v1.KafkaClusterTemplate{
					Pod: &v1.PodTemplate{
						Affinity:    wandb.GetAffinity(infraSpec.WBInfraSpec),
						Tolerations: *wandb.GetTolerations(infraSpec.WBInfraSpec),
					},
				},
			},
			EntityOperator: &v1.EntityOperatorSpec{
				TopicOperator: &v1.EntityTopicOperatorSpec{WatchedNamespace: nsnBuilder.Namespace()},
				UserOperator:  &v1.EntityUserOperatorSpec{WatchedNamespace: nsnBuilder.Namespace()},
				Template: &v1.EntityOperatorTemplate{
					Pod: &v1.PodTemplate{
						Affinity:    wandb.GetAffinity(infraSpec.WBInfraSpec),
						Tolerations: *wandb.GetTolerations(infraSpec.WBInfraSpec),
					},
				},
			},
		},
	}

	// Add metrics configuration if telemetry is enabled
	kafka.Spec.Kafka.MetricsConfig = createKafkaMetricsConfig(infraSpec.Telemetry)

	// Set owner reference
	if err := ctrl.SetControllerReference(wandb, kafka, scheme); err != nil {
		log.Error("failed to set owner reference on Kafka CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}
	log.Info("Successfully set owner reference on Kafka CR")

	log.Debug("Kafka is enabled, providing vendor spec")
	return kafka, nil
}

// ToKafkaNodePoolVendorSpec converts a WBKafkaSpec to a KafkaNodePool CR.
// This function creates the node pool that contains the actual replica count
// and runs in KRaft mode with broker and controller roles.
func ToKafkaNodePoolVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*v1.KafkaNodePool, error) {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)

	infraSpec := wandb.Spec.Kafka
	if !infraSpec.Enabled {
		return nil, nil
	}

	retentionPolicy := wandb.GetRetentionPolicy(wandb.Spec.Kafka.WBInfraSpec)
	nsnBuilder := strimzi.CreateNsNameBuilder(types.NamespacedName{
		Namespace: infraSpec.Namespace, Name: infraSpec.Name,
	})

	onDeletePurge := false
	if retentionPolicy.OnDelete == apiv2.WBPurgeOnDelete {
		onDeletePurge = true
	}

	nodePool := &v1.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.NodePoolName(),
			Namespace: nsnBuilder.Namespace(),
			Labels: map[string]string{
				"strimzi.io/cluster": nsnBuilder.KafkaName(),
			},
		},
		Spec: v1.KafkaNodePoolSpec{
			Replicas: infraSpec.Replicas,
			Roles:    []string{strimzi.RoleBroker, strimzi.RoleController},
			Storage: v1.KafkaStorage{
				Type: "jbod",
				Volumes: []v1.StorageVolume{
					{
						ID:          0,
						Type:        strimzi.StorageType,
						Size:        infraSpec.StorageSize,
						DeleteClaim: onDeletePurge,
					},
				},
			},
		},
	}

	if infraSpec.SkipDataRecovery {
		nodePool.Annotations["wandb.apps.wandb.com/skipDataRecovery"] = "true"
	}

	// Add resources if specified
	if len(infraSpec.Config.Resources.Requests) > 0 || len(infraSpec.Config.Resources.Limits) > 0 {
		nodePool.Spec.Resources = &infraSpec.Config.Resources
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(wandb, nodePool, scheme); err != nil {
		log.Error("failed to set owner reference on KafkaNodePool CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return nodePool, nil
}
