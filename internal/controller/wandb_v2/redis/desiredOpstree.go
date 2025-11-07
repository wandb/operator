package redis

import (
	"context"

	common "github.com/wandb/operator/api/redis-operator-vendored/common/v1beta2"
	"github.com/wandb/operator/api/redis-operator-vendored/redis/v1beta2"
	redisv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/api/redis-operator-vendored/redissentinel/v1beta2"
	v2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	OpstreeImage         = "quay.io/opstree/redis:v7.0.15"
	OpstreeSentinelImage = "quay.io/opstree/redis-sentinel:v7.0.12"
)

// desiredOpstree delegates to *build* the desired CRs for all OpsTree Redis CRs we work with:
// Redis, RedisReplication, RedisSentinel
func desiredOpstree(
	ctx context.Context,
	desiredRedis v2.WBRedisSpec,
	namespacedName types.NamespacedName,
) (opstree, error) {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var result = opstree{}

	if result.standalone, err = desiredOpstreeRedis(namespacedName, desiredRedis); err != nil {
		log.Error(err, "Failed to build desired opstree redis")
		return result, err
	}

	if result.replication, err = desiredOpstreeReplication(namespacedName, desiredRedis); err != nil {
		log.Error(err, "Failed to build desired opstree replication")
		return result, err
	}

	if result.sentinel, err = desiredOpstreeSentinel(namespacedName, desiredRedis); err != nil {
		log.Error(err, "Failed to build desired opstree sentinel")
		return result, err
	}

	return result, nil
}

// desiredOpstreeRedis will create an opstree Redis, unless `WbRedisSpec.Sentinel.Enabled` == true.
func desiredOpstreeRedis(
	namespacedName types.NamespacedName, wbSpec v2.WBRedisSpec,
) (
	*v1beta2.Redis, error,
) {
	if wbRedisSentinelEnabled(wbSpec) {
		return nil, nil
	}

	return &redisv1beta2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: redisv1beta2.RedisSpec{
			KubernetesConfig: common.KubernetesConfig{
				Image:           OpstreeImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			Storage: &common.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: wbSpec.Config.Resources.Requests[corev1.ResourceStorage],
							},
						},
					},
				},
			},
		},
	}, nil
}

// desiredOpstreeSentinel will build an opstree RedisSentinel, unless `wbRedisSentinelEnabled()` is false
func desiredOpstreeSentinel(
	namespacedName types.NamespacedName, wbSpec v2.WBRedisSpec,
) (
	*redissentinelv1beta2.RedisSentinel, error,
) {
	if !wbRedisSentinelEnabled(wbSpec) {
		return nil, nil
	}

	sentinelCount := int32(ReplicaSentinelCount)
	redisSentinel := redissentinelv1beta2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: redissentinelv1beta2.RedisSentinelSpec{
			Size: &sentinelCount,
			KubernetesConfig: common.KubernetesConfig{
				Image:           OpstreeSentinelImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			RedisSentinelConfig: &redissentinelv1beta2.RedisSentinelConfig{
				RedisSentinelConfig: common.RedisSentinelConfig{
					RedisReplicationName: NamePrefix,
					MasterGroupName:      DefaultSentinelGroup,
				},
			},
		},
	}
	return &redisSentinel, nil
}

// desiredOpstreeReplication will build an opstree RedisSentinel, unless `wbRedisSentinelEnabled()` is false
func desiredOpstreeReplication(
	namespacedName types.NamespacedName, wbSpec v2.WBRedisSpec,
) (
	*redisreplicationv1beta2.RedisReplication, error,
) {
	if !wbRedisSentinelEnabled(wbSpec) {
		return nil, nil
	}

	replicaCount := int32(ReplicaSentinelCount)
	result := redisreplicationv1beta2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: redisreplicationv1beta2.RedisReplicationSpec{
			Size: &replicaCount,
			KubernetesConfig: common.KubernetesConfig{
				Image:           OpstreeImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			Storage: &common.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: wbSpec.Config.Resources.Requests[corev1.ResourceStorage],
							},
						},
					},
				},
			},
		},
	}
	return &result, nil
}
