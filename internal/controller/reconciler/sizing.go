package reconciler

import (
	"github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	v3 "k8s.io/api/autoscaling/v2"
	"k8s.io/api/core/v1"
)

func ResolveResources(app manifest.Application, wandb *v2.WeightsAndBiases, containerResources *v1.ResourceRequirements) *v1.ResourceRequirements {
	var resources *v1.ResourceRequirements

	// check if there is "default" in the sizing map and apply those values
	if defaultConfig, ok := app.Sizing["default"]; ok && defaultConfig.Resources != nil {
		resources = mergeResources(resources, defaultConfig.Resources, wandb.Spec.RequireLimits)
	}

	// check if there is a sizing config in the map that corresponds to the size in the wandb spec and apply that
	if sizeConfig, ok := app.Sizing[wandb.Spec.Size]; ok && sizeConfig.Resources != nil {
		resources = mergeResources(resources, sizeConfig.Resources, wandb.Spec.RequireLimits)
	}

	// check if the container has a resource and if so apply those settings
	resources = mergeResources(resources, containerResources, wandb.Spec.RequireLimits)

	// Legacy override wins over sizing/container resources; limits stay gated by requireLimits.
	if lo, ok := wandb.Spec.Wandb.LegacyOverrides[app.Name]; ok && lo.Resources != nil {
		resources = mergeResources(resources, lo.Resources, wandb.Spec.RequireLimits)
	}

	if resources == nil {
		return nil
	}

	if len(resources.Limits) == 0 && len(resources.Requests) == 0 {
		return nil
	}

	return resources
}

func ResolveAutoscaling(app manifest.Application, wandb *v2.WeightsAndBiases) *v3.HorizontalPodAutoscalerSpec {
	hpa := &v3.HorizontalPodAutoscalerSpec{}

	// check if there is "default" in the sizing map and apply those values
	if defaultConfig, ok := app.Sizing["default"]; ok && defaultConfig.Autoscaling != nil {
		hpa.MinReplicas = defaultConfig.Autoscaling.Horizontal.MinReplicas
		hpa.MaxReplicas = defaultConfig.Autoscaling.Horizontal.MaxReplicas
		hpa.Metrics = defaultConfig.Autoscaling.Horizontal.Metrics
		hpa.Behavior = defaultConfig.Autoscaling.Horizontal.Behavior
		hpa.ScaleTargetRef = defaultConfig.Autoscaling.Horizontal.ScaleTargetRef
	}

	// check if there is a sizing config in the map that corresponds to the size in the wandb spec and apply that
	if sizeConfig, ok := app.Sizing[wandb.Spec.Size]; ok && sizeConfig.Autoscaling != nil {
		if sizeConfig.Autoscaling.Horizontal.MinReplicas != nil {
			hpa.MinReplicas = sizeConfig.Autoscaling.Horizontal.MinReplicas
		}
		if sizeConfig.Autoscaling.Horizontal.MaxReplicas != 0 {
			hpa.MaxReplicas = sizeConfig.Autoscaling.Horizontal.MaxReplicas
		}
		if len(sizeConfig.Autoscaling.Horizontal.Metrics) > 0 {
			hpa.Metrics = sizeConfig.Autoscaling.Horizontal.Metrics
		}
		if sizeConfig.Autoscaling.Horizontal.Behavior != nil {
			hpa.Behavior = sizeConfig.Autoscaling.Horizontal.Behavior
		}
		if sizeConfig.Autoscaling.Horizontal.ScaleTargetRef.Name != "" {
			hpa.ScaleTargetRef = sizeConfig.Autoscaling.Horizontal.ScaleTargetRef
		}
	}

	if hpa.MaxReplicas == 0 {
		return nil
	}

	return hpa
}

