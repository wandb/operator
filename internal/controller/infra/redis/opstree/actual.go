package opstree

import (
	"context"

	"github.com/wandb/operator/internal/model"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type opstreeRedis struct {
	standalone  *redisv1beta2.Redis
	replication *redisreplicationv1beta2.RedisReplication
	sentinel    *redissentinelv1beta2.RedisSentinel
	config      model.RedisConfig
	client      client.Client
	owner       metav1.Object
	scheme      *runtime.Scheme
}

// Initialize delegates to *get* all OpsTree Redis CRs we work with:
// Redis, RedisSentinel and RedisReplication
func Initialize(
	ctx context.Context,
	client client.Client,
	redisConfig model.RedisConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (
	*opstreeRedis, error,
) {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var standalone = &redisv1beta2.Redis{}
	var replication = &redisreplicationv1beta2.RedisReplication{}
	var sentinel = &redissentinelv1beta2.RedisSentinel{}
	var result = opstreeRedis{
		config:      redisConfig,
		client:      client,
		owner:       owner,
		scheme:      scheme,
		standalone:  nil,
		replication: nil,
		sentinel:    nil,
	}

	err = client.Get(ctx, standaloneNamespacedName(redisConfig.Namespace), standalone)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting actual opstree redis standalone")
			return nil, err
		}
	} else {
		result.standalone = standalone
	}

	err = client.Get(ctx, sentinelNamespacedName(redisConfig.Namespace), sentinel)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting actual opstree redis sentinel")
			return nil, err
		}
	} else {
		result.sentinel = sentinel
	}

	err = client.Get(ctx, replicationNamespacedName(redisConfig.Namespace), replication)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting actual opstree redis replication")
			return nil, err
		}
	} else {
		result.replication = replication
	}

	return &result, nil
}

func (a *opstreeRedis) Upsert(ctx context.Context, redisConfig model.RedisConfig) *model.Results {
	var results = model.InitResults()

	var nextResults *model.Results

	if redisConfig.IsHighAvailability() {
		var desiredSentinel *redissentinelv1beta2.RedisSentinel
		var desiredReplication *redisreplicationv1beta2.RedisReplication

		desiredSentinel, nextResults = buildDesiredSentinel(ctx, redisConfig)
		results.Merge(nextResults)
		if results.HasCriticalError() {
			return results
		}

		desiredReplication, nextResults = buildDesiredReplication(ctx, redisConfig)
		results.Merge(nextResults)
		if results.HasCriticalError() {
			return results
		}

		if a.sentinel == nil {
			a.createSentinel(ctx, desiredSentinel)
		} else {
			a.updateSentinel(ctx, desiredSentinel)
		}

		if a.replication == nil {
			a.createReplication(ctx, desiredReplication)
		} else {
			a.updateReplication(ctx, desiredReplication)
		}

	} else {
		var desiredStandalone *redisv1beta2.Redis

		desiredStandalone, nextResults = buildDesiredStandalone(ctx, redisConfig)
		results.Merge(nextResults)
		if results.HasCriticalError() {
			return results
		}
		if a.standalone == nil {
			a.createStandalone(ctx, desiredStandalone)
		} else {
			a.updateStandalone(ctx, desiredStandalone)
		}
	}

	return results
}

func (a *opstreeRedis) Delete(ctx context.Context) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	var err error
	var results = model.InitResults()

	if a.standalone != nil {
		if err = a.client.Delete(ctx, a.standalone); err != nil {
			log.Error(err, "Failed to delete OpsTree Redis")
			results.AddErrors(err)
			return results
		}
		results.AddStatuses(model.NewRedisStatus(model.RedisStandaloneDeleted, a.standalone.Name))
	}
	if a.sentinel != nil {
		if err = a.client.Delete(ctx, a.sentinel); err != nil {
			log.Error(err, "Failed to delete OpsTree RedisSentinel")
			results.AddErrors(err)
			return results
		}
		results.AddStatuses(model.NewRedisStatus(model.RedisSentinelDeleted, a.sentinel.Name))
	}
	if a.replication != nil {
		if err = a.client.Delete(ctx, a.replication); err != nil {
			log.Error(err, "Failed to delete OpsTree RedisReplication")
			results.AddErrors(err)
			return results
		}
		results.AddStatuses(model.NewRedisStatus(model.RedisReplicationDeleted, a.replication.Name))
	}
	return nil
}
