package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validateRedisSpec(wandb *apiv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	redisPath := field.NewPath("spec").Child("redis")
	spec := wandb.Spec.Redis

	if !spec.Enabled {
		return errors
	}

	if spec.StorageSize != "" {
		if _, err := resource.ParseQuantity(spec.StorageSize); err != nil {
			errors = append(errors, field.Invalid(
				redisPath.Child("storageSize"),
				spec.StorageSize,
				"must be a valid resource quantity (e.g., '10Gi')",
			))
		}
	}

	if spec.Sentinel.Enabled {
		if !spec.Enabled {
			errors = append(errors, field.Invalid(
				redisPath.Child("sentinel").Child("enabled"),
				spec.Sentinel.Enabled,
				"Redis Sentinel cannot be enabled when Redis is disabled",
			))
		}
	}

	return errors
}

func validateRedisChanges(newWandb, oldWandb *apiv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	redisPath := field.NewPath("spec").Child("redis")
	newSpec := newWandb.Spec.Redis
	oldSpec := oldWandb.Spec.Redis

	if !newSpec.Enabled {
		return errors
	}

	// storageSize may be initially set as part of an update but cannot be changed afterwards
	if oldSpec.StorageSize != "" &&
		oldSpec.StorageSize != newSpec.StorageSize {
		errors = append(errors, field.Invalid(
			redisPath.Child("storageSize"),
			newSpec.StorageSize,
			"storageSize may not be changed",
		))
	}

	if oldSpec.Namespace != newSpec.Namespace {
		errors = append(errors, field.Invalid(
			redisPath.Child("namespace"),
			newSpec.Namespace,
			"namespace may not be changed",
		))
	}

	if oldSpec.Sentinel.Enabled != newSpec.Sentinel.Enabled {
		if !newSpec.Enabled {
			errors = append(errors, field.Invalid(
				redisPath.Child("sentinel").Child("enabled"),
				newSpec.Sentinel.Enabled,
				"Redis Sentinel cannot be toggled between enabled and disabled (yet)",
			))
		}
	}

	return errors
}
