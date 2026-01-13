package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredKafka *strimziv1.Kafka,
	desiredNodePool *strimziv1.KafkaNodePool,
) []metav1.Condition {
	ctx, _ = logx.IntoContext(ctx, logx.Kafka)
	results := make([]metav1.Condition, 0)

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	results = append(results, writeKafkaState(ctx, client, nsnBuilder, desiredKafka)...)
	results = append(results, writeNodePoolState(ctx, client, nsnBuilder, desiredNodePool)...)

	return results
}

func writeKafkaState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.Kafka,
) []metav1.Condition {
	var actual = &strimziv1.Kafka{}

	found, err := common.GetResource(
		ctx, client, nsnBuilder.KafkaNsName(), KafkaResourceType, actual,
	)
	// if we error on getting the Kafka resource:
	// * Reconciling failed
	// * we don't know if it is installed
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   KafkaCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		actual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, client, desired, actual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	// regardless whether the Kafka CRUD was successful or not, we can infer the Kafka installation status
	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   KafkaCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   KafkaCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   KafkaCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   KafkaCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}

func writeNodePoolState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	desired *strimziv1.KafkaNodePool,
) []metav1.Condition {
	var actual = &strimziv1.KafkaNodePool{}
	found, err := common.GetResource(
		ctx, client, nsnBuilder.NodePoolNsName(), NodePoolResourceType, actual,
	)
	// if we error on getting the NodePool resource:
	// * Reconciling failed
	// * we don't know if it is installed
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   NodePoolCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		actual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, client, desired, actual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	} else {
		// if we successfully created the resource, try to restore existing connection info from a previous installation
		if action == common.CreateAction {
			if err = restoreKafkaConnInfo(ctx, client, nsnBuilder, desired, actual); err != nil {
				result = append(result, metav1.Condition{
					Type:   common.ReconciledType,
					Status: metav1.ConditionFalse,
					Reason: common.ApiErrorReason,
				})
			}
		}
	}

	// regardless whether the NodePool CRUD was successful or not, we can infer the NodePool installation status
	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   NodePoolCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   NodePoolCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   NodePoolCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   NodePoolCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}
