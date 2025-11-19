package opstree

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator/common"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
)

func (a *opstreeRedis) updateStandalone(
	ctx context.Context, desiredStandalone *redisv1beta2.Redis,
) *common.Results {
	var results = common.InitResults()

	namespace := a.standalone.Namespace
	redisHost := "wandb-redis." + namespace + ".svc.cluster.local"
	redisPort := "6379"
	connInfo := common.RedisStandaloneConnInfo{
		Host: redisHost,
		Port: redisPort,
	}
	results.AddStatuses(common.NewRedisStandaloneConnDetail(connInfo))

	return results
}

func (a *opstreeRedis) updateSentinel(
	ctx context.Context, desiredSentinel *redissentinelv1beta2.RedisSentinel,
) *common.Results {
	var results = common.InitResults()

	namespace := a.sentinel.Namespace
	sentinelHost := "wandb-redis-sentinel." + namespace + ".svc.cluster.local"
	sentinelPort := "26379"
	masterName := a.config.Sentinel.MasterGroupName
	connInfo := common.RedisSentinelConnInfo{
		SentinelHost: sentinelHost,
		SentinelPort: sentinelPort,
		MasterName:   masterName,
	}
	results.AddStatuses(common.NewRedisSentinelConnDetail(connInfo))

	return results
}

func (a *opstreeRedis) updateReplication(
	ctx context.Context, desiredReplication *redisreplicationv1beta2.RedisReplication,
) *common.Results {
	var results = common.InitResults()

	return results
}