// mergeResources merges an overlay ResourceRequirements into a base, with
// overlay values taking precedence on a per-resource-name basis.
func mergeResources(base, overlay *v1.ResourceRequirements, requireLimits bool) *v1.ResourceRequirements {
	if base == nil && overlay == nil {
		return nil
	}
	result := &v1.ResourceRequirements{}
	if base != nil {
		if base.Limits != nil {
			result.Limits = make(v1.ResourceList)
			for k, v := range base.Limits {
				result.Limits[k] = v
			}
		}
		if base.Requests != nil {
			result.Requests = make(v1.ResourceList)
			for k, v := range base.Requests {
				result.Requests[k] = v
			}
		}
	}
	if overlay != nil {
		if overlay.Limits != nil {
			if result.Limits == nil {
				result.Limits = make(v1.ResourceList)
			}
			for k, v := range overlay.Limits {
				result.Limits[k] = v
			}
		}
		if overlay.Requests != nil {
			if result.Requests == nil {
				result.Requests = make(v1.ResourceList)
			}
			for k, v := range overlay.Requests {
				result.Requests[k] = v
			}
		}
	}

	if !requireLimits {
		result.Limits = nil
	}
	return result
}

// ResolveInfraSizing resolves a SizingConfig from an InfraConfig map for the
// given Size. It merges the "default" sizing with the size-specific sizing,
// where size-specific values override defaults.
func ResolveInfraSizing(sizing map[v2.Size]manifest.SizingConfig, size v2.Size, requireLimits bool) *manifest.SizingConfig {
	result := &manifest.SizingConfig{}

	// Apply "default" sizing baseline
	if defaultSizing, ok := sizing["default"]; ok {
		result.Replicas = defaultSizing.Replicas
		result.Shards = defaultSizing.Shards
		result.Copies = defaultSizing.Copies
		result.VolumeSize = defaultSizing.VolumeSize
		result.MetadataVolumeSize = defaultSizing.MetadataVolumeSize
		if defaultSizing.Resources != nil {
			result.Resources = defaultSizing.Resources.DeepCopy()
		}
	}

	// Override with size-specific sizing, merging resources
	if sizeSizing, ok := sizing[size]; ok {
		if sizeSizing.Replicas != 0 {
			result.Replicas = sizeSizing.Replicas
		}
		if sizeSizing.Shards != 0 {
			result.Shards = sizeSizing.Shards
		}
		if sizeSizing.Copies != 0 {
			result.Copies = sizeSizing.Copies
		}
		if sizeSizing.VolumeSize != "" {
			result.VolumeSize = sizeSizing.VolumeSize
		}
		if sizeSizing.MetadataVolumeSize != "" {
			result.MetadataVolumeSize = sizeSizing.MetadataVolumeSize
		}
		result.Resources = mergeResources(result.Resources, sizeSizing.Resources, requireLimits)
	}

	return result
}

// ResolveKafkaSizing resolves a SizingConfig from the KafkaConfig for the given Size.
func ResolveKafkaSizing(sizing map[v2.Size]manifest.KafkaSizingConfig, size v2.Size, requireLimits bool) *manifest.KafkaSizingConfig {
	result := &manifest.KafkaSizingConfig{}

	if defaultSizing, ok := sizing["default"]; ok {
		result.Replicas = defaultSizing.Replicas
		result.VolumeSize = defaultSizing.VolumeSize
		result.ReplicationFactor = defaultSizing.ReplicationFactor
		result.MinInSyncReplicas = defaultSizing.MinInSyncReplicas
		result.OffsetsTopicRF = defaultSizing.OffsetsTopicRF
		result.TransactionStateRF = defaultSizing.TransactionStateRF
		result.TransactionStateISR = defaultSizing.TransactionStateISR
		if defaultSizing.Resources != nil {
			result.Resources = defaultSizing.Resources.DeepCopy()
		}
	}

	if sizeSizing, ok := sizing[size]; ok {
		if sizeSizing.Replicas != 0 {
			result.Replicas = sizeSizing.Replicas
		}
		if sizeSizing.VolumeSize != "" {
			result.VolumeSize = sizeSizing.VolumeSize
		}
		if sizeSizing.ReplicationFactor != 0 {
			result.ReplicationFactor = sizeSizing.ReplicationFactor
		}
		if sizeSizing.MinInSyncReplicas != 0 {
			result.MinInSyncReplicas = sizeSizing.MinInSyncReplicas
		}
		if sizeSizing.OffsetsTopicRF != 0 {
			result.OffsetsTopicRF = sizeSizing.OffsetsTopicRF
		}
		if sizeSizing.TransactionStateRF != 0 {
			result.TransactionStateRF = sizeSizing.TransactionStateRF
		}
		if sizeSizing.TransactionStateISR != 0 {
			result.TransactionStateISR = sizeSizing.TransactionStateISR
		}
		result.Resources = mergeResources(result.Resources, sizeSizing.Resources, requireLimits)
	}

	return result
}

