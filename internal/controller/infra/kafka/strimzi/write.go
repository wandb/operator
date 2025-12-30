package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredKafka *strimziv1.Kafka,
	desiredNodePool *strimziv1.KafkaNodePool,
	retentionPolicy common.RetentionPolicy,
) error {
	var err error

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if err = writeKafkaState(ctx, client, nsnBuilder, desiredKafka, retentionPolicy); err != nil {
		return err
	}
	if err = writeNodePoolState(ctx, client, nsnBuilder, desiredNodePool, retentionPolicy); err != nil {
		return err
	}

	return nil
}

func writeKafkaState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.Kafka,
	retentionPolicy common.RetentionPolicy,
) error {
	var err error
	var found bool
	var actual = &strimziv1.Kafka{}

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.KafkaNsName(), KafkaResourceType, actual,
	); err != nil {
		return err
	}
	if !found {
		actual = nil
	}

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}

func writeNodePoolState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.KafkaNodePool,
	retentionPolicy common.RetentionPolicy,
) error {
	var err error
	var found bool
	var actual = &strimziv1.KafkaNodePool{}

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.NodePoolNsName(), NodePoolResourceType, actual,
	); err != nil {
		return err
	}
	if !found {
		actual = nil
	}

	shouldRestore := desired != nil && actual == nil

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	if shouldRestore {
		if err = restoreKafkaConnInfo(ctx, client, nsnBuilder, desired, actual); err != nil {
			return err
		}
	}

	return nil
}
