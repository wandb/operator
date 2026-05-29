package opstree

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	rediscommon "github.com/wandb/operator/pkg/vendored/redis-operator/common/v1beta2"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	RedisModuleName       = "redis"
	RedisStandaloneImage  = "quay.io/opstree/redis:v7.0.15"
	RedisReplicationImage = "quay.io/opstree/redis:v7.0.15"
	RedisSentinelImage    = "quay.io/opstree/redis-sentinel:v7.0.12"
)

const (
	DefaultSentinelGroup      = "gorilla"
	DefaultRedisExporterImage = "quay.io/opstree/redis-exporter:v1.44.0"
	DefaultRedisExporterPort  = 9121
)

const (
	redisRunAsUser  int64 = 1000
	redisRunAsGroup int64 = 1000
	redisFSGroup    int64 = 1000

	redisWritableTmpVolumeName = "redis-tmp"
	redisWritableTmpMountPath  = "/tmp"

	redisCapabilityAll corev1.Capability = "ALL"
)

func redisPodSecurityContext() *corev1.PodSecurityContext {
	if utils.IsOpenShift() {
		return &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.To(true),
			SeccompProfile: redisRuntimeDefaultSeccompProfile(),
		}
	}

	return &corev1.PodSecurityContext{
		RunAsUser:      ptr.To(redisRunAsUser),
		RunAsGroup:     ptr.To(redisRunAsGroup),
		RunAsNonRoot:   ptr.To(true),
		FSGroup:        ptr.To(redisFSGroup),
		SeccompProfile: redisRuntimeDefaultSeccompProfile(),
	}
}

func redisContainerSecurityContext() *corev1.SecurityContext {
	securityContext := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{redisCapabilityAll},
		},
		SeccompProfile: redisRuntimeDefaultSeccompProfile(),
	}
	if !utils.IsOpenShift() {
		securityContext.RunAsUser = ptr.To(redisRunAsUser)
		securityContext.RunAsGroup = ptr.To(redisRunAsGroup)
	}
	return securityContext
}

