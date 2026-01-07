package opstree

import (
	"context"
	"fmt"

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
	AppConnTypeName = "RedisAppConn"
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

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if err = writeStandaloneState(ctx, client, nsnBuilder, standaloneDesired); err != nil {
		return err
	}

	if err = writeSentinelState(ctx, client, nsnBuilder, sentinelDesired); err != nil {
		return err
	}

	if err = writeReplicationState(ctx, client, nsnBuilder, replicationDesired); err != nil {
		return err
	}

	return nil
}

func writeStandaloneState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	standaloneDesired *redisv1beta2.Redis,
) error {
	var standaloneActual = &redisv1beta2.Redis{}
	var err error
	var found bool

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.StandaloneNsName(), StandaloneType, standaloneActual,
	); err != nil {
		return err
	}
	if !found {
		standaloneActual = nil
	}

	_, result := common.CrudResource(ctx, client, standaloneDesired, standaloneActual)
	return result
}

func writeSentinelState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
) error {
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var err error
	var found bool

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.SentinelNsName(), SentinelType, sentinelActual,
	); err != nil {
		return err
	}
	if !found {
		sentinelActual = nil
	}

	_, result := common.CrudResource(ctx, client, sentinelDesired, sentinelActual)
	return result
}

func writeReplicationState(
	ctx context.Context,
	client client.Client,
	nsnBuilder *NsNameBuilder,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) error {
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var err error
	var found bool

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.ReplicationNsName(), ReplicationType, replicationActual,
	); err != nil {
		return err
	}
	if !found {
		replicationActual = nil
	}

	_, result := common.CrudResource(ctx, client, replicationDesired, replicationActual)
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
