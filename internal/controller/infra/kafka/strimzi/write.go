package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
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
) []translator.ProtoCondition {
	results := make([]translator.ProtoCondition, 0)

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	results = append(results, writeKafkaState(ctx, client, nsnBuilder, desiredKafka))
	results = append(results, writeNodePoolState(ctx, client, nsnBuilder, desiredNodePool))

	// unless not ready, consider it installed
	if !translator.IsNotReady(results) {
		results = append(results, translator.InstalledProtoBuilder(KafkaDeploymentReason).Build())
	}

	return results
}

func writeKafkaState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.Kafka,
) translator.ProtoCondition {
	var err error
	var found bool
	var actual = &strimziv1.Kafka{}

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.KafkaNsName(), KafkaResourceType, actual,
	); err != nil {
		return translator.ErrorProtoBuilder(
			translator.MachineryErrorType, KafkaInstanceReason,
		).Message(err.Error()).Build()
	}
	if !found {
		actual = nil
	}

	action, err := common.CrudResource(ctx, client, desired, actual)
	if err != nil {
		return translator.ErrorProtoBuilder(
			translator.MachineryErrorType, KafkaInstanceReason,
		).Message(err.Error()).Build()
	}

	switch action {
	case common.CreateAction:
		return translator.PendingProtoBuilder(translator.ResourceCreatePending, KafkaInstanceReason).Build()
	case common.DeleteAction:
		return translator.PendingProtoBuilder(translator.ResourceCreatePending, KafkaInstanceReason).Build()
	case common.NoAction:
		return translator.NotInstalledProtoBuilder(KafkaInstanceReason).Build()
	}

	return translator.InstalledProtoBuilder(KafkaInstanceReason).Build()
}

func writeNodePoolState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.KafkaNodePool,
) translator.ProtoCondition {
	var err error
	var found bool
	var actual = &strimziv1.KafkaNodePool{}

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.NodePoolNsName(), NodePoolResourceType, actual,
	); err != nil {
		return translator.ErrorProtoBuilder(
			translator.MachineryErrorType, KafkaNodePoolReason,
		).Message(err.Error()).Build()
	}
	if !found {
		actual = nil
	}

	action, err := common.CrudResource(ctx, client, desired, actual)
	if err != nil {
		return translator.ErrorProtoBuilder(
			translator.MachineryErrorType, KafkaNodePoolReason,
		).Message(err.Error()).Build()
	}

	if action == common.CreateAction {
		if err = restoreKafkaConnInfo(ctx, client, nsnBuilder, desired, actual); err != nil {
			return translator.ErrorProtoBuilder(
				translator.MachineryErrorType, KafkaConnectionSecretReason,
			).Message(err.Error()).Build()
		}
	}

	switch action {
	case common.CreateAction:
		return translator.PendingProtoBuilder(translator.ResourceCreatePending, KafkaNodePoolReason).Build()
	case common.DeleteAction:
		return translator.PendingProtoBuilder(translator.ResourceCreatePending, KafkaNodePoolReason).Build()
	case common.NoAction:
		return translator.NotInstalledProtoBuilder(KafkaNodePoolReason).Build()
	}

	return translator.InstalledProtoBuilder(KafkaNodePoolReason).Build()
}
