package opstree

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StandaloneType  = "RedisStandalone"
	SentinelType    = "RedisSentinel"
	ReplicationType = "RedisReplication"
	AppConnTypeName = "RedisAppConn"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	standaloneDesired *redisv1beta2.Redis,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) []metav1.Condition {
	ctx, _ = logx.WithSlog(ctx, logx.Redis)
	results := make([]metav1.Condition, 0)

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	results = append(results, writeStandaloneState(ctx, client, nsnBuilder, standaloneDesired)...)
	results = append(results, writeSentinelState(ctx, client, nsnBuilder, sentinelDesired)...)
	results = append(results, writeReplicationState(ctx, client, nsnBuilder, replicationDesired)...)

	return results
}

func writeStandaloneState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	standaloneDesired *redisv1beta2.Redis,
) []metav1.Condition {
	var standaloneActual = &redisv1beta2.Redis{}

	found, err := common.GetResource(
		ctx, client, nsnBuilder.StandaloneNsName(), StandaloneType, standaloneActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   RedisStandaloneCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		standaloneActual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, client, standaloneDesired, standaloneActual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   RedisStandaloneCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   RedisStandaloneCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   RedisStandaloneCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   RedisStandaloneCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}

func writeSentinelState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
) []metav1.Condition {
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}

	found, err := common.GetResource(
		ctx, client, nsnBuilder.SentinelNsName(), SentinelType, sentinelActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   RedisSentinelCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		sentinelActual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, client, sentinelDesired, sentinelActual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   RedisSentinelCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   RedisSentinelCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   RedisSentinelCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   RedisSentinelCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}

func writeReplicationState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) []metav1.Condition {
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}

	found, err := common.GetResource(
		ctx, client, nsnBuilder.ReplicationNsName(), ReplicationType, replicationActual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   RedisReplicationCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}
	}
	if !found {
		replicationActual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, client, replicationDesired, replicationActual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   RedisReplicationCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   RedisReplicationCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   RedisReplicationCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   RedisReplicationCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return result
}

type redisConnInfo struct {
	Host           string
	Port           string
	SentinelHost   string
	SentinelPort   string
	SentinelMaster string
}

func (c *redisConnInfo) toURL() string {
	if c.SentinelHost != "" {
		return fmt.Sprintf("redis://%s:%s?master=%s", c.SentinelHost, c.SentinelPort, c.SentinelMaster)
	}
	return fmt.Sprintf("redis://%s:%s", c.Host, c.Port)
}
