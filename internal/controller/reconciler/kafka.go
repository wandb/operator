package reconciler

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	externalkafka "github.com/wandb/operator/internal/controller/infra/external/kafka"
	"github.com/wandb/operator/internal/controller/infra/managed/kafka/strimzi"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func kafkaWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaWriteState(ctx, client, wandb)
	}
	if wandb.Spec.Kafka.ExternalKafka != nil {
		return externalKafkaWriteState(ctx, client, wandb)
	}
	return nil
}

func kafkaReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.KafkaConnection) {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaReadState(ctx, client, wandb, newConditions)
	}
	if wandb.Spec.Kafka.ExternalKafka != nil {
		return externalKafkaReadState(ctx, client, wandb, newConditions)
	}
	return newConditions, nil
}

func kafkaInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.KafkaConnection,
) (ctrl.Result, error) {
	if wandb.Spec.Kafka.ManagedKafka != nil {
		return managedKafkaInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
	}
	if wandb.Spec.Kafka.ExternalKafka != nil {
		return externalKafkaInferStatus(ctx, client, wandb, newConditions, newInfraConn)
	}
	return ctrl.Result{}, nil
}

func kafkaPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	if spec := wandb.Spec.Kafka.ManagedKafka; spec != nil {
		specNamespacedName := managedKafkaSpecNamespacedName(spec)
		onDeleteRule := strimzi.ToKafkaOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return strimzi.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if wandb.Spec.Kafka.ExternalKafka != nil {
		return externalkafka.DeleteConnectionSecret(ctx, client, wandb)
	}
	return nil
}

func kafkaDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.Kafka.ManagedKafka
	if spec == nil {
		return nil
	}
	specNamespacedName := managedKafkaSpecNamespacedName(spec)
	return strimzi.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedKafkaWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.Kafka.ManagedKafka

	log := ctrl.LoggerFrom(ctx)
	var desiredKafka *v1.Kafka
	desiredKafka, err := strimzi.ToKafkaVendorSpec(
		ctx,
		wandb,
		client.Scheme(),
	)
	if err != nil {
		log.Error(err, "failed to translate kafka spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	var desiredNodePool *v1.KafkaNodePool
	desiredNodePool, err = strimzi.ToKafkaNodePoolVendorSpec(
		ctx,
		wandb,
		client.Scheme(),
	)
	if err != nil {
		log.Error(err, "failed to translate kafka node pool spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	specNamespacedName := managedKafkaSpecNamespacedName(spec)

	if conditions := strimzi.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
		return conditions
	}

	results := make([]metav1.Condition, 0)
	results = append(results, strimzi.WriteState(ctx, client, specNamespacedName, desiredKafka, desiredNodePool)...)

	return results
}

func managedKafkaReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.KafkaConnection) {
	spec := wandb.Spec.Kafka.ManagedKafka

	specNamespacedName := managedKafkaSpecNamespacedName(spec)
	onDeleteRule := strimzi.ToKafkaOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	readConditions, newInfraConn := strimzi.ReadState(ctx, client, specNamespacedName, wandb, onDeleteRule)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func managedKafkaInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.KafkaConnection,
) (ctrl.Result, error) {
	oldConditions := wandb.Status.KafkaStatus.Conditions
	oldInfraConn := wandb.Status.KafkaStatus.Connection

	enabled := true
	updatedStatus, events, ctrlResult := strimzi.ComputeStatus(
		ctx,
		enabled,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.KafkaStatus = updatedStatus
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalKafkaWriteState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) []metav1.Condition {
	return externalkafka.WriteState(ctx, c, wandb)
}

func externalKafkaReadState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition) ([]metav1.Condition, *apiv2.KafkaConnection) {
	return externalkafka.ReadState(ctx, c, wandb, newConditions)
}

func externalKafkaInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition, newInfraConn *apiv2.KafkaConnection) (ctrl.Result, error) {
	oldInfraConn := wandb.Status.KafkaStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(wandb.Status.KafkaStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.KafkaStatus = apiv2.KafkaInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, c.Status().Update(ctx, wandb)
}

// helpers

func managedKafkaSpecNamespacedName(spec *apiv2.ManagedKafkaSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}

func createKafkaTopics(ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases, manifest manifest.Manifest) (ctrl.Result, error) {
	// Create Strimzi KafkaTopic resources for enabled topics
	if wandb.Spec.Kafka.ManagedKafka != nil {
		kafkaSpec := wandb.Spec.Kafka.ManagedKafka
		for _, topic := range manifest.Kafka.Topics {
			if len(topic.Features) > 0 && !manifest.FeaturesEnabled(topic.Features) {
				continue
			}

			kafkaNS := kafkaSpec.Namespace
			if kafkaNS == "" {
				kafkaNS = wandb.Namespace
			}
			clusterName := kafkaSpec.Name
			if clusterName == "" {
				clusterName = wandb.Name
			}

			// Use the logical topic name as the resource name
			resName := topic.Name
			if resName == "" {
				// If not set, fallback to topic entry name
				resName = topic.Topic
			}
			if resName == "" {
				// Nothing actionable without a name
				continue
			}
			labels := map[string]string{
				"strimzi.io/cluster":           clusterName,
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/part-of":    "wandb",
				"app.kubernetes.io/instance":   wandb.Name,
			}

			// Prepare spec
			partitions := int32(1)
			if topic.PartitionCount > 0 {
				partitions = int32(topic.PartitionCount)
			}
			replicas := int32(1)
			if kafkaSpec.Config.ReplicationConfig.DefaultReplicationFactor > 0 {
				replicas = int32(kafkaSpec.Config.ReplicationConfig.DefaultReplicationFactor)
			}

			// Build typed KafkaTopic object
			kt := &v1.KafkaTopic{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resName,
					Namespace: kafkaNS,
					Labels:    labels,
				},
				Spec: v1.KafkaTopicSpec{
					TopicName:  topic.Topic,
					Partitions: partitions,
					Replicas:   replicas,
				},
			}

			// Create or Update
			existing := &v1.KafkaTopic{}
			getErr := client.Get(ctx, types.NamespacedName{Name: kt.Name, Namespace: kafkaNS}, existing)
			if getErr != nil {
				if errors.IsNotFound(getErr) {
					// Set ownerRef only if same namespace
					if kafkaNS == wandb.Namespace {
						_ = controllerutil.SetOwnerReference(wandb, kt, client.Scheme())
					}
					if err := client.Create(ctx, kt); err != nil {
						return ctrl.Result{}, err
					}
				} else {
					return ctrl.Result{}, getErr
				}
			} else {
				// Update managed spec fields and preserve other fields
				existing.Spec.TopicName = topic.Topic
				existing.Spec.Partitions = partitions
				existing.Spec.Replicas = replicas
				// Preserve/ensure labels
				exLabels := existing.GetLabels()
				if exLabels == nil {
					exLabels = map[string]string{}
				}
				for k, v := range labels {
					exLabels[k] = v
				}
				existing.SetLabels(exLabels)
				if err := client.Update(ctx, existing); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}
