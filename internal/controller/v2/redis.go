package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/redis/opstree"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
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
		onDeleteRule := translatorv2.ToRedisOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return opstree.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if wandb.Spec.Redis.ExternalRedis != nil {
		return deleteWandbConnectionSecret(ctx, client, types.NamespacedName{
			Namespace: wandb.Namespace,
			Name:      redisConnectionName,
		})
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
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.Redis.ExternalRedis
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"url":      spec.URL,
		"Host":     spec.Host,
		"Port":     spec.Port,
		"Password": spec.Password,
		"Tls":      spec.Tls,
		"SslCa":    spec.SslCa,
	}

	data := map[string]string{}
	for key, sel := range fields {
		val, err := resolveSecretKey(ctx, c, wandb.Namespace, sel)
		if err != nil {
			logger.Error(err, "failed to resolve external redis field", "key", key)
			return []metav1.Condition{{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			}}
		}
		if val != "" {
			data[key] = val
		}
	}

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: redisConnectionName}
	return writeExternalConnectionSecret(ctx, c, wandb, nsName, data)
}

func externalRedisReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.RedisConnection) {
	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: redisConnectionName}
	secret := &corev1.Secret{}
	found, err := common.GetResource(ctx, c, nsName, "Secret", secret)
	if err != nil {
		return append(newConditions, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		}), nil
	}
	if !found {
		return newConditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return newConditions, &translator.RedisConnection{
		URL:  corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Host: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
	}
}

func externalRedisInferStatus(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.RedisConnection,
) (ctrl.Result, error) {
	oldInfraConn := translatorv2.ToTranslatorRedisConnection(wandb.Status.RedisStatus.Connection)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	state := common.HealthyState
	ready := true
	if newInfraConn == nil {
		state = common.ErrorState
		ready = false
	}

	updatedConditions := common.ComputeConditionUpdates(
		wandb.Status.RedisStatus.Conditions,
		newConditions,
		wandb.Generation,
		translator.DefaultConditionExpiry,
	)

	wandb.Status.RedisStatus = translatorv2.ToWbRedisInfraStatus(translator.RedisStatus{
		InfraStatus: translator.InfraStatus{
			Ready:      ready,
			State:      state,
			Conditions: updatedConditions,
		},
		Connection: *conn,
	})
	return ctrl.Result{}, c.Status().Update(ctx, wandb)
}

// helpers

func managedRedisSpecNamespacedName(spec *apiv2.ManagedRedisSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
