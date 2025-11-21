package opstree

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StandaloneType  = "RedisStandalone"
	SentinelType    = "RedisSentinel"
	ReplicationType = "RedisReplication"
)

func CrudStandaloneResource(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	standaloneDesired *redisv1beta2.Redis,
) error {
	var standaloneActual = &redisv1beta2.Redis{}
	var err error

	if err = common.GetResource(
		ctx, client, StandaloneNamespacedName(specNamespacedName), StandaloneType, standaloneActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, standaloneDesired, standaloneActual)
}

func CrudSentinelResource(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
) error {
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var err error

	if err = common.GetResource(
		ctx, client, SentinelNamespacedName(specNamespacedName), SentinelType, sentinelActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, sentinelDesired, sentinelActual)
}

func CrudReplicationResource(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) error {
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var err error

	if err = common.GetResource(
		ctx, client, ReplicationNamespacedName(specNamespacedName), ReplicationType, replicationActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, replicationDesired, replicationActual)
}
