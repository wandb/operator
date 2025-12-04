package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	kafkav1beta2 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredKafka *kafkav1beta2.Kafka,
	desiredNodePool *kafkav1beta2.KafkaNodePool,
) error {
	var err error

	if err = writeKafkaState(ctx, client, specNamespacedName, desiredKafka); err != nil {
		return err
	}
	if err = writeNodePoolState(ctx, client, specNamespacedName, desiredNodePool); err != nil {
		return err
	}

	return nil
}

func writeKafkaState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desired *kafkav1beta2.Kafka,
) error {
	var err error
	var actual = &kafkav1beta2.Kafka{}

	if err = common.GetResource(
		ctx, client, KafkaNamespacedName(specNamespacedName), KafkaResourceType, actual,
	); err != nil {
		return err
	}
	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}

func writeNodePoolState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desired *kafkav1beta2.KafkaNodePool,
) error {
	var err error
	var actual = &kafkav1beta2.KafkaNodePool{}

	if err = common.GetResource(
		ctx, client, NodePoolNamespacedName(specNamespacedName), NodePoolResourceType, actual,
	); err != nil {
		return err
	}
	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}
