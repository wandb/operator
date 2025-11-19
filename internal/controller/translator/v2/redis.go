package v2

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/controller/translator/utils"
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

// BuildRedisConfig will create a new common.RedisConfig with defaultConfig applied if not
// present in actual. It should *never* be saved into the CR!
func BuildRedisConfig(actual apiv2.WBRedisSpec, defaultConfig common.RedisConfig) (common.RedisConfig, error) {
	redisConfig := TranslateRedisSpec(actual)

	if redisConfig.StorageSize.IsZero() {
		redisConfig.StorageSize = defaultConfig.StorageSize
	}
	redisConfig.Namespace = utils.Coalesce(redisConfig.Namespace, defaultConfig.Namespace)

	mergedResources := utils.Resources(
		corev1.ResourceRequirements{Requests: redisConfig.Requests, Limits: redisConfig.Limits},
		corev1.ResourceRequirements{Requests: defaultConfig.Requests, Limits: defaultConfig.Limits},
	)
	redisConfig.Requests = mergedResources.Requests
	redisConfig.Limits = mergedResources.Limits

	if actual.Sentinel == nil {
		redisConfig.Sentinel = defaultConfig.Sentinel
	} else {
		mergedSentinelResources := utils.Resources(
			corev1.ResourceRequirements{Requests: redisConfig.Sentinel.Requests, Limits: redisConfig.Sentinel.Limits},
			corev1.ResourceRequirements{Requests: defaultConfig.Sentinel.Requests, Limits: defaultConfig.Sentinel.Limits},
		)
		redisConfig.Sentinel.Requests = mergedSentinelResources.Requests
		redisConfig.Sentinel.Limits = mergedSentinelResources.Limits
		redisConfig.Sentinel.MasterGroupName = utils.Coalesce(redisConfig.Sentinel.MasterGroupName, defaultConfig.Sentinel.MasterGroupName)
		redisConfig.Sentinel.Enabled = actual.Sentinel.Enabled
	}

	redisConfig.Enabled = actual.Enabled

	return redisConfig, nil
}

func RedisSentinelEnabled(wbSpec apiv2.WBRedisSpec) bool {
	return wbSpec.Sentinel != nil && wbSpec.Sentinel.Enabled
}

func TranslateRedisSpec(spec apiv2.WBRedisSpec) common.RedisConfig {
	config := common.RedisConfig{
		Enabled:   spec.Enabled,
		Namespace: spec.Namespace,
	}

	if spec.StorageSize != "" {
		config.StorageSize = resource.MustParse(spec.StorageSize)
	}

	if spec.Config != nil {
		config.Requests = spec.Config.Resources.Requests
		config.Limits = spec.Config.Resources.Limits
	}

	if spec.Sentinel != nil {
		config.Sentinel.Enabled = spec.Sentinel.Enabled
		config.Sentinel.ReplicaCount = defaults.ReplicaSentinelCount
		if spec.Sentinel.Config != nil {
			config.Sentinel.MasterGroupName = spec.Sentinel.Config.MasterName
			config.Sentinel.Requests = spec.Sentinel.Config.Resources.Requests
			config.Sentinel.Limits = spec.Sentinel.Config.Resources.Limits
		}
	}

	return config
}

func ExtractRedisStatus(ctx context.Context, results *common.Results) apiv2.WBRedisStatus {
	return TranslateRedisStatus(
		ctx,
		common.ExtractRedisStatus(ctx, results),
	)
}

func TranslateRedisStatus(ctx context.Context, m common.RedisStatus) apiv2.WBRedisStatus {
	var result apiv2.WBRedisStatus
	var details []apiv2.WBStatusDetail

	for _, err := range m.Errors {
		details = append(details, apiv2.WBStatusDetail{
			State:   apiv2.WBStateError,
			Code:    err.Code(),
			Message: err.Reason(),
		})
	}

	for _, detail := range m.Details {
		state := translateRedisStatusCode(detail.Code())
		details = append(details, apiv2.WBStatusDetail{
			State:   state,
			Code:    detail.Code(),
			Message: detail.Message(),
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
	result.Details = details
	result.State = computeOverallState(details, m.Ready)
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

func (i *InfraConfigBuilder) AddRedisConfig(actual apiv2.WBRedisSpec) *InfraConfigBuilder {
	var err error
	var size common.Size
	var defaultConfig common.RedisConfig
	var mergedConfig common.RedisConfig

	size, err = ToModelSize(i.size)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	defaultConfig, err = defaults.BuildRedisDefaults(size, i.ownerNamespace)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}

	mergedConfig, err = BuildRedisConfig(actual, defaultConfig)
	if err != nil {
		i.errors = append(i.errors, err)
		return i
	}
	i.mergedRedis = mergedConfig
	return i
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

	if spec.Sentinel != nil && spec.Sentinel.Enabled {
		return nil, fmt.Errorf("cannot create redis standalone with sentinel enabled")
	}

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	redis := &redisv1beta2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RedisNamePrefix,
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
	if spec.Config != nil && (len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0) {
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

	if spec.Sentinel == nil || !spec.Sentinel.Enabled {
		return nil, fmt.Errorf("cannot create redis sentinel without sentinel enabled in spec")
	}

	// Default sentinel count to 3 if not specified
	sentinelCount := int32(defaults.ReplicaSentinelCount)

	// Get master name from config or use default
	masterName := DefaultSentinelGroup
	if spec.Sentinel.Config != nil && spec.Sentinel.Config.MasterName != "" {
		masterName = spec.Sentinel.Config.MasterName
	}

	sentinel := &redissentinelv1beta2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RedisNamePrefix,
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
					RedisReplicationName: common.RedisNamePrefix,
					MasterGroupName:      masterName,
				},
			},
		},
	}

	// Add resources if specified
	if spec.Sentinel.Config != nil && (len(spec.Sentinel.Config.Resources.Requests) > 0 || len(spec.Sentinel.Config.Resources.Limits) > 0) {
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

	if spec.Sentinel == nil || !spec.Sentinel.Enabled {
		return nil, fmt.Errorf("cannot create redis replication without sentinel enabled in spec")
	}

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	// Default replication count to 3 if not specified
	replicaCount := int32(defaults.ReplicaSentinelCount)

	replication := &redisreplicationv1beta2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RedisNamePrefix,
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
	if spec.Config != nil && (len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0) {
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
