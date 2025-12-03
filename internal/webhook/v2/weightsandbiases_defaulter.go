/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/defaults"
)

var defLog = ctrl.Log.WithName("wandb-v2-defaulter")

func Default(ctx context.Context, wandb *apiv2.WeightsAndBiases) error {
	defLog.Info("applying defaults", "name", wandb.Name)

	if wandb.Spec.Size == "" {
		wandb.Spec.Size = apiv2.WBSizeDev
	}

	size, err := toCommonSize(wandb.Spec.Size)
	if err != nil {
		return err
	}

	if err := applyMySQLDefaults(wandb, size); err != nil {
		return fmt.Errorf("failed to apply MySQL defaults: %w", err)
	}

	if err := applyRedisDefaults(wandb, size); err != nil {
		return fmt.Errorf("failed to apply Redis defaults: %w", err)
	}

	if err := applyKafkaDefaults(wandb, size); err != nil {
		return fmt.Errorf("failed to apply Kafka defaults: %w", err)
	}

	if err := applyMinioDefaults(wandb, size); err != nil {
		return fmt.Errorf("failed to apply Minio defaults: %w", err)
	}

	if err := applyClickHouseDefaults(wandb, size); err != nil {
		return fmt.Errorf("failed to apply ClickHouse defaults: %w", err)
	}

	return nil
}

func toCommonSize(size apiv2.WBSize) (defaults.Size, error) {
	switch size {
	case apiv2.WBSizeDev:
		return defaults.SizeDev, nil
	case apiv2.WBSizeSmall:
		return defaults.SizeSmall, nil
	default:
		return "", fmt.Errorf("unsupported size: %s", size)
	}
}

func applyMySQLDefaults(wandb *apiv2.WeightsAndBiases, size defaults.Size) error {

	defaultConfig, err := defaults.BuildMySQLDefaults(size, wandb.Namespace)
	if err != nil {
		return err
	}

	if wandb.Spec.MySQL.Namespace == "" {
		wandb.Spec.MySQL.Namespace = defaultConfig.Namespace
	}
	if wandb.Spec.MySQL.Name == "" {
		wandb.Spec.MySQL.Name = defaultConfig.Name
	}

	if wandb.Spec.MySQL.StorageSize == "" {
		wandb.Spec.MySQL.StorageSize = defaultConfig.StorageSize
	}

	if wandb.Spec.MySQL.Replicas == 0 {
		wandb.Spec.MySQL.Replicas = defaultConfig.Replicas
	}

	if wandb.Spec.MySQL.Config.Resources.Requests == nil {
		wandb.Spec.MySQL.Config.Resources.Requests = corev1.ResourceList{}
	}
	if wandb.Spec.MySQL.Config.Resources.Limits == nil {
		wandb.Spec.MySQL.Config.Resources.Limits = corev1.ResourceList{}
	}

	for k, v := range defaultConfig.Resources.Requests {
		if _, exists := wandb.Spec.MySQL.Config.Resources.Requests[k]; !exists {
			wandb.Spec.MySQL.Config.Resources.Requests[k] = v
		}
	}
	for k, v := range defaultConfig.Resources.Limits {
		if _, exists := wandb.Spec.MySQL.Config.Resources.Limits[k]; !exists {
			wandb.Spec.MySQL.Config.Resources.Limits[k] = v
		}
	}

	return nil
}

