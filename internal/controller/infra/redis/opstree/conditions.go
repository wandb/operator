package opstree

import (
	"context"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator/common"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConditions(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
) ([]common.RedisCondition, error) {
	var standaloneActual = &redisv1beta2.Redis{}
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var results []common.RedisCondition
	var err error

	if err = ctrlcommon.GetResource(
		ctx, client, StandaloneNamespacedName(specNamespacedName), StandaloneType, standaloneActual,
	); err != nil {
		return results, err
	}
	if err = ctrlcommon.GetResource(
		ctx, client, SentinelNamespacedName(specNamespacedName), SentinelType, sentinelActual,
	); err != nil {
		return results, err
	}
	if err = ctrlcommon.GetResource(
		ctx, client, ReplicationNamespacedName(specNamespacedName), ReplicationType, replicationActual,
	); err != nil {
		return results, err
	}

	///////////
	if standaloneActual != nil {
		redisHost := "wandb-redis." + standaloneActual.Namespace + ".svc.cluster.local"
		redisPort := "6379"
		connInfo := common.RedisStandaloneConnInfo{
			Host: redisHost,
			Port: redisPort,
		}
		results = append(results, common.NewRedisStandaloneConnCondition(connInfo))
	}

	if sentinelActual != nil && replicationActual != nil {
		sentinelHost := "wandb-redis-sentinel." + sentinelActual.Namespace + ".svc.cluster.local"
		sentinelPort := "26379"
		masterName := "gorilla"
		connInfo := common.RedisSentinelConnInfo{
			SentinelHost: sentinelHost,
			SentinelPort: sentinelPort,
			MasterName:   masterName,
		}
		results = append(results, common.NewRedisSentinelConnCondition(connInfo))
	}
	///////////

	return results, nil
}
