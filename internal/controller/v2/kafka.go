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
	log := ctrl.LoggerFrom(ctx)
	var desiredKafka *v1.Kafka
	desiredKafka, err := translatorv2.ToKafkaVendorSpec(
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
	desiredNodePool, err = translatorv2.ToKafkaNodePoolVendorSpec(
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

	specNamespacedName := kafkaSpecNamespacedName(wandb.Spec.Kafka)

	if conditions := strimzi.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), wandb.Spec.Kafka.Replicas); conditions != nil {
		return conditions
	}

	results := make([]metav1.Condition, 0)
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
	onDeleteRule := translatorv2.ToKafkaOnDeleteRule(wandb, wandb.GetRetentionPolicy(wandb.Spec.Kafka.ManagedInfraSpec))
	readConditions, newInfraConn := strimzi.ReadState(ctx, client, specNamespacedName, wandb, onDeleteRule)
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

func kafkaPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	specNamespacedName := kafkaSpecNamespacedName(wandb.Spec.Kafka)
	onDeleteRule := translatorv2.ToKafkaOnDeleteRule(wandb, wandb.GetRetentionPolicy(wandb.Spec.Kafka.ManagedInfraSpec))
	return strimzi.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
}

func kafkaDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	specNamespacedName := kafkaSpecNamespacedName(wandb.Spec.Kafka)
	return strimzi.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

func kafkaSpecNamespacedName(kafka apiv2.KafkaSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: kafka.Namespace,
		Name:      kafka.Name,
	}

}
