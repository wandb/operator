package opstree

import (
	"context"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	transcommon "github.com/wandb/operator/internal/controller/translator/common"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readStandaloneConnectionDetails(standaloneActual *redisv1beta2.Redis) *redisConnInfo {
	redisHost := "wandb-redis." + standaloneActual.Namespace + ".svc.cluster.local"
	redisPort := "6379"

	return &redisConnInfo{
		Host: redisHost,
		Port: redisPort,
	}
}

func readSentinelConnectionDetails(sentinelActual *redissentinelv1beta2.RedisSentinel) *redisConnInfo {
	sentinelHost := "wandb-redis-sentinel." + sentinelActual.Namespace + ".svc.cluster.local"
	sentinelPort := "26379"
	masterName := "gorilla"

	return &redisConnInfo{
		SentinelHost:   sentinelHost,
		SentinelPort:   sentinelPort,
		SentinelMaster: masterName,
	}
}

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) ([]transcommon.RedisCondition, error) {
	var standaloneActual = &redisv1beta2.Redis{}
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var results []transcommon.RedisCondition
	var err error

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.StandaloneNsName(), StandaloneType, standaloneActual,
	); err != nil {
		return results, err
	}
	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.SentinelNsName(), SentinelType, sentinelActual,
	); err != nil {
		return results, err
	}
	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.ReplicationNsName(), ReplicationType, replicationActual,
	); err != nil {
		return results, err
	}

	if standaloneActual != nil {
		connInfo := readStandaloneConnectionDetails(standaloneActual)

		var connection *transcommon.RedisConnection
		if connection, err = writeRedisConnInfo(
			ctx, client, wandbOwner, nsNameBldr, connInfo,
		); err != nil {
			return results, err
		}

		results = append(results, transcommon.NewRedisStandaloneConnCondition(*connection))
	}

	if sentinelActual != nil && replicationActual != nil {
		connInfo := readSentinelConnectionDetails(sentinelActual)

		var connection *transcommon.RedisConnection
		if connection, err = writeRedisConnInfo(
			ctx, client, wandbOwner, nsNameBldr, connInfo,
		); err != nil {
			return results, err
		}

		results = append(results, transcommon.NewRedisSentinelConnCondition(*connection))
	}

	return results, nil
}
