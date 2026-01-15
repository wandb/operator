package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func kafkaWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	if wandb.Spec.Kafka.Affinity == nil {
		wandb.Spec.Kafka.Affinity = wandb.Spec.Affinity
	}
	if wandb.Spec.Kafka.Tolerations == nil {
		wandb.Spec.Kafka.Tolerations = wandb.Spec.Tolerations
	}

	var desiredKafka *v1.Kafka
	desiredKafka, err := translatorv2.ToKafkaVendorSpec(ctx, wandb.Spec.Kafka, wandb, client.Scheme())
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	var desiredNodePool *v1.KafkaNodePool
	desiredNodePool, err = translatorv2.ToKafkaNodePoolVendorSpec(ctx, wandb.Spec.Kafka, wandb, client.Scheme())
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	results := make([]metav1.Condition, 0)

	specNamespacedName := kafkaSpecNamespacedName(wandb.Spec.Kafka)
	results = append(results, strimzi.WriteState(ctx, client, specNamespacedName, desiredKafka, desiredNodePool)...)

	return results
}

func kafkaReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.InfraConnection) {
	specNamespacedName := kafkaSpecNamespacedName(wandb.Spec.Kafka)
	readConditions, newInfraConn := strimzi.ReadState(ctx, client, specNamespacedName, wandb)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func kafkaInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.InfraConnection,
) (ctrl.Result, error) {
	oldConditions := wandb.Status.KafkaStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorInfraConnection(wandb.Status.KafkaStatus.Connection)

	updatedStatus, events, ctrlResult := strimzi.ComputeStatus(
		ctx,
		wandb.Spec.Kafka.Enabled,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.KafkaStatus = translatorv2.ToWbInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

func kafkaRetentionPolicy(ctx context.Context, wandb *apiv2.WeightsAndBiases) apiv2.WBRetentionPolicy {
	if wandb.Spec.Kafka.RetentionPolicy == nil {
		return wandb.Spec.RetentionPolicy
	}
	return *wandb.Spec.Kafka.RetentionPolicy
}

func kafkaPreserveFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	var specNamespacedName = kafkaSpecNamespacedName(wandb.Spec.Kafka)

	if err := strimzi.PreserveFinalizer(ctx, client, specNamespacedName, wandb); err != nil {
		return err
	}
	return nil
}

func kafkaSpecNamespacedName(kafka apiv2.WBKafkaSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: kafka.Namespace,
		Name:      kafka.Name,
	}

}
