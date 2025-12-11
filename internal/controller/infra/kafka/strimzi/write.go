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

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = writeKafkaState(ctx, client, nsNameBldr, desiredKafka); err != nil {
		return err
	}
	if err = writeNodePoolState(ctx, client, nsNameBldr, desiredNodePool); err != nil {
		return err
	}

	return nil
}

func writeKafkaState(
	ctx context.Context,
	client client.Client,
	nsNameBldr *NsNameBuilder,
	desired *kafkav1beta2.Kafka,
) error {
	var err error
	var found bool
	var actual = &kafkav1beta2.Kafka{}

	if found, err = common.GetResource(
		ctx, client, nsNameBldr.KafkaNsName(), KafkaResourceType, actual,
	); err != nil {
		return err
	}
	if !found {
		actual = nil
	}

	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}

func writeNodePoolState(
	ctx context.Context,
	client client.Client,
	nsNameBldr *NsNameBuilder,
	desired *kafkav1beta2.KafkaNodePool,
) error {
	var err error
	var found bool
	var actual = &kafkav1beta2.KafkaNodePool{}

	if found, err = common.GetResource(
		ctx, client, nsNameBldr.NodePoolNsName(), NodePoolResourceType, actual,
	); err != nil {
		return err
	}
	if !found {
		actual = nil
	}

	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}
