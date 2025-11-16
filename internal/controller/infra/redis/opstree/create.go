package opstree

import (
	"context"

	"github.com/wandb/operator/internal/model"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

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

	if err = controllerutil.SetOwnerReference(a.owner, desiredSentinel, a.scheme); err != nil {
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

	if err = controllerutil.SetOwnerReference(a.owner, desiredReplication, a.scheme); err != nil {
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

	if err = controllerutil.SetOwnerReference(a.owner, desiredStandalone, a.scheme); err != nil {
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