func applyRedisDefaults(wandb *apiv2.WeightsAndBiases, size defaults.Size) error {
	defaultConfig, err := defaults.BuildRedisDefaults(size, wandb.Namespace)
	if err != nil {
		return err
	}

	if wandb.Spec.Redis.Namespace == "" {
		wandb.Spec.Redis.Namespace = defaultConfig.Namespace
	}
	if wandb.Spec.Redis.Name == "" {
		wandb.Spec.Redis.Name = defaultConfig.Name
	}

	if wandb.Spec.Redis.StorageSize == "" {
		wandb.Spec.Redis.StorageSize = defaultConfig.StorageSize.String()
	}

	if wandb.Spec.Redis.Config.Resources.Requests == nil {
		wandb.Spec.Redis.Config.Resources.Requests = corev1.ResourceList{}
	}
	if wandb.Spec.Redis.Config.Resources.Limits == nil {
		wandb.Spec.Redis.Config.Resources.Limits = corev1.ResourceList{}
	}

	for k, v := range defaultConfig.Requests {
		if _, exists := wandb.Spec.Redis.Config.Resources.Requests[k]; !exists {
			wandb.Spec.Redis.Config.Resources.Requests[k] = v
		}
	}
	for k, v := range defaultConfig.Limits {
		if _, exists := wandb.Spec.Redis.Config.Resources.Limits[k]; !exists {
			wandb.Spec.Redis.Config.Resources.Limits[k] = v
		}
	}

	// Always use the default Sentinel.Enabled, based on Size
	wandb.Spec.Redis.Sentinel.Enabled = defaultConfig.Sentinel.Enabled
	if wandb.Spec.Redis.Sentinel.Config.MasterName == "" {
		wandb.Spec.Redis.Sentinel.Config.MasterName = defaultConfig.Sentinel.MasterGroupName
	}

	if wandb.Spec.Redis.Sentinel.Config.Resources.Requests == nil {
		wandb.Spec.Redis.Sentinel.Config.Resources.Requests = corev1.ResourceList{}
	}
	if wandb.Spec.Redis.Sentinel.Config.Resources.Limits == nil {
		wandb.Spec.Redis.Sentinel.Config.Resources.Limits = corev1.ResourceList{}
	}

	for k, v := range defaultConfig.Sentinel.Requests {
		if _, exists := wandb.Spec.Redis.Sentinel.Config.Resources.Requests[k]; !exists {
			wandb.Spec.Redis.Sentinel.Config.Resources.Requests[k] = v
		}
	}
	for k, v := range defaultConfig.Sentinel.Limits {
		if _, exists := wandb.Spec.Redis.Sentinel.Config.Resources.Limits[k]; !exists {
			wandb.Spec.Redis.Sentinel.Config.Resources.Limits[k] = v
		}
	}

	return nil
}

func applyKafkaDefaults(wandb *apiv2.WeightsAndBiases, size defaults.Size) error {
	defaultConfig, err := defaults.BuildKafkaDefaults(size, wandb.Namespace)
	if err != nil {
		return err
	}

	if wandb.Spec.Kafka.Namespace == "" {
		wandb.Spec.Kafka.Namespace = defaultConfig.Namespace
	}

	if wandb.Spec.Kafka.Name == "" {
		wandb.Spec.Kafka.Name = defaultConfig.Name
	}

	if wandb.Spec.Kafka.StorageSize == "" {
		wandb.Spec.Kafka.StorageSize = defaultConfig.StorageSize
	}

	if wandb.Spec.Kafka.Replicas == 0 {
		wandb.Spec.Kafka.Replicas = defaultConfig.Replicas
	}

	if wandb.Spec.Kafka.Config.Resources.Requests == nil {
		wandb.Spec.Kafka.Config.Resources.Requests = corev1.ResourceList{}
	}
	if wandb.Spec.Kafka.Config.Resources.Limits == nil {
		wandb.Spec.Kafka.Config.Resources.Limits = corev1.ResourceList{}
	}

	for k, v := range defaultConfig.Resources.Requests {
		if _, exists := wandb.Spec.Kafka.Config.Resources.Requests[k]; !exists {
			wandb.Spec.Kafka.Config.Resources.Requests[k] = v
		}
	}
	for k, v := range defaultConfig.Resources.Limits {
		if _, exists := wandb.Spec.Kafka.Config.Resources.Limits[k]; !exists {
			wandb.Spec.Kafka.Config.Resources.Limits[k] = v
		}
	}

	if wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor == 0 {
		wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor = defaultConfig.ReplicationConfig.DefaultReplicationFactor
	}
	if wandb.Spec.Kafka.Config.ReplicationConfig.MinInSyncReplicas == 0 {
		wandb.Spec.Kafka.Config.ReplicationConfig.MinInSyncReplicas = defaultConfig.ReplicationConfig.MinInSyncReplicas
	}
	if wandb.Spec.Kafka.Config.ReplicationConfig.OffsetsTopicRF == 0 {
		wandb.Spec.Kafka.Config.ReplicationConfig.OffsetsTopicRF = defaultConfig.ReplicationConfig.OffsetsTopicRF
	}
	if wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateRF == 0 {
		wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateRF = defaultConfig.ReplicationConfig.TransactionStateRF
	}
	if wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateISR == 0 {
		wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateISR = defaultConfig.ReplicationConfig.TransactionStateISR
	}

	return nil
}

