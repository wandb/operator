package opstree

import (
	"context"

	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/internal/model"
	common "github.com/wandb/operator/internal/vendored/redis-operator/common/v1beta2"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// buildDesiredStandalone will create an opstree Redis, unless `WbRedisSpec.Sentinel.Enabled` == true.
func buildDesiredStandalone(
	ctx context.Context,
	redisConfig model.RedisConfig,
) (
	*redisv1beta2.Redis, *model.Results,
) {
	log := ctrl.LoggerFrom(ctx)
	var results = model.InitResults()
	var err error
	var msg string

	if redisConfig.IsHighAvailability() {
		msg = "cannot create redis standalone high availability configuration"
		err = model.NewRedisError(model.RedisDeploymentConflictCode, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return nil, results
	}

	return &redisv1beta2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namePrefix,
			Namespace: redisConfig.Namespace,
		},
		Spec: redisv1beta2.RedisSpec{
			KubernetesConfig: common.KubernetesConfig{
				Image:           standaloneImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources: &corev1.ResourceRequirements{
					Requests: redisConfig.Requests,
					Limits:   redisConfig.Limits,
				},
			},
			Storage: &common.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: redisConfig.StorageSize,
							},
						},
					},
				},
			},
		},
	}, results
}

// buildDesiredSentinel will build an opstree RedisSentinel, unless `wbRedisSentinelEnabled()` is false
func buildDesiredSentinel(
	ctx context.Context, redisConfig model.RedisConfig,
) (
	*redissentinelv1beta2.RedisSentinel, *model.Results,
) {
	log := ctrl.LoggerFrom(ctx)
	var results = model.InitResults()
	var err error
	var msg string

	if !redisConfig.IsHighAvailability() {
		msg = "cannot create redis sentinel without high availability configuration"
		err = model.NewRedisError(model.RedisDeploymentConflictCode, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return nil, results
	}

	sentinelCount := int32(redisConfig.Sentinel.ReplicaCount)
	sentinel := redissentinelv1beta2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namePrefix,
			Namespace: redisConfig.Namespace,
		},
		Spec: redissentinelv1beta2.RedisSentinelSpec{
			Size: &sentinelCount,
			KubernetesConfig: common.KubernetesConfig{
				Image:           sentinelImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources: &corev1.ResourceRequirements{
					Requests: redisConfig.Sentinel.Requests,
					Limits:   redisConfig.Sentinel.Limits,
				},
			},
			RedisSentinelConfig: &redissentinelv1beta2.RedisSentinelConfig{
				RedisSentinelConfig: common.RedisSentinelConfig{
					RedisReplicationName: namePrefix,
					MasterGroupName:      translatorv2.DefaultSentinelGroup,
				},
			},
		},
	}
	return &sentinel, results
}

// buildDesiredReplication will build an opstree RedisSentinel, unless `wbRedisSentinelEnabled()` is false
func buildDesiredReplication(
	ctx context.Context, redisDetails model.RedisConfig,
) (
	*redisreplicationv1beta2.RedisReplication, *model.Results,
) {
	log := ctrl.LoggerFrom(ctx)
	var results = model.InitResults()
	var err error
	var msg string

	if !redisDetails.IsHighAvailability() {
		msg = "cannot create redis replication without high availability configuration"
		err = model.NewRedisError(model.RedisDeploymentConflictCode, msg)
		log.Error(err, msg, vendorKey, vendorName)
		results.AddErrors(err)
		return nil, results
	}

	replicaCount := int32(redisDetails.Sentinel.ReplicaCount)
	replication := redisreplicationv1beta2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namePrefix,
			Namespace: redisDetails.Namespace,
		},
		Spec: redisreplicationv1beta2.RedisReplicationSpec{
			Size: &replicaCount,
			KubernetesConfig: common.KubernetesConfig{
				Image:           replicationImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources: &corev1.ResourceRequirements{
					Requests: redisDetails.Requests,
					Limits:   redisDetails.Limits,
				},
			},
			Storage: &common.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: redisDetails.StorageSize,
							},
						},
					},
				},
			},
		},
	}
	return &replication, results
}
