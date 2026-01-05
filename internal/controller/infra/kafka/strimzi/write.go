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
	wandbOwner client.Object,
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
	if err = writeNodePoolState(ctx, client, nsnBuilder, wandbOwner, desiredNodePool, retentionPolicy); err != nil {
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
	wandbOwner client.Object,
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

	action, err := common.CrudResource(ctx, client, desired, actual)
	if err != nil {
		return err
	}

	switch action {
	case common.CreateAction:
		if err = restoreKafkaBackupConnInfo(ctx, client, wandbOwner, nsnBuilder, desired, actual); err != nil {
			return err
		}
		if err = restoreKafkaNodePoolClusterId(ctx, client, nsnBuilder, desired); err != nil {
			return err
		}
		break
	case common.DeleteAction:
		switch retentionPolicy {
		case common.PurgePolicy:
			if err = deleteKafkaConnInfo(ctx, client, nsnBuilder); err != nil {
				return err
			}
		case common.RetainPolicy:
			if err = backupKafkaConnInfo(ctx, client, nsnBuilder.SpecNsName()); err != nil {
				return err
			}
		}
	}

	return nil
}
