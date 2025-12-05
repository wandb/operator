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

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	standaloneDesired *redisv1beta2.Redis,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) error {
	var err error

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = writeStandaloneState(ctx, client, nsNameBldr, standaloneDesired); err != nil {
		return err
	}

	if err = writeSentinelState(ctx, client, nsNameBldr, sentinelDesired); err != nil {
		return err
	}

	if err = writeReplicationState(ctx, client, nsNameBldr, replicationDesired); err != nil {
		return err
	}

	return nil
}

func writeStandaloneState(
	ctx context.Context,
	client client.Client,
	nsNameBldr *NsNameBuilder,
	standaloneDesired *redisv1beta2.Redis,
) error {
	var standaloneActual = &redisv1beta2.Redis{}
	var err error

	if err = common.GetResource(
		ctx, client, nsNameBldr.StandaloneNsName(), StandaloneType, standaloneActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, standaloneDesired, standaloneActual)
}

func writeSentinelState(
	ctx context.Context,
	client client.Client,
	nsNameBldr *NsNameBuilder,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
) error {
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var err error

	if err = common.GetResource(
		ctx, client, nsNameBldr.SentinelNsName(), SentinelType, sentinelActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, sentinelDesired, sentinelActual)
}

func writeReplicationState(
	ctx context.Context,
	client client.Client,
	nsNameBldr *NsNameBuilder,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) error {
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var err error

	if err = common.GetResource(
		ctx, client, nsNameBldr.ReplicationNsName(), ReplicationType, replicationActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, replicationDesired, replicationActual)
}
