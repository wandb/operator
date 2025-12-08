package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	kafkav1beta2 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func kafkaWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desiredKafka *kafkav1beta2.Kafka
	var desiredNodePool *kafkav1beta2.KafkaNodePool
	var specNamespacedName = kafkaSpecNamespacedName(wandb.Spec.Kafka)

	if desiredKafka, err = translatorv2.ToKafkaVendorSpec(ctx, wandb.Spec.Kafka, wandb, client.Scheme()); err != nil {
		return err
	}
	if desiredNodePool, err = translatorv2.ToKafkaNodePoolVendorSpec(ctx, wandb.Spec.Kafka, wandb, client.Scheme()); err != nil {
		return err
	}
	if err = strimzi.WriteState(ctx, client, specNamespacedName, desiredKafka, desiredNodePool); err != nil {
		return err
	}

	return err
}

func kafkaReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var conditions []translator.KafkaCondition
	var specNamespacedName = kafkaSpecNamespacedName(wandb.Spec.Kafka)

	if conditions, err = strimzi.ReadState(ctx, client, specNamespacedName, wandb); err != nil {
		return err
	}
	wandb.Status.KafkaStatus = translatorv2.ExtractKafkaStatus(ctx, conditions)
	if err = client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "failed to update status")
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
