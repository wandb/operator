package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/redis/opstree"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/defaults"
	rediscommon "github.com/wandb/operator/internal/vendored/redis-operator/common/v1beta2"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultSentinelGroup = defaults.DefaultSentinelGroup
)

func ExtractRedisStatus(ctx context.Context, conditions []common.RedisCondition) apiv2.WBRedisStatus {
	return TranslateRedisStatus(
		ctx,
		common.ExtractRedisStatus(ctx, conditions),
	)
}

func TranslateRedisStatus(ctx context.Context, m common.RedisStatus) apiv2.WBRedisStatus {
	var result apiv2.WBRedisStatus
	var conditions []apiv2.WBStatusCondition

	for _, condition := range m.Conditions {
		state := translateRedisStatusCode(condition.Code())
		conditions = append(conditions, apiv2.WBStatusCondition{
			State:   state,
			Code:    condition.Code(),
			Message: condition.Message(),
		})
	}

	result.Connection = apiv2.WBRedisConnection{
		RedisHost:         m.Connection.RedisHost,
		RedisPort:         m.Connection.RedisPort,
		RedisSentinelHost: m.Connection.SentinelHost,
		RedisSentinelPort: m.Connection.SentinelPort,
		RedisMasterName:   m.Connection.SentinelMaster,
	}

	result.Ready = m.Ready
	result.Conditions = conditions
	result.State = computeOverallState(conditions, m.Ready)
	result.LastReconciled = metav1.Now()

	return result
}

func translateRedisStatusCode(code string) apiv2.WBStateType {
	switch code {
	case string(common.RedisSentinelCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.RedisReplicationCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.RedisStandaloneCreatedCode):
		return apiv2.WBStateUpdating
	case string(common.RedisSentinelDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.RedisReplicationDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.RedisStandaloneDeletedCode):
		return apiv2.WBStateDeleting
	case string(common.RedisSentinelConnectionCode):
		return apiv2.WBStateReady
	case string(common.RedisStandaloneConnectionCode):
		return apiv2.WBStateReady
	default:
		return apiv2.WBStateUnknown
	}
}

// ToRedisStandaloneVendorSpec converts a WBRedisSpec to a Redis standalone CR.
// This function creates a standalone Redis instance (no HA, no sentinel).
// Returns an error if sentinel is enabled in the spec.
func ToRedisStandaloneVendorSpec(
	ctx context.Context,
	spec apiv2.WBRedisSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*redisv1beta2.Redis, error) {
	log := ctrl.LoggerFrom(ctx)

	if !spec.Enabled {
		return nil, nil
	}

	if spec.Sentinel.Enabled {
		return nil, nil
	}

	if spec.Sentinel.Enabled {
		return nil, fmt.Errorf("cannot create redis standalone with sentinel enabled")
	}

	specName := spec.Name

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	redis := &redisv1beta2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opstree.StandaloneName(specName),
			Namespace: spec.Namespace,
		},
		Spec: redisv1beta2.RedisSpec{
			KubernetesConfig: rediscommon.KubernetesConfig{
				Image:           common.RedisStandaloneImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources:       &corev1.ResourceRequirements{},
			},
			Storage: &rediscommon.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageQuantity,
							},
						},
					},
				},
			},
		},
	}

	// Add resources if specified
	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		redis.Spec.KubernetesConfig.Resources = &corev1.ResourceRequirements{
			Requests: spec.Config.Resources.Requests,
			Limits:   spec.Config.Resources.Limits,
		}
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, redis, scheme); err != nil {
		log.Error(err, "failed to set owner reference on Redis CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return redis, nil
}

// ToRedisSentinelVendorSpec converts a WBRedisSpec to a RedisSentinel CR.
// This function creates a Redis Sentinel for HA configuration.
// Returns an error if sentinel is not enabled in the spec.
func ToRedisSentinelVendorSpec(
	ctx context.Context,
	spec apiv2.WBRedisSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*redissentinelv1beta2.RedisSentinel, error) {
	log := ctrl.LoggerFrom(ctx)

	if !spec.Enabled {
		return nil, nil
	}

	if !spec.Sentinel.Enabled {
		return nil, nil
	}

	specName := spec.Name

	// Default sentinel count to 3 if not specified
	sentinelCount := int32(defaults.ReplicaSentinelCount)

	// Get master name from config or use default
	masterName := DefaultSentinelGroup
	if spec.Sentinel.Config.MasterName != "" {
		masterName = spec.Sentinel.Config.MasterName
	}

	sentinel := &redissentinelv1beta2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opstree.SentinelName(specName),
			Namespace: spec.Namespace,
		},
		Spec: redissentinelv1beta2.RedisSentinelSpec{
			Size: &sentinelCount,
			KubernetesConfig: rediscommon.KubernetesConfig{
				Image:           common.RedisSentinelImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources:       &corev1.ResourceRequirements{},
			},
			RedisSentinelConfig: &redissentinelv1beta2.RedisSentinelConfig{
				RedisSentinelConfig: rediscommon.RedisSentinelConfig{
					RedisReplicationName: opstree.ReplicationName(specName),
					MasterGroupName:      masterName,
				},
			},
		},
	}

	// Add resources if specified
	if len(spec.Sentinel.Config.Resources.Requests) > 0 || len(spec.Sentinel.Config.Resources.Limits) > 0 {
		sentinel.Spec.KubernetesConfig.Resources = &corev1.ResourceRequirements{
			Requests: spec.Sentinel.Config.Resources.Requests,
			Limits:   spec.Sentinel.Config.Resources.Limits,
		}
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, sentinel, scheme); err != nil {
		log.Error(err, "failed to set owner reference on RedisSentinel CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return sentinel, nil
}

// ToRedisReplicationVendorSpec converts a WBRedisSpec to a RedisReplication CR.
// This function creates a Redis replication setup for HA configuration.
// Returns an error if sentinel is not enabled in the spec.
func ToRedisReplicationVendorSpec(
	ctx context.Context,
	spec apiv2.WBRedisSpec,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*redisreplicationv1beta2.RedisReplication, error) {
	log := ctrl.LoggerFrom(ctx)

	if !spec.Enabled {
		return nil, nil
	}

	if !spec.Sentinel.Enabled {
		return nil, nil
	}

	specName := spec.Name

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	// Default replication count to 3 if not specified
	replicaCount := int32(defaults.ReplicaSentinelCount)

	replication := &redisreplicationv1beta2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opstree.ReplicationName(specName),
			Namespace: spec.Namespace,
		},
		Spec: redisreplicationv1beta2.RedisReplicationSpec{
			Size: &replicaCount,
			KubernetesConfig: rediscommon.KubernetesConfig{
				Image:           common.RedisReplicationImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources:       &corev1.ResourceRequirements{},
			},
			Storage: &rediscommon.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageQuantity,
							},
						},
					},
				},
			},
		},
	}

	// Add resources if specified
	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		replication.Spec.KubernetesConfig.Resources = &corev1.ResourceRequirements{
			Requests: spec.Config.Resources.Requests,
			Limits:   spec.Config.Resources.Limits,
		}
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, replication, scheme); err != nil {
		log.Error(err, "failed to set owner reference on RedisReplication CR")
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return replication, nil
}
