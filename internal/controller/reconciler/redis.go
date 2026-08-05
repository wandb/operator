package reconciler

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	externalredis "github.com/wandb/operator/internal/controller/infra/external/redis"
	"github.com/wandb/operator/internal/controller/infra/managed/redis/opstree"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
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
	mfst manifest.Manifest,
) map[string][]metav1.Condition {
	out := map[string][]metav1.Condition{}
	for key, spec := range wandb.Spec.Redis {
		switch {
		case spec.ManagedRedis != nil:
			out[key] = managedRedisWriteState(ctx, client, wandb, spec.ManagedRedis, mfst)
		case spec.ExternalRedis != nil:
			out[key] = externalredis.WriteState(ctx, client, wandb, key, spec.ExternalRedis)
		}
	}
	return out
}

func redisReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	conditions map[string][]metav1.Condition,
) (map[string][]metav1.Condition, map[string]*apiv2.RedisConnection) {
	outConds := map[string][]metav1.Condition{}
	outConns := map[string]*apiv2.RedisConnection{}
	for key, spec := range wandb.Spec.Redis {
		switch {
		case spec.ManagedRedis != nil:
			outConds[key], outConns[key] = managedRedisReadState(ctx, client, wandb, spec.ManagedRedis, conditions[key])
		case spec.ExternalRedis != nil:
			outConds[key], outConns[key] = externalredis.ReadState(ctx, client, wandb, key, conditions[key])
		default:
			outConds[key] = conditions[key]
		}
	}
	return outConds, outConns
}

func redisInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	conditions map[string][]metav1.Condition,
	infraConns map[string]*apiv2.RedisConnection,
) (ctrl.Result, error) {
	if wandb.Status.RedisStatus == nil {
		wandb.Status.RedisStatus = map[string]apiv2.RedisInfraStatus{}
	}
	var results []ctrl.Result
	var firstErr error
	for key, spec := range wandb.Spec.Redis {
		var res ctrl.Result
		var err error
		switch {
		case spec.ManagedRedis != nil:
			res, err = managedRedisInferStatus(ctx, client, recorder, wandb, key, conditions[key], infraConns[key])
		case spec.ExternalRedis != nil:
			res, err = externalRedisInferStatus(ctx, client, wandb, key, conditions[key], infraConns[key])
		}
		results = append(results, res)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return consolidateResults(results), firstErr
}

func runRedisRetentionFinalizer(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, spec apiv2.RedisSpec) error {
	switch wandb.GetRetentionPolicy(redisInstanceInfraSpec(spec)).OnDelete {
	case apiv2.PurgeOnDelete:
		return redisPurgeFinalizer(ctx, c, wandb, key, spec)
	case apiv2.DetachOnDelete:
		return redisDetachFinalizer(ctx, c, wandb, key, spec)
	}
	return nil
}

func redisInstanceInfraSpec(spec apiv2.RedisSpec) apiv2.ManagedInfraSpec {
	if spec.ManagedRedis != nil {
		return spec.ManagedRedis.ManagedInfraSpec
	}
	return apiv2.ManagedInfraSpec{}
}

func redisPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec apiv2.RedisSpec,
) error {
	if managed := spec.ManagedRedis; managed != nil {
		specNamespacedName := managedRedisSpecNamespacedName(managed)
		onDeleteRule := opstree.ToRedisOnDeleteRule(wandb, wandb.GetRetentionPolicy(managed.ManagedInfraSpec))
		return opstree.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if spec.ExternalRedis != nil {
		return externalredis.DeleteConnectionSecret(ctx, client, wandb, key)
	}
	return nil
}

func redisDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	_ string,
	spec apiv2.RedisSpec,
) error {
	managed := spec.ManagedRedis
	if managed == nil {
		return nil
	}
	specNamespacedName := managedRedisSpecNamespacedName(managed)
	return opstree.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedRedisWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	spec *apiv2.ManagedRedisSpec,
	mfst manifest.Manifest,
) []metav1.Condition {
	log := ctrl.LoggerFrom(ctx)
	var specNamespacedName = managedRedisSpecNamespacedName(spec)

	standaloneDesired, err := opstree.ToRedisStandaloneVendorSpec(ctx, wandb, spec, client.Scheme(), mfst)
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

	sentinelDesired, err := opstree.ToRedisSentinelVendorSpec(ctx, wandb, spec, client.Scheme(), mfst)
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

	replicationDesired, err := opstree.ToRedisReplicationVendorSpec(ctx, wandb, spec, client.Scheme(), mfst)
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
	spec *apiv2.ManagedRedisSpec,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.RedisConnection) {
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
	key string,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.RedisConnection,
) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	enabled := true
	oldStatus := wandb.Status.RedisStatus[key]
	oldConditions := oldStatus.Conditions
	oldInfraConn := oldStatus.Connection

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
	wandb.Status.RedisStatus[key] = updatedStatus
	err := updateWandbStatusIfChanged(ctx, client, wandb, statusBefore)

	return ctrlResult, err
}

// external

func externalRedisInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, newConditions []metav1.Condition, newInfraConn *apiv2.RedisConnection) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	oldStatus := wandb.Status.RedisStatus[key]
	oldInfraConn := oldStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(oldStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.RedisStatus[key] = apiv2.RedisInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, updateWandbStatusIfChanged(ctx, c, wandb, statusBefore)
}

// helpers

func managedRedisSpecNamespacedName(spec *apiv2.ManagedRedisSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
