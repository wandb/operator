package opstree

import (
	"context"

	redisv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redissentinel/v1beta2"
	"github.com/wandb/operator/internal/model"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type opstreeRedis struct {
	standalone  *redisv1beta2.Redis
	replication *redisreplicationv1beta2.RedisReplication
	sentinel    *redissentinelv1beta2.RedisSentinel
	client      client.Client
	owner       metav1.Object
}

// Initialize delegates to *get* all OpsTree Redis CRs we work with:
// Redis, RedisSentinel and RedisReplication
func Initialize(
	ctx context.Context,
	client client.Client,
	redisConfig model.RedisConfig,
	owner metav1.Object,
) (
	*opstreeRedis, error,
) {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var standalone = &redisv1beta2.Redis{}
	var replication = &redisreplicationv1beta2.RedisReplication{}
	var sentinel = &redissentinelv1beta2.RedisSentinel{}
	var result = opstreeRedis{
		client:      client,
		owner:       owner,
		standalone:  nil,
		replication: nil,
		sentinel:    nil,
	}

	err = client.Get(ctx, standaloneNamespacedName(redisConfig.Namespace), standalone)
	if err != nil {
		if !machErrors.IsNotFound(err) {
			log.Error(err, "error getting actual opstree redis standalone")
			return nil, err
		}
	} else {
		result.standalone = standalone
	}

	err = client.Get(ctx, sentinelNamespacedName(redisConfig.Namespace), sentinel)
	if err != nil {
		if !machErrors.IsNotFound(err) {
			log.Error(err, "error getting actual opstree redis sentinel")
			return nil, err
		}
	} else {
		result.sentinel = sentinel
	}

	err = client.Get(ctx, replicationNamespacedName(redisConfig.Namespace), replication)
	if err != nil {
		if !machErrors.IsNotFound(err) {
			log.Error(err, "error getting actual opstree redis replication")
			return nil, err
		}
	} else {
		result.replication = replication
	}

	return &result, nil
}

func (a *opstreeRedis) Upsert(ctx context.Context, redisConfig model.RedisConfig) *model.Results {
	//log := ctrl.LoggerFrom(ctx)
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

func (a *opstreeRedis) createSentinel(
	ctx context.Context, desiredSentinel *redissentinelv1beta2.RedisSentinel,
) *model.Results {
	log := ctrl.LoggerFrom(ctx)

	var results = model.InitResults()
	var msg string
	var err error

	if a.standalone != nil {
		msg = "cannot create desiredSentinel with standalone CR present"
		err = model.NewRedisError(model.RedisDeploymentConflict, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return results
	}

	if a.sentinel != nil {
		msg = "cannot create desiredSentinel when its CR is already present"
		err = model.NewRedisError(model.RedisDeploymentConflict, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return results
	}

	if err = controllerutil.SetOwnerReference(a.owner, desiredSentinel, a.client.Scheme()); err != nil {
		log.Error(err, "Failed to set owner reference for RedisSentinel")
		results.AddErrors(err)
		return results
	}

	if err = a.client.Create(ctx, desiredSentinel); err != nil {
		log.Error(err, "Failed to create Redis Sentinel")
		results.AddErrors(err)
		return results
	}

	results.AddStatuses(
		model.NewRedisStatus(model.RedisSentinelCreated, sentinelName),
	)

	return results
}

func (a *opstreeRedis) createReplication(
	ctx context.Context, desiredReplication *redisreplicationv1beta2.RedisReplication,
) *model.Results {
	log := ctrl.LoggerFrom(ctx)

	var results = model.InitResults()
	var msg string
	var err error

	if a.standalone != nil {
		msg = "cannot create desiredReplication with standalone CR present"
		err = model.NewRedisError(model.RedisDeploymentConflict, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return results
	}

	if a.replication != nil {
		msg = "cannot create desiredReplication when its CR is already present"
		err = model.NewRedisError(model.RedisDeploymentConflict, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return results
	}

	if err = controllerutil.SetOwnerReference(a.owner, desiredReplication, a.client.Scheme()); err != nil {
		log.Error(err, "Failed to set owner reference for RedisReplication")
		results.AddErrors(err)
		return results
	}

	if err = a.client.Create(ctx, desiredReplication); err != nil {
		log.Error(err, "Failed to create Redis Replication")
		results.AddErrors(err)
		return results
	}

	results.AddStatuses(
		model.NewRedisStatus(model.RedisReplicationCreated, replicationName),
	)

	return results
}

func (a *opstreeRedis) createStandalone(
	ctx context.Context, desiredStandalone *redisv1beta2.Redis,
) *model.Results {
	log := ctrl.LoggerFrom(ctx)

	var results = model.InitResults()
	var msg string
	var err error

	if a.sentinel != nil {
		msg = "cannot create desiredStandalone with sentinel CR present"
		err = model.NewRedisError(model.RedisDeploymentConflict, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return results
	}

	if a.replication != nil {
		msg = "cannot create desiredStandalone with replication CR present"
		err = model.NewRedisError(model.RedisDeploymentConflict, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return results
	}

	if a.standalone != nil {
		msg = "cannot create desiredStandalone when its CR is already present"
		err = model.NewRedisError(model.RedisDeploymentConflict, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return results
	}

	if err = controllerutil.SetOwnerReference(a.owner, desiredStandalone, a.client.Scheme()); err != nil {
		log.Error(err, "Failed to set owner reference for RedisStandalone")
		results.AddErrors(err)
		return results
	}

	if err = a.client.Create(ctx, desiredStandalone); err != nil {
		log.Error(err, "Failed to create Redis Standalone")
		results.AddErrors(err)
		return results
	}

	results.AddStatuses(
		model.NewRedisStatus(model.RedisStandaloneCreated, standaloneName),
	)

	return results
}

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
	masterName := "fake-master-name"
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
