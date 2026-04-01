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
	if wandb.Spec.Redis.ManagedRedis != nil {
		return managedRedisWriteState(ctx, client, wandb)
	}
	if wandb.Spec.Redis.ExternalRedis != nil {
		return externalRedisWriteState(ctx, client, wandb)
	}
	return nil
}

func redisReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.RedisConnection) {
	if wandb.Spec.Redis.ManagedRedis != nil {
		return managedRedisReadState(ctx, client, wandb, newConditions)
	}
	if wandb.Spec.Redis.ExternalRedis != nil {
		return externalRedisReadState(ctx, client, wandb, newConditions)
	}
	return newConditions, nil
}

func redisInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.RedisConnection,
) (ctrl.Result, error) {
	if wandb.Spec.Redis.ManagedRedis != nil {
		return managedRedisInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
	}
	// TODO: external redis infer status
	return ctrl.Result{}, nil
}

func redisPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.Redis.ManagedRedis
	if spec == nil {
		return nil
	}
	specNamespacedName := managedRedisSpecNamespacedName(spec)
	onDeleteRule := translatorv2.ToRedisOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	return opstree.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
}

func redisDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.Redis.ManagedRedis
	if spec == nil {
		return nil
	}
	specNamespacedName := managedRedisSpecNamespacedName(spec)
	return opstree.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedRedisWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.Redis.ManagedRedis

	log := ctrl.LoggerFrom(ctx)
	var specNamespacedName = managedRedisSpecNamespacedName(spec)

	standaloneDesired, err := translatorv2.ToRedisStandaloneVendorSpec(ctx, wandb, client.Scheme())
	if err != nil {
		log.Error(err, "failed to translate redis standalone spec")
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
		log.Error(err, "failed to translate redis sentinel spec")
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
		log.Error(err, "failed to translate redis replication spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	if conditions := opstree.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID()); conditions != nil {
		return conditions
	}

	results := opstree.WriteState(ctx, client, specNamespacedName, standaloneDesired, sentinelDesired, replicationDesired, translatorv2.BuildWandbRedisLabels(wandb))
	return results
}

func managedRedisReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.RedisConnection) {
	spec := wandb.Spec.Redis.ManagedRedis

	specNamespacedName := managedRedisSpecNamespacedName(spec)
	onDeleteRule := translatorv2.ToRedisOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	readConditions, newInfraConn := opstree.ReadState(ctx, client, specNamespacedName, wandb, onDeleteRule)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func managedRedisInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.RedisConnection,
) (ctrl.Result, error) {
	enabled := true
	oldConditions := wandb.Status.RedisStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorRedisConnection(wandb.Status.RedisStatus.Connection)

	updatedStatus, events, ctrlResult := opstree.ComputeStatus(
		ctx,
		enabled,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.RedisStatus = translatorv2.ToWbRedisInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalRedisWriteState(
	_ context.Context,
	_ client.Client,
	_ *apiv2.WeightsAndBiases,
) []metav1.Condition {
	// TODO: implement external redis write state
	return nil
}

func externalRedisReadState(
	_ context.Context,
	_ client.Client,
	_ *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.RedisConnection) {
	// TODO: implement external redis read state
	return newConditions, nil
}

// helpers

func managedRedisSpecNamespacedName(spec *apiv2.ManagedRedisSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
