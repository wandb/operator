package wandb_v2

import (
	"context"
	"errors"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *WeightsAndBiasesV2Reconciler) handleKafkaHA(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) ctrlqueue.CtrlState {
	var err error
	var desiredKafka wandbKafkaWrapper
	var actualKafka wandbKafkaWrapper
	var reconciliation wandbKafkaDoReconcile
	log := ctrl.LoggerFrom(ctx)
	namespacedName := kafkaNamespacedName(wandb)

	if !wandb.Spec.Kafka.Enabled {
		log.Info("Kafka not enabled, skipping")
		return ctrlqueue.CtrlContinue()
	}

	log.Info("Handling Kafka HA")

	if actualKafka, err = getActualKafka(ctx, r, namespacedName); err != nil {
		log.Error(err, "Failed to get actual Kafka HA resources")
		return ctrlqueue.CtrlError(err)
	}

	if ctrlState := actualKafka.maybeHandleDeletion(ctx, wandb, actualKafka, r); ctrlState.ShouldExit(ctrlqueue.PackageScope) {
		return ctrlState
	}

	if desiredKafka, err = getDesiredKafkaHA(ctx, wandb, namespacedName, actualKafka); err != nil {
		log.Error(err, "Failed to get desired Kafka HA configuration")
		return ctrlqueue.CtrlError(err)
	}

	if reconciliation, err = computeKafkaReconcileDrift(ctx, wandb, desiredKafka, actualKafka, r); err != nil {
		log.Error(err, "Failed to compute Kafka HA reconcile drift")
		return ctrlqueue.CtrlError(err)
	}

	if reconciliation != nil {
		return reconciliation.Execute(ctx, r)
	}

	return ctrlqueue.CtrlContinue()
}

func getDesiredKafkaHA(
	_ context.Context, wandb *apiv2.WeightsAndBiases, namespacedName types.NamespacedName, actual wandbKafkaWrapper,
) (
	wandbKafkaWrapper, error,
) {
	result := wandbKafkaWrapper{
		kafkaInstalled:    false,
		kafkaObj:          nil,
		nodePoolObj:       nil,
		secretInstalled:   false,
		secret:            nil,
		nodePoolInstalled: false,
	}

	if !wandb.Spec.Kafka.Enabled {
		return result, nil
	}

	result.kafkaInstalled = true
	result.nodePoolInstalled = true

	storageSize := wandb.Spec.Kafka.StorageSize
	if storageSize == "" {
		storageSize = "10Gi"
	}

	replicas := int32(3)

	kafka := &v1beta3.Kafka{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"app": "wandb-kafka",
			},
			Annotations: map[string]string{
				"strimzi.io/node-pools": "enabled",
			},
		},
		Spec: v1beta3.KafkaSpec{
			Kafka: v1beta3.KafkaClusterSpec{
				Version:         "4.1.0",
				MetadataVersion: "4.1-IV0",
				Replicas:        0,
				Listeners: []v1beta3.GenericKafkaListener{
					{
						Name: "plain",
						Port: 9092,
						Type: "internal",
						Tls:  false,
					},
					{
						Name: "tls",
						Port: 9093,
						Type: "internal",
						Tls:  true,
					},
				},
				Config: map[string]string{
					"offsets.topic.replication.factor":         "3",
					"transaction.state.log.replication.factor": "3",
					"transaction.state.log.min.isr":            "2",
					"default.replication.factor":               "3",
					"min.insync.replicas":                      "2",
				},
			},
		},
	}

	nodePool := &v1beta3.KafkaNodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-kafka-pool",
			Namespace: namespacedName.Namespace,
			Labels: map[string]string{
				"strimzi.io/cluster": namespacedName.Name,
			},
		},
		Spec: v1beta3.KafkaNodePoolSpec{
			Replicas: replicas,
			Roles:    []string{"broker", "controller"},
			Storage: v1beta3.KafkaStorage{
				Type: "jbod",
				Volumes: []v1beta3.StorageVolume{
					{
						ID:          0,
						Type:        "persistent-claim",
						Size:        storageSize,
						DeleteClaim: true,
					},
				},
			},
		},
	}

	wandbBackupSpec := wandb.Spec.Kafka.Backup
	if wandbBackupSpec.Enabled {
		if wandbBackupSpec.StorageType != apiv2.WBBackupStorageTypeFilesystem {
			return result, errors.New("only filesystem backup storage type is supported for Kafka")
		}
	}

	result.kafkaObj = kafka
	result.nodePoolObj = nodePool

	if actual.IsReady() && actual.kafkaObj != nil && len(actual.kafkaObj.Status.Listeners) > 0 {
		var bootstrapServers string
		for _, listener := range actual.kafkaObj.Status.Listeners {
			if listener.Name == "plain" {
				bootstrapServers = listener.BootstrapServers
				break
			}
		}

		if bootstrapServers != "" {
			namespace := namespacedName.Namespace
			connectionSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wandb-kafka-connection",
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"KAFKA_BOOTSTRAP_SERVERS": bootstrapServers,
				},
			}

			result.secret = connectionSecret
			result.secretInstalled = true
		}
	}

	return result, nil
}
