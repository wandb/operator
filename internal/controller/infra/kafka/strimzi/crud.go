package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	kafkav1beta2 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CrudKafkaResource(
	ctx context.Context,
	client client.Client,
	namespacedName types.NamespacedName,
	desired *kafkav1beta2.Kafka,
) error {
	var err error
	var actual = &kafkav1beta2.Kafka{}

	if err = common.GetResource(
		ctx, client, namespacedName, KafkaResourceType, actual,
	); err != nil {
		return err
	}
	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}

func CrudNodePoolResource(
	ctx context.Context,
	client client.Client,
	namespacedName types.NamespacedName,
	desired *kafkav1beta2.KafkaNodePool,
) error {
	var err error
	var actual = &kafkav1beta2.KafkaNodePool{}

	if err = common.GetResource(
		ctx, client, namespacedName, NodePoolResourceType, actual,
	); err != nil {
		return err
	}
	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}
