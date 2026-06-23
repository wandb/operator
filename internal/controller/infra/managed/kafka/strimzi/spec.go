package strimzi

import (
	"context"
	"fmt"
	"os"
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	v1 "github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	KafkaModuleName      = "kafka"
	KafkaVersion         = "4.1.0"
	KafkaMetadataVersion = "4.1-IV0"

	// TODO: remove this hardcoded default once all supported manifest versions
	// supply kafka.image.
	defaultKafkaImage = "quay.io/strimzi/kafka:0.49.1-kafka-4.1.0"
)

func KafkaImage(img manifest.ImageRef) string {
	globalImageRegistry := "" // TODO: source from wandb.Spec.Global.ImageRegistry once that field exists.
	if out := img.GetImage(globalImageRegistry); out != "" {
		return out
	}
	// Fallback for older manifests that don't supply the image.
	return defaultKafkaImage
}

const (
	MetricsReporterType = "strimziMetricsReporter"
)

const (
	kafkaRunAsUser  int64 = 1001
	kafkaRunAsGroup int64 = 1001

	kafkaCapabilityAll corev1.Capability = "ALL"
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

// kafkaPodSecurityContext keeps the optional KAFKA_FSGROUP override while
// hardening Strimzi pods with the UID/GID used by the Strimzi images.
func kafkaPodSecurityContext() *corev1.PodSecurityContext {
	if utils.IsOpenShift() {
		return &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.To(true),
			SeccompProfile: kafkaRuntimeDefaultSeccompProfile(),
		}
	}

	securityContext := &corev1.PodSecurityContext{
		RunAsUser:      ptr.To(kafkaRunAsUser),
		RunAsGroup:     ptr.To(kafkaRunAsGroup),
		RunAsNonRoot:   ptr.To(true),
		SeccompProfile: kafkaRuntimeDefaultSeccompProfile(),
	}
	val, ok := os.LookupEnv("KAFKA_FSGROUP")
	if ok {
		if fsGroup, err := strconv.ParseInt(val, 10, 64); err == nil {
			securityContext.FSGroup = ptr.To(fsGroup)
		}
	}

	return securityContext
}

func kafkaContainerSecurityContext() *corev1.SecurityContext {
	securityContext := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{kafkaCapabilityAll},
		},
		SeccompProfile: kafkaRuntimeDefaultSeccompProfile(),
	}
	if !utils.IsOpenShift() {
		securityContext.RunAsUser = ptr.To(kafkaRunAsUser)
		securityContext.RunAsGroup = ptr.To(kafkaRunAsGroup)
	}
	return securityContext
}

func kafkaRuntimeDefaultSeccompProfile() *corev1.SeccompProfile {
	return &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
}

