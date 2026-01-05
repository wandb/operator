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

	"github.com/wandb/operator/internal/defaults"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	appsv2 "github.com/wandb/operator/api/v2"
)

// nolint:unused
// log is for logging in this package.
var weightsandbiaseslog = logf.Log.WithName("weightsandbiases-resource")

// SetupWeightsAndBiasesWebhookWithManager registers the webhook for WeightsAndBiases in the manager.
func SetupWeightsAndBiasesWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&appsv2.WeightsAndBiases{}).
		WithValidator(&WeightsAndBiasesCustomValidator{}).
		WithDefaulter(&WeightsAndBiasesCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-apps-wandb-com-v2-weightsandbiases,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps.wandb.com,resources=weightsandbiases,verbs=create;update,versions=v2,name=mweightsandbiases-v2.kb.io,admissionReviewVersions=v1

// WeightsAndBiasesCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind WeightsAndBiases when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type WeightsAndBiasesCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &WeightsAndBiasesCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind WeightsAndBiases.
func (d *WeightsAndBiasesCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	wandb, ok := obj.(*appsv2.WeightsAndBiases)

	if !ok {
		return fmt.Errorf("expected an WeightsAndBiases object but got %T", obj)
	}
	weightsandbiaseslog.Info("Defaulting for WeightsAndBiases", "name", wandb.GetName())

	if wandb.Spec.Size == "" {
		wandb.Spec.Size = appsv2.WBSizeDev
	}

	if wandb.Spec.Affinity == nil {
		wandb.Spec.Affinity = &corev1.Affinity{}
	}

	if wandb.Spec.Tolerations == nil {
		wandb.Spec.Tolerations = &[]corev1.Toleration{}
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

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-apps-wandb-com-v2-weightsandbiases,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.wandb.com,resources=weightsandbiases,verbs=create;update,versions=v2,name=vweightsandbiases-v2.kb.io,admissionReviewVersions=v1

// WeightsAndBiasesCustomValidator struct is responsible for validating the WeightsAndBiases resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type WeightsAndBiasesCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &WeightsAndBiasesCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type WeightsAndBiases.
func (v *WeightsAndBiasesCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	wandb, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	weightsandbiaseslog.Info("Validation for WeightsAndBiases upon creation", "name", wandb.GetName())

	return validateSpec(ctx, wandb)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type WeightsAndBiases.
func (v *WeightsAndBiasesCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newWandb, ok := newObj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object for the newObj but got %T", newObj)
	}
	oldWandb, ok := oldObj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object for the oldObj but got %T", oldObj)
	}
	weightsandbiaseslog.Info("Validation for WeightsAndBiases upon update", "name", newWandb.GetName())

	var specWarnings, changeWarnings admission.Warnings
	var err error

	weightsandbiaseslog.Info("validate V2 update", "name", newWandb.Name)

	if specWarnings, err = validateSpec(ctx, newWandb); err != nil {
		return specWarnings, err
	}
	changeWarnings, err = validateChanges(ctx, newWandb, oldWandb)
	return append(specWarnings, changeWarnings...), err
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type WeightsAndBiases.
func (v *WeightsAndBiasesCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	weightsandbiases, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	weightsandbiaseslog.Info("Validation for WeightsAndBiases upon deletion", "name", weightsandbiases.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

func toCommonSize(size appsv2.WBSize) (defaults.Size, error) {
	switch size {
	case appsv2.WBSizeDev:
		return defaults.SizeDev, nil
	case appsv2.WBSizeSmall:
		return defaults.SizeSmall, nil
	default:
		return "", fmt.Errorf("unsupported size: %s", size)
	}
}

func applyMySQLDefaults(wandb *appsv2.WeightsAndBiases, size defaults.Size) error {

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

func applyRedisDefaults(wandb *appsv2.WeightsAndBiases, size defaults.Size) error {
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

func applyKafkaDefaults(wandb *appsv2.WeightsAndBiases, size defaults.Size) error {
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

func applyMinioDefaults(wandb *appsv2.WeightsAndBiases, size defaults.Size) error {
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

	if wandb.Spec.Minio.Config.MinioBrowserSetting == "" {
		wandb.Spec.Minio.Config.MinioBrowserSetting = defaultConfig.MinioBrowserSetting
	}
	if wandb.Spec.Minio.Config.RootUser == "" {
		wandb.Spec.Minio.Config.RootUser = defaultConfig.RootUser
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

func applyClickHouseDefaults(wandb *appsv2.WeightsAndBiases, size defaults.Size) error {
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

func validateSpec(ctx context.Context, newWandb *appsv2.WeightsAndBiases) (admission.Warnings, error) {
	var allErrors field.ErrorList
	var warnings admission.Warnings

	allErrors = append(allErrors, validateRedisSpec(newWandb)...)

	if len(allErrors) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(
		schema.GroupKind{Group: "apps.wandb.com", Kind: "WeightsAndBiases"},
		newWandb.Name,
		allErrors,
	)
}

func validateChanges(ctx context.Context, newWandb *appsv2.WeightsAndBiases, oldWandb *appsv2.WeightsAndBiases) (admission.Warnings, error) {
	var allErrors field.ErrorList
	var warnings admission.Warnings

	allErrors = append(allErrors, validateRedisChanges(newWandb, oldWandb)...)

	if len(allErrors) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(
		schema.GroupKind{Group: "apps.wandb.com", Kind: "WeightsAndBiases"},
		newWandb.Name,
		allErrors,
	)
}

func validateRedisSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
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

func validateRedisChanges(newWandb, oldWandb *appsv2.WeightsAndBiases) field.ErrorList {
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
