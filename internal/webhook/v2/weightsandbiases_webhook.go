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
	"strings"

	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	appsv2 "github.com/wandb/operator/api/v2"
)

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
func (d *WeightsAndBiasesCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	_, log := logx.WithSlog(ctx, logx.DefaultingWebhook)
	wandb, ok := obj.(*appsv2.WeightsAndBiases)

	if !ok {
		return fmt.Errorf("expected an WeightsAndBiases object but got %T", obj)
	}
	log.Info("Defaulting for WeightsAndBiases", "name", wandb.GetName())

	if wandb.Spec.Size == "" {
		wandb.Spec.Size = appsv2.SizeDev
	}

	if wandb.Spec.RetentionPolicy.OnDelete == "" {
		wandb.Spec.RetentionPolicy.OnDelete = appsv2.DetachOnDelete
	}

	if wandb.Spec.Affinity == nil {
		wandb.Spec.Affinity = &corev1.Affinity{}
	}

	if wandb.Spec.Tolerations == nil {
		wandb.Spec.Tolerations = &[]corev1.Toleration{}
	}

	if wandb.Spec.Wandb.ManifestRepository == "" {
		wandb.Spec.Wandb.ManifestRepository = "oci://us-docker.pkg.dev/wandb-production/public/wandb/server-manifest"
	}

	if !strings.Contains(wandb.Spec.Wandb.ManifestRepository, "://") {
		// Prepend a default scheme (e.g., oci://) to ensure proper parsing of the host.
		wandb.Spec.Wandb.ManifestRepository = "oci://" + wandb.Spec.Wandb.ManifestRepository
	}

	if wandb.Spec.Wandb.InternalServiceAuth.Enabled == nil {
		wandb.Spec.Wandb.InternalServiceAuth.Enabled = pointer.Bool(true)
	}

	if wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer == "" {
		wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer = "https://kubernetes.default.svc.cluster.local"
	}

	if wandb.Spec.Wandb.ServiceAccount.Create == nil {
		wandb.Spec.Wandb.ServiceAccount.Create = pointer.Bool(true)
	}

	if wandb.Spec.Wandb.ServiceAccount.ServiceAccountName == "" {
		wandb.Spec.Wandb.ServiceAccount.ServiceAccountName = "wandb"
	}

	if wandb.Status.Wandb.Applications == nil {
		wandb.Status.Wandb.Applications = make(map[string]appsv2.ApplicationStatus)
	}

	applyMySQLDefaults(wandb)
	applyRedisDefaults(wandb)
	applyKafkaDefaults(wandb)
	applyMinioDefaults(wandb)
	applyMySQLDefaults(wandb)
	applyClickHouseDefaults(wandb)

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
	ctx, log := logx.WithSlog(ctx, logx.ValidatingWebhook)
	wandb, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	log.Info("Validation for WeightsAndBiases upon creation", "name", wandb.GetName())

	return validateSpec(ctx, wandb)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type WeightsAndBiases.
func (v *WeightsAndBiasesCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	ctx, log := logx.WithSlog(ctx, logx.ValidatingWebhook)
	newWandb, ok := newObj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object for the newObj but got %T", newObj)
	}
	oldWandb, ok := oldObj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object for the oldObj but got %T", oldObj)
	}
	log.Info("Validation for WeightsAndBiases upon update", "name", newWandb.GetName())

	var specWarnings, changeWarnings admission.Warnings
	var err error

	log.Info("validate V2 update", "name", newWandb.Name)

	if specWarnings, err = validateSpec(ctx, newWandb); err != nil {
		return specWarnings, err
	}
	changeWarnings, err = validateChanges(ctx, newWandb, oldWandb)
	return append(specWarnings, changeWarnings...), err
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type WeightsAndBiases.
func (v *WeightsAndBiasesCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ctx, log := logx.WithSlog(ctx, logx.ValidatingWebhook)
	weightsandbiases, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	log.Info("Validation for WeightsAndBiases upon deletion", "name", weightsandbiases.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

func applyMySQLDefaults(wandb *appsv2.WeightsAndBiases) {
	spec := wandb.Spec.MySQL.ManagedMysql
	if spec == nil {
		return
	}

	if spec.Name == "" {
		spec.Name = fmt.Sprintf("%s-mysql", wandb.Name)
	}

	if spec.Namespace == "" {
		spec.Namespace = wandb.Namespace
	}
	if spec.DeploymentType == "" {
		spec.DeploymentType = appsv2.MySQLTypeMysql
	}
}

func applyRedisDefaults(wandb *appsv2.WeightsAndBiases) {
	spec := wandb.Spec.Redis.ManagedRedis
	if spec == nil {
		return
	}

	if spec.Name == "" {
		spec.Name = fmt.Sprintf("%s-redis", wandb.Name)
	}

	if spec.Namespace == "" {
		spec.Namespace = wandb.Namespace
	}

	if wandb.Spec.Size != appsv2.SizeDev {
		spec.Sentinel.Enabled = true
	}
}

func applyKafkaDefaults(wandb *appsv2.WeightsAndBiases) {
	spec := wandb.Spec.Kafka.ManagedKafka
	if spec == nil {
		return
	}

	if spec.Name == "" {
		spec.Name = fmt.Sprintf("%s-kafka", wandb.Name)
	}

	if spec.Namespace == "" {
		spec.Namespace = wandb.Namespace
	}
}

func applyMinioDefaults(wandb *appsv2.WeightsAndBiases) {
	spec := wandb.Spec.Minio.ManagedMinio
	if spec == nil {
		return
	}

	if spec.Name == "" {
		spec.Name = fmt.Sprintf("%s-minio", wandb.Name)
	}

	if spec.Namespace == "" {
		spec.Namespace = wandb.Namespace
	}

	if spec.Config.RootUser == "" {
		spec.Config.RootUser = "admin"
	}

	if spec.Config.MinioBrowserSetting == "" {
		spec.Config.MinioBrowserSetting = "on"
	}
}

func applyClickHouseDefaults(wandb *appsv2.WeightsAndBiases) {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	if spec == nil {
		return
	}

	if spec.Name == "" {
		spec.Name = fmt.Sprintf("%s-clickhouse", wandb.Name)
	}

	if spec.Namespace == "" {
		spec.Namespace = wandb.Namespace
	}
}

func validateSpec(_ context.Context, newWandb *appsv2.WeightsAndBiases) (admission.Warnings, error) {
	var allErrors field.ErrorList
	var warnings admission.Warnings

	allErrors = append(allErrors, validateMySQLSpec(newWandb)...)
	allErrors = append(allErrors, validateRedisSpec(newWandb)...)
	allErrors = append(allErrors, validateKafkaSpec(newWandb)...)
	allErrors = append(allErrors, validateMinioSpec(newWandb)...)
	allErrors = append(allErrors, validateClickHouseSpec(newWandb)...)

	if len(allErrors) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(
		schema.GroupKind{Group: "apps.wandb.com", Kind: "WeightsAndBiases"},
		newWandb.Name,
		allErrors,
	)
}

func validateChanges(_ context.Context, newWandb *appsv2.WeightsAndBiases, oldWandb *appsv2.WeightsAndBiases) (admission.Warnings, error) {
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

func validateMySQLSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	mysqlPath := field.NewPath("spec").Child("mysql")

	if wandb.Spec.MySQL.ManagedMysql != nil && wandb.Spec.MySQL.ExternalMysql != nil {
		errors = append(errors, field.Invalid(
			mysqlPath,
			"",
			"managedMysql and externalMysql are mutually exclusive",
		))
	}

	return errors
}

func validateRedisSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	redisPath := field.NewPath("spec").Child("redis")

	if wandb.Spec.Redis.ManagedRedis != nil && wandb.Spec.Redis.ExternalRedis != nil {
		errors = append(errors, field.Invalid(
			redisPath,
			"",
			"managedRedis and externalRedis are mutually exclusive",
		))
	}

	spec := wandb.Spec.Redis.ManagedRedis
	if spec == nil {
		return errors
	}

	if spec.StorageSize != "" {
		if _, err := resource.ParseQuantity(spec.StorageSize); err != nil {
			errors = append(errors, field.Invalid(
				redisPath.Child("managedRedis").Child("storageSize"),
				spec.StorageSize,
				"must be a valid resource quantity (e.g., '10Gi')",
			))
		}
	}

	return errors
}

func validateKafkaSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	kafkaPath := field.NewPath("spec").Child("kafka")

	if wandb.Spec.Kafka.ManagedKafka != nil && wandb.Spec.Kafka.ExternalKafka != nil {
		errors = append(errors, field.Invalid(
			kafkaPath,
			"",
			"managedKafka and externalKafka are mutually exclusive",
		))
	}

	return errors
}

func validateMinioSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	minioPath := field.NewPath("spec").Child("minio")

	if wandb.Spec.Minio.ManagedMinio != nil && wandb.Spec.Minio.ExternalMinio != nil {
		errors = append(errors, field.Invalid(
			minioPath,
			"",
			"managedMinio and externalMinio are mutually exclusive",
		))
	}

	return errors
}

func validateClickHouseSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	chPath := field.NewPath("spec").Child("clickhouse")

	if wandb.Spec.ClickHouse.ManagedClickHouse != nil && wandb.Spec.ClickHouse.ExternalClickHouse != nil {
		errors = append(errors, field.Invalid(
			chPath,
			"",
			"managedClickhouse and externalClickhouse are mutually exclusive",
		))
	}

	return errors
}

func validateRedisChanges(newWandb, oldWandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	redisPath := field.NewPath("spec").Child("redis").Child("managedRedis")
	newSpec := newWandb.Spec.Redis.ManagedRedis
	oldSpec := oldWandb.Spec.Redis.ManagedRedis

	if newSpec == nil {
		return errors
	}

	if oldSpec != nil {
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
			errors = append(errors, field.Invalid(
				redisPath.Child("sentinel").Child("enabled"),
				newSpec.Sentinel.Enabled,
				"Redis Sentinel cannot be toggled between enabled and disabled (yet)",
			))
		}
	}

	return errors
}
