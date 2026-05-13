package reconciler

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	externalredis "github.com/wandb/operator/internal/controller/infra/external/redis"
	"github.com/wandb/operator/internal/controller/infra/managed/redis/opstree"
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
) ([]metav1.Condition, *apiv2.RedisConnection) {
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
	newInfraConn *apiv2.RedisConnection,
) (ctrl.Result, error) {
	if wandb.Spec.Redis.ManagedRedis != nil {
		return managedRedisInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
	}
	if wandb.Spec.Redis.ExternalRedis != nil {
		return externalRedisInferStatus(ctx, client, wandb, newConditions, newInfraConn)
	}
	return ctrl.Result{}, nil
}

func redisPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	if spec := wandb.Spec.Redis.ManagedRedis; spec != nil {
		specNamespacedName := managedRedisSpecNamespacedName(spec)
		onDeleteRule := opstree.ToRedisOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return opstree.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if wandb.Spec.Redis.ExternalRedis != nil {
		return externalredis.DeleteConnectionSecret(ctx, client, wandb)
	}
	return nil
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

	standaloneDesired, err := opstree.ToRedisStandaloneVendorSpec(ctx, wandb, client.Scheme())
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

	sentinelDesired, err := opstree.ToRedisSentinelVendorSpec(ctx, wandb, client.Scheme())
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

	replicationDesired, err := opstree.ToRedisReplicationVendorSpec(ctx, wandb, client.Scheme())
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

	results := opstree.WriteState(ctx, client, specNamespacedName, standaloneDesired, sentinelDesired, replicationDesired, opstree.BuildWandbRedisLabels(wandb))
	return results
}

func managedRedisReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.RedisConnection) {
	spec := wandb.Spec.Redis.ManagedRedis

	specNamespacedName := managedRedisSpecNamespacedName(spec)
	onDeleteRule := opstree.ToRedisOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
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
	newInfraConn *apiv2.RedisConnection,
) (ctrl.Result, error) {
	enabled := true
	oldConditions := wandb.Status.RedisStatus.Conditions
	oldInfraConn := wandb.Status.RedisStatus.Connection

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
	wandb.Status.RedisStatus = updatedStatus
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalRedisWriteState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) []metav1.Condition {
	return externalredis.WriteState(ctx, c, wandb)
}

func externalRedisReadState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition) ([]metav1.Condition, *apiv2.RedisConnection) {
	return externalredis.ReadState(ctx, c, wandb, newConditions)
}

func externalRedisInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition, newInfraConn *apiv2.RedisConnection) (ctrl.Result, error) {
	oldInfraConn := wandb.Status.RedisStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(wandb.Status.RedisStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.RedisStatus = apiv2.RedisInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, c.Status().Update(ctx, wandb)
}

// helpers

func managedRedisSpecNamespacedName(spec *apiv2.ManagedRedisSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
