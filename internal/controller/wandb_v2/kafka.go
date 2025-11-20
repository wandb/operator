package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	kafkav1beta2 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
)

func (r *WeightsAndBiasesV2Reconciler) kafkaResourceReconcile(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desiredKafka *kafkav1beta2.Kafka
	var desiredNodePool *kafkav1beta2.KafkaNodePool

	if desiredKafka, err = translatorv2.ToKafkaVendorSpec(ctx, wandb.Spec.Kafka, wandb, r.Scheme); err != nil {
		return err
	}
	if err = strimzi.CrudKafkaResource(ctx, r.Client, translatorv2.KafkaNamespacedName(wandb.Spec.Kafka), desiredKafka); err != nil {
		return err
	}

	if desiredNodePool, err = translatorv2.ToKafkaNodePoolVendorSpec(ctx, wandb.Spec.Kafka, wandb, r.Scheme); err != nil {
		return err
	}
	if err = strimzi.CrudNodePoolResource(ctx, r.Client, translatorv2.KafkaNodePoolNamespacedName(wandb.Spec.Kafka), desiredNodePool); err != nil {
		return err
	}

	//wandb.Status.KafkaStatus = translatorv2.ExtractKafkaStatus(ctx, results)
	//if err = r.Status().Update(ctx, wandb); err != nil {
	//	results.AddErrors(err)
	//}

	return err
}

func (r *WeightsAndBiasesV2Reconciler) kafkaStatusUpdate(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var conditions []common.InfraStatusDetail

	if conditions, err = strimzi.GetConditions(
		ctx,
		r,
		translatorv2.KafkaNamespacedName(wandb.Spec.Kafka),
		translatorv2.KafkaNodePoolNamespacedName(wandb.Spec.Kafka),
	); err != nil {
		return err
	}
	wandb.Status.KafkaStatus = translatorv2.ExtractKafkaStatus(ctx, conditions)

}