func applyMinioDefaults(wandb *apiv2.WeightsAndBiases, size defaults.Size) error {
	defaultConfig, err := defaults.BuildMinioDefaults(size, wandb.Namespace)
	if err != nil {
		return err
	}

	if wandb.Spec.Minio.Namespace == "" {
		wandb.Spec.Minio.Namespace = defaultConfig.Namespace
	}

	if wandb.Spec.Minio.Name == "" {
		wandb.Spec.Minio.Name = defaultConfig.Name
	}

	if wandb.Spec.Minio.StorageSize == "" {
		wandb.Spec.Minio.StorageSize = defaultConfig.StorageSize
	}

	if wandb.Spec.Minio.Replicas == 0 {
		wandb.Spec.Minio.Replicas = defaultConfig.Servers
	}

	if wandb.Spec.Minio.Config.Resources.Requests == nil {
		wandb.Spec.Minio.Config.Resources.Requests = corev1.ResourceList{}
	}
	if wandb.Spec.Minio.Config.Resources.Limits == nil {
		wandb.Spec.Minio.Config.Resources.Limits = corev1.ResourceList{}
	}

	for k, v := range defaultConfig.Resources.Requests {
		if _, exists := wandb.Spec.Minio.Config.Resources.Requests[k]; !exists {
			wandb.Spec.Minio.Config.Resources.Requests[k] = v
		}
	}
	for k, v := range defaultConfig.Resources.Limits {
		if _, exists := wandb.Spec.Minio.Config.Resources.Limits[k]; !exists {
			wandb.Spec.Minio.Config.Resources.Limits[k] = v
		}
	}

	return nil
}

func applyClickHouseDefaults(wandb *apiv2.WeightsAndBiases, size defaults.Size) error {
	defaultConfig, err := defaults.BuildClickHouseDefaults(size, wandb.Namespace)
	if err != nil {
		return err
	}

	if wandb.Spec.ClickHouse.Namespace == "" {
		wandb.Spec.ClickHouse.Namespace = defaultConfig.Namespace
	}

	if wandb.Spec.ClickHouse.Name == "" {
		wandb.Spec.ClickHouse.Name = defaultConfig.Name
	}

	if wandb.Spec.ClickHouse.StorageSize == "" {
		wandb.Spec.ClickHouse.StorageSize = defaultConfig.StorageSize
	}

	if wandb.Spec.ClickHouse.Replicas == 0 {
		wandb.Spec.ClickHouse.Replicas = defaultConfig.Replicas
	}

	if wandb.Spec.ClickHouse.Version == "" {
		wandb.Spec.ClickHouse.Version = defaultConfig.Version
	}

	if wandb.Spec.ClickHouse.Config.Resources.Requests == nil {
		wandb.Spec.ClickHouse.Config.Resources.Requests = corev1.ResourceList{}
	}
	if wandb.Spec.ClickHouse.Config.Resources.Limits == nil {
		wandb.Spec.ClickHouse.Config.Resources.Limits = corev1.ResourceList{}
	}

	for k, v := range defaultConfig.Resources.Requests {
		if _, exists := wandb.Spec.ClickHouse.Config.Resources.Requests[k]; !exists {
			wandb.Spec.ClickHouse.Config.Resources.Requests[k] = v
		}
	}
	for k, v := range defaultConfig.Resources.Limits {
		if _, exists := wandb.Spec.ClickHouse.Config.Resources.Limits[k]; !exists {
			wandb.Spec.ClickHouse.Config.Resources.Limits[k] = v
		}
	}

	return nil
}