// ToKafkaVendorSpec converts a KafkaSpec to a Kafka CR.
// This function translates the high-level Kafka spec into the vendor-specific
// Kafka format used by the Strimzi operator.
// Note: In KRaft mode with node pools, the Kafka.Spec.Kafka.Replicas MUST be 0.
func ToKafkaVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
	mfst manifest.Manifest,
) (*v1.Kafka, error) {
	_, log := logx.WithSlog(ctx, logx.Kafka)

	infraSpec := wandb.Spec.Kafka.ManagedKafka
	if infraSpec == nil {
		log.Debug("Kafka is disabled, no vendor spec")
		return nil, nil
	}

	nsnBuilder := CreateNsNameBuilder(types.NamespacedName{
		Namespace: infraSpec.Namespace, Name: infraSpec.Name,
	})
	kafkaLabels := BuildWandbKafkaLabels(wandb)
	kafkaLabels["app"] = nsnBuilder.KafkaName()

	kafka := &v1.Kafka{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.KafkaName(),
			Namespace: nsnBuilder.Namespace(),
			Labels:    kafkaLabels,
			Annotations: map[string]string{
				"strimzi.io/node-pools": "enabled",
			},
		},
		Spec: v1.KafkaSpec{
			Kafka: v1.KafkaClusterSpec{
				Version:         KafkaVersion,
				MetadataVersion: KafkaMetadataVersion,
				Image:           KafkaImage(mfst.Kafka.Image),
				Replicas:        0, // CRITICAL: Must be 0 when using node pools in KRaft mode
				Listeners: []v1.GenericKafkaListener{
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
				Config: map[string]string{
					"offsets.topic.replication.factor":         strconv.Itoa(int(infraSpec.Config.ReplicationConfig.OffsetsTopicRF)),
					"transaction.state.log.replication.factor": strconv.Itoa(int(infraSpec.Config.ReplicationConfig.TransactionStateRF)),
					"transaction.state.log.min.isr":            strconv.Itoa(int(infraSpec.Config.ReplicationConfig.TransactionStateISR)),
					"default.replication.factor":               strconv.Itoa(int(infraSpec.Config.ReplicationConfig.DefaultReplicationFactor)),
					"min.insync.replicas":                      strconv.Itoa(int(infraSpec.Config.ReplicationConfig.MinInSyncReplicas)),
				},
				Template: &v1.KafkaClusterTemplate{
					Pod: &v1.PodTemplate{
						Metadata: &v1.MetadataTemplate{
							Labels: BuildWandbKafkaLabels(wandb),
						},
						Affinity:        wandb.GetAffinity(infraSpec.ManagedInfraSpec),
						Tolerations:     *wandb.GetTolerations(infraSpec.ManagedInfraSpec),
						SecurityContext: kafkaPodSecurityContext(),
					},
				},
			},
			EntityOperator: &v1.EntityOperatorSpec{
				TopicOperator: &v1.EntityTopicOperatorSpec{WatchedNamespace: nsnBuilder.Namespace()},
				UserOperator:  &v1.EntityUserOperatorSpec{WatchedNamespace: nsnBuilder.Namespace()},
				Template: &v1.EntityOperatorTemplate{
					Pod: &v1.PodTemplate{
						Metadata: &v1.MetadataTemplate{
							Labels: BuildWandbKafkaLabels(wandb),
						},
						Affinity:        wandb.GetAffinity(infraSpec.ManagedInfraSpec),
						Tolerations:     *wandb.GetTolerations(infraSpec.ManagedInfraSpec),
						SecurityContext: kafkaPodSecurityContext(),
					},
					TopicOperatorContainer: &v1.ContainerTemplate{
						SecurityContext: kafkaContainerSecurityContext(),
					},
					UserOperatorContainer: &v1.ContainerTemplate{
						SecurityContext: kafkaContainerSecurityContext(),
					},
					TlsSidecarContainer: &v1.ContainerTemplate{
						SecurityContext: kafkaContainerSecurityContext(),
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

	log.Debug("Kafka is enabled, providing vendor spec")
	return kafka, nil
}

// ToKafkaNodePoolVendorSpec converts a KafkaSpec to a KafkaNodePool CR.
// This function creates the node pool that contains the actual replica count
// and runs in KRaft mode with broker and controller roles.
func ToKafkaNodePoolVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
	mfst manifest.Manifest,
) (*v1.KafkaNodePool, error) {
	_, log := logx.WithSlog(ctx, logx.Kafka)

	infraSpec := wandb.Spec.Kafka.ManagedKafka
	if infraSpec == nil {
		return nil, nil
	}

	retentionPolicy := wandb.GetRetentionPolicy(infraSpec.ManagedInfraSpec)
	nsnBuilder := CreateNsNameBuilder(types.NamespacedName{
		Namespace: infraSpec.Namespace, Name: infraSpec.Name,
	})

	onDeletePurge := retentionPolicy.OnDelete == apiv2.PurgeOnDelete

	nodePoolLabels := BuildWandbKafkaLabels(wandb)
	nodePoolLabels["strimzi.io/cluster"] = nsnBuilder.KafkaName()
	nodePool := &v1.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.NodePoolName(),
			Namespace: nsnBuilder.Namespace(),
			Labels:    nodePoolLabels,
		},
		Spec: v1.KafkaNodePoolSpec{
			Replicas: infraSpec.Replicas,
			Roles:    []string{RoleBroker, RoleController},
			Storage: v1.KafkaStorage{
				Type: "jbod",
				Volumes: []v1.StorageVolume{
					{
						ID:          0,
						Type:        StorageType,
						Size:        infraSpec.StorageSize,
						DeleteClaim: onDeletePurge,
					},
				},
			},
			Template: &v1.KafkaNodePoolTemplate{
				Pod: &v1.PodTemplate{
					Metadata: &v1.MetadataTemplate{
						Labels: BuildWandbKafkaLabels(wandb),
					},
					SecurityContext: kafkaPodSecurityContext(),
				},
				InitContainer: &v1.ContainerTemplate{
					SecurityContext: kafkaContainerSecurityContext(),
				},
				KafkaContainer: &v1.ContainerTemplate{
					SecurityContext: kafkaContainerSecurityContext(),
				},
				PersistentVolumeClaim: &v1.ResourceTemplate{
					Metadata: &v1.MetadataTemplate{
						Labels: BuildWandbKafkaLabels(wandb),
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

func BuildWandbKafkaLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, KafkaModuleName)
}

func ToKafkaOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) common.OnDeleteRule {
	return common.ToOnDeleteRule(wandb, retentionPolicy, KafkaModuleName)
}
