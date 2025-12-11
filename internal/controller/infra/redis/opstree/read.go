package opstree

import (
	"context"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) (*translator.RedisStatus, error) {
	var standaloneActual = &redisv1beta2.Redis{}
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var status = &translator.RedisStatus{}
	var err error
	var found bool

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if found, err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.StandaloneNsName(), StandaloneType, standaloneActual,
	); err != nil {
		return nil, err
	}
	if !found {
		standaloneActual = nil
	}
	if found, err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.SentinelNsName(), SentinelType, sentinelActual,
	); err != nil {
		return nil, err
	}
	if !found {
		sentinelActual = nil
	}
	if found, err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.ReplicationNsName(), ReplicationType, replicationActual,
	); err != nil {
		return nil, err
	}
	if !found {
		replicationActual = nil
	}

	///////////////////////////////////
	// set connection details

	if standaloneActual != nil {
		connInfo := readStandaloneConnectionDetails(standaloneActual)

		var connection *translator.InfraConnection
		if connection, err = writeRedisConnInfo(
			ctx, client, wandbOwner, nsNameBldr, connInfo,
		); err != nil {
			return nil, err
		}

		if connection != nil {
			status.Connection = *connection
		}
	}

	if sentinelActual != nil && replicationActual != nil {
		connInfo := readSentinelConnectionDetails(sentinelActual)

		var connection *translator.InfraConnection
		if connection, err = writeRedisConnInfo(
			ctx, client, wandbOwner, nsNameBldr, connInfo,
		); err != nil {
			return nil, err
		}

		if connection != nil {
			status.Connection = *connection
		}
	}

	///////////////////////////////////
	// add conditions

	///////////////////////////////////
	// set top-level summary

	return status, nil
}