func redisWritableVolumeMount() rediscommon.AdditionalVolume {
	return rediscommon.AdditionalVolume{
		Volume: []corev1.Volume{
			{
				Name: redisWritableTmpVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		MountPath: []corev1.VolumeMount{
			{Name: redisWritableTmpVolumeName, MountPath: redisWritableTmpMountPath},
		},
	}
}

func redisAdditionalVolumePtr() *rediscommon.AdditionalVolume {
	volumeMount := redisWritableVolumeMount()
	return &volumeMount
}

func redisRuntimeDefaultSeccompProfile() *corev1.SeccompProfile {
	return &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
}

// createRedisExporterConfig creates a RedisExporter configuration if telemetry is enabled.
// Returns nil if telemetry is disabled.
func createRedisExporterConfig(telemetry apiv2.Telemetry) *rediscommon.RedisExporter {
	if !telemetry.Enabled {
		return nil
	}

	port := DefaultRedisExporterPort
	return &rediscommon.RedisExporter{
		Enabled:         true,
		Port:            &port,
		Image:           DefaultRedisExporterImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: redisContainerSecurityContext(),
	}
}

// ToRedisStandaloneVendorSpec converts a RedisSpec to a Redis standalone CR.
// This function creates a standalone Redis instance (no HA, no sentinel).
// Returns an error if sentinel is enabled in the spec.
func ToRedisStandaloneVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*redisv1beta2.Redis, error) {
	_, log := logx.WithSlog(ctx, logx.Redis)
	spec := wandb.Spec.Redis.ManagedRedis
	if spec == nil {
		return nil, nil
	}

	if spec.Sentinel.Enabled {
		return nil, nil
	}

	nsnBuilder := CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	redis := &redisv1beta2.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.StandaloneName(),
			Namespace: nsnBuilder.StandaloneNamespace(),
		},
		Spec: redisv1beta2.RedisSpec{
			KubernetesConfig: rediscommon.KubernetesConfig{
				Image:           RedisStandaloneImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources:       &corev1.ResourceRequirements{},
			},
			Affinity:           wandb.GetAffinity(spec.ManagedInfraSpec),
			PodSecurityContext: redisPodSecurityContext(),
			SecurityContext:    redisContainerSecurityContext(),
			Tolerations:        wandb.GetTolerations(spec.ManagedInfraSpec),
			Storage: &rediscommon.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: BuildWandbRedisLabels(wandb),
					},
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
				VolumeMount: redisWritableVolumeMount(),
			},
		},
	}

	if len(spec.Config.Resources.Requests) > 0 || len(spec.Config.Resources.Limits) > 0 {
		redis.Spec.KubernetesConfig.Resources = &corev1.ResourceRequirements{
			Requests: spec.Config.Resources.Requests,
			Limits:   spec.Config.Resources.Limits,
		}
	}

	if err := ctrl.SetControllerReference(wandb, redis, scheme); err != nil {
		log.Error("failed to set owner reference on Redis CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	redis.Spec.RedisExporter = createRedisExporterConfig(spec.Telemetry)

	return redis, nil
}

// ToRedisSentinelVendorSpec converts a RedisSpec to a RedisSentinel CR.
// This function creates a Redis Sentinel for HA configuration.
// Returns an error if sentinel is not enabled in the spec.
func ToRedisSentinelVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*redissentinelv1beta2.RedisSentinel, error) {
	_, log := logx.WithSlog(ctx, logx.Redis)
	spec := wandb.Spec.Redis.ManagedRedis
	if spec == nil {
		return nil, nil
	}

	if !spec.Sentinel.Enabled {
		return nil, nil
	}

	nsnBuilder := CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	// TODO I dont think we want to default this at all?
	sentinelCount := int32(3)

	// Get master name from config or use default
	masterName := DefaultSentinelGroup
	if spec.Sentinel.Config.MasterName != "" {
		masterName = spec.Sentinel.Config.MasterName
	}

	sentinel := &redissentinelv1beta2.RedisSentinel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.SentinelName(),
			Namespace: nsnBuilder.Namespace(),
		},
		Spec: redissentinelv1beta2.RedisSentinelSpec{
			Size: &sentinelCount,
			KubernetesConfig: rediscommon.KubernetesConfig{
				Image:           RedisSentinelImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources:       &corev1.ResourceRequirements{},
			},
			PodSecurityContext: redisPodSecurityContext(),
			SecurityContext:    redisContainerSecurityContext(),
			Affinity:           wandb.GetAffinity(spec.ManagedInfraSpec),
			Tolerations:        wandb.GetTolerations(spec.ManagedInfraSpec),
			VolumeMount:        redisAdditionalVolumePtr(),
			RedisSentinelConfig: &redissentinelv1beta2.RedisSentinelConfig{
				RedisSentinelConfig: rediscommon.RedisSentinelConfig{
					RedisReplicationName: nsnBuilder.ReplicationName(),
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
	if err := ctrl.SetControllerReference(wandb, sentinel, scheme); err != nil {
		log.Error("failed to set owner reference on RedisSentinel CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Add RedisExporter if telemetry is enabled
	sentinel.Spec.RedisExporter = createRedisExporterConfig(spec.Telemetry)

	return sentinel, nil
}

// ToRedisReplicationVendorSpec converts a RedisSpec to a RedisReplication CR.
// This function creates a Redis replication setup for HA configuration.
// Returns an error if sentinel is not enabled in the spec.
func ToRedisReplicationVendorSpec(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
	scheme *runtime.Scheme,
) (*redisreplicationv1beta2.RedisReplication, error) {
	_, log := logx.WithSlog(ctx, logx.Redis)
	spec := wandb.Spec.Redis.ManagedRedis
	if spec == nil {
		return nil, nil
	}

	if !spec.Sentinel.Enabled {
		return nil, nil
	}

	nsnBuilder := CreateNsNameBuilder(types.NamespacedName{
		Namespace: spec.Namespace, Name: spec.Name,
	})

	// Parse storage quantity
	storageQuantity, err := resource.ParseQuantity(spec.StorageSize)
	if err != nil {
		log.Error("Failed to parse storage size", "storageSize", spec.StorageSize, "error", err)
		return nil, fmt.Errorf("invalid storage size %q: %w", spec.StorageSize, err)
	}

	// TODO I dont think we want to default this at all?
	replicaCount := int32(3)

	replication := &redisreplicationv1beta2.RedisReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.ReplicationName(),
			Namespace: nsnBuilder.Namespace(),
		},
		Spec: redisreplicationv1beta2.RedisReplicationSpec{
			Size: &replicaCount,
			KubernetesConfig: rediscommon.KubernetesConfig{
				Image:           RedisReplicationImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Resources:       &corev1.ResourceRequirements{},
			},
			PodSecurityContext: redisPodSecurityContext(),
			SecurityContext:    redisContainerSecurityContext(),
			Affinity:           wandb.GetAffinity(spec.ManagedInfraSpec),
			Tolerations:        wandb.GetTolerations(spec.ManagedInfraSpec),
			Storage: &rediscommon.Storage{
				VolumeClaimTemplate: corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: BuildWandbRedisLabels(wandb),
					},
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
				VolumeMount: redisWritableVolumeMount(),
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
	if err := ctrl.SetControllerReference(wandb, replication, scheme); err != nil {
		log.Error("failed to set owner reference on RedisReplication CR", logx.ErrAttr(err))
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Add RedisExporter if telemetry is enabled
	replication.Spec.RedisExporter = createRedisExporterConfig(spec.Telemetry)

	return replication, nil
}

func BuildWandbRedisLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, RedisModuleName)
}

func ToRedisOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) common.OnDeleteRule {
	return common.ToOnDeleteRule(wandb, retentionPolicy, RedisModuleName)
}
