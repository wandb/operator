package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/redis/opstree"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func redisWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	var specNamespacedName = redisSpecNamespacedName(wandb.Spec.Redis)

	standaloneDesired, err := translatorv2.ToRedisStandaloneVendorSpec(ctx, wandb, client.Scheme())
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	sentinelDesired, err := translatorv2.ToRedisSentinelVendorSpec(ctx, wandb, client.Scheme())
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	replicationDesired, err := translatorv2.ToRedisReplicationVendorSpec(ctx, wandb, client.Scheme())
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	results := opstree.WriteState(ctx, client, specNamespacedName, standaloneDesired, sentinelDesired, replicationDesired)
	return results
}

func redisReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.InfraConnection) {
	specNamespacedName := redisSpecNamespacedName(wandb.Spec.Redis)
	readConditions, newInfraConn := opstree.ReadState(ctx, client, specNamespacedName, wandb)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func redisInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.InfraConnection,
) (ctrl.Result, error) {
	oldConditions := wandb.Status.RedisStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorInfraConnection(wandb.Status.RedisStatus.Connection)

	updatedStatus, events, ctrlResult := opstree.ComputeStatus(
		ctx,
		wandb.Spec.Redis.Enabled,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.RedisStatus = translatorv2.ToWbInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

func redisSpecNamespacedName(redis apiv2.WBRedisSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: redis.Namespace,
		Name:      redis.Name,
	}
}