// ApplyInfraSizing applies manifest-derived sizing to the wandb spec's infra
// components. Values from the manifest are only applied when the corresponding
// spec field has not been explicitly set by the user (i.e., is zero-valued).
// infraSizingConfig returns the manifest sizing config for an instance key,
// falling back to the manifest "default" config when the key has no entry.
func infraSizingConfig[T any](m map[string]T, key string) (T, bool) {
	if cfg, ok := m[key]; ok {
		return cfg, true
	}
	cfg, ok := m[v2.DefaultInstanceName]
	return cfg, ok
}

func ApplyInfraSizing(wandb *v2.WeightsAndBiases, manifest manifest.Manifest) {
	size := wandb.Spec.Size

	// MySQL: size each managed instance, preferring a manifest sizing config
	// matching the instance key and falling back to the manifest "default".
	for key, instance := range wandb.Spec.MySQL {
		spec := instance.ManagedMysql
		if spec == nil {
			continue
		}
		mysqlConfig, ok := infraSizingConfig(manifest.Mysql, key)
		if !ok {
			continue
		}
		sizing := ResolveInfraSizing(mysqlConfig.Sizing, size, wandb.Spec.RequireLimits)
		if spec.Replicas == 0 && sizing.Replicas != 0 {
			spec.Replicas = sizing.Replicas
		}
		if spec.StorageSize == "" && sizing.VolumeSize != "" {
			spec.StorageSize = sizing.VolumeSize
		}
		if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
			spec.Config.Resources = *sizing.Resources
		}
	}

	// Redis
	for key, instance := range wandb.Spec.Redis {
		spec := instance.ManagedRedis
		if spec == nil {
			continue
		}
		redisConfig, ok := infraSizingConfig(manifest.Redis, key)
		if !ok {
			continue
		}
		sizing := ResolveInfraSizing(redisConfig.Sizing, size, wandb.Spec.RequireLimits)
		if spec.StorageSize == "" && sizing.VolumeSize != "" {
			spec.StorageSize = sizing.VolumeSize
		}
		if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
			spec.Config.Resources = *sizing.Resources
		}
	}

	// ClickHouse
	for key, instance := range wandb.Spec.ClickHouse {
		spec := instance.ManagedClickHouse
		if spec == nil {
			continue
		}

		// Keeper sizing comes from the manifest's clickhouseKeeper block
		// (independent of the clickhouse block); CR values are treated as user
		// overrides.
		if keeperConfig, ok := infraSizingConfig(manifest.ClickhouseKeeper, key); ok {
			keeperSizing := ResolveInfraSizing(keeperConfig.Sizing, size, wandb.Spec.RequireLimits)
			if spec.Keeper.Replicas == 0 && keeperSizing.Replicas != 0 {
				spec.Keeper.Replicas = keeperSizing.Replicas
			}
			if spec.Keeper.StorageSize == "" && keeperSizing.VolumeSize != "" {
				spec.Keeper.StorageSize = keeperSizing.VolumeSize
			}
			if keeperSizing.Resources != nil && len(spec.Keeper.Config.Resources.Requests) == 0 && len(spec.Keeper.Config.Resources.Limits) == 0 {
				spec.Keeper.Config.Resources = *keeperSizing.Resources
			}
		}

		clickhouseConfig, ok := infraSizingConfig(manifest.Clickhouse, key)
		if !ok {
			continue
		}
		sizing := ResolveInfraSizing(clickhouseConfig.Sizing, size, wandb.Spec.RequireLimits)
		if spec.Replicas == 0 && sizing.Replicas != 0 {
			spec.Replicas = sizing.Replicas
		}
		if spec.StorageSize == "" && sizing.VolumeSize != "" {
			spec.StorageSize = sizing.VolumeSize
		}
		if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
			spec.Config.Resources = *sizing.Resources
		}
	}

	// ObjectStore (bucket)
	for key, instance := range wandb.Spec.ObjectStore {
		spec := instance.ManagedObjectStore
		if spec == nil {
			continue
		}
		objectStoreConfig, ok := infraSizingConfig(manifest.Bucket, key)
		if !ok {
			continue
		}
		sizing := ResolveInfraSizing(objectStoreConfig.Sizing, size, wandb.Spec.RequireLimits)
		if spec.Replicas == 0 && sizing.Replicas != 0 {
			spec.Replicas = sizing.Replicas
		}
		if spec.Copies == 0 && sizing.Copies != 0 {
			spec.Copies = sizing.Copies
		}
		if spec.StorageSize == "" && sizing.VolumeSize != "" {
			spec.StorageSize = sizing.VolumeSize
		}
		// Neutral manifest value maps to the SeaweedFS-specific filer disk; CR override wins.
		if spec.SeaweedObjectStoreSpec.FilerStorageSize == "" && sizing.MetadataVolumeSize != "" {
			spec.SeaweedObjectStoreSpec.FilerStorageSize = sizing.MetadataVolumeSize
		}
		if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
			spec.Config.Resources = *sizing.Resources
		}
	}

	// Kafka
	if wandb.Spec.Kafka.ManagedKafka != nil {
		if sizing := ResolveKafkaSizing(manifest.Kafka.Sizing, size, wandb.Spec.RequireLimits); sizing != nil {
			spec := wandb.Spec.Kafka.ManagedKafka
			if spec.Replicas == 0 && sizing.Replicas != 0 {
				spec.Replicas = sizing.Replicas
			}
			if spec.StorageSize == "" && sizing.VolumeSize != "" {
				spec.StorageSize = sizing.VolumeSize
			}
			if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
				spec.Config.Resources = *sizing.Resources
			}
			if spec.Config.ReplicationConfig.DefaultReplicationFactor == 0 && sizing.ReplicationFactor != 0 {
				spec.Config.ReplicationConfig.DefaultReplicationFactor = sizing.ReplicationFactor
			}
			if spec.Config.ReplicationConfig.MinInSyncReplicas == 0 && sizing.MinInSyncReplicas != 0 {
				spec.Config.ReplicationConfig.MinInSyncReplicas = sizing.MinInSyncReplicas
			}
			if spec.Config.ReplicationConfig.OffsetsTopicRF == 0 && sizing.OffsetsTopicRF != 0 {
				spec.Config.ReplicationConfig.OffsetsTopicRF = sizing.OffsetsTopicRF
			}
			if spec.Config.ReplicationConfig.TransactionStateRF == 0 && sizing.TransactionStateRF != 0 {
				spec.Config.ReplicationConfig.TransactionStateRF = sizing.TransactionStateRF
			}
			if spec.Config.ReplicationConfig.TransactionStateISR == 0 && sizing.TransactionStateISR != 0 {
				spec.Config.ReplicationConfig.TransactionStateISR = sizing.TransactionStateISR
			}
		}
	}
}
