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
	namespacedName types.NamespacedName,
) ([]common.InfraStatusDetail, error) {
	var standaloneActual = &redisv1beta2.Redis{}
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var results []common.InfraStatusDetail
	var err error

	if err = ctrlcommon.GetResource(
		ctx, client, namespacedName, StandaloneType, standaloneActual,
	); err != nil {
		return results, err
	}
	if err = ctrlcommon.GetResource(
		ctx, client, namespacedName, SentinelType, sentinelActual,
	); err != nil {
		return results, err
	}
	if err = ctrlcommon.GetResource(
		ctx, client, namespacedName, ReplicationType, replicationActual,
	); err != nil {
		return results, err
	}

	///////////
	// Extract connection info from PXC CR
	// Connection endpoint depends on configuration:
	// - Dev (no ProxySQL): connect directly to PXC service
	// - HA (with ProxySQL): connect via ProxySQL service
	namespace := namespacedName.Namespace
	if standaloneActual != nil {
		redisHost := "wandb-redis." + namespace + ".svc.cluster.local"
		redisPort := "6379"
		connInfo := common.RedisStandaloneConnInfo{
			Host: redisHost,
			Port: redisPort,
		}
		results = append(results, common.NewRedisStandaloneConnDetail(connInfo))
	}

	if sentinelActual != nil && replicationActual != nil {
		sentinelHost := "wandb-redis-sentinel." + namespace + ".svc.cluster.local"
		sentinelPort := "26379"
		masterName := "gorilla"
		connInfo := common.RedisSentinelConnInfo{
			SentinelHost: sentinelHost,
			SentinelPort: sentinelPort,
			MasterName:   masterName,
		}
		results = append(results, common.NewRedisSentinelConnDetail(connInfo))
	}
	///////////

	return results, nil
}
