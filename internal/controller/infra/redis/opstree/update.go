package opstree

import (
	"context"

	redisv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redissentinel/v1beta2"
	"github.com/wandb/operator/internal/model"
)

func (a *opstreeRedis) updateStandalone(
	ctx context.Context, desiredStandalone *redisv1beta2.Redis,
) *model.Results {
	var results = model.InitResults()

	namespace := a.standalone.Namespace
	redisHost := "wandb-redis." + namespace + ".svc.cluster.local"
	redisPort := "6379"
	connInfo := model.RedisStandaloneConnInfo{
		Host: redisHost,
		Port: redisPort,
	}
	results.AddStatuses(model.NewRedisStandaloneConnDetail(connInfo))

	return results
}

func (a *opstreeRedis) updateSentinel(
	ctx context.Context, desiredSentinel *redissentinelv1beta2.RedisSentinel,
) *model.Results {
	var results = model.InitResults()

	namespace := a.sentinel.Namespace
	sentinelHost := "wandb-redis-sentinel." + namespace + ".svc.cluster.local"
	sentinelPort := "26379"
	masterName := a.config.Sentinel.MasterGroupName
	connInfo := model.RedisSentinelConnInfo{
		SentinelHost: sentinelHost,
		SentinelPort: sentinelPort,
		MasterName:   masterName,
	}
	results.AddStatuses(model.NewRedisSentinelConnDetail(connInfo))

	return results
}

func (a *opstreeRedis) updateReplication(
	ctx context.Context, desiredReplication *redisreplicationv1beta2.RedisReplication,
) *model.Results {
	var results = model.InitResults()

	return results
}
