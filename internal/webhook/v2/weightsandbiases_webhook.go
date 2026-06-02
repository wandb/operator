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
	"k8s.io/utils/ptr"
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

	if wandb.Spec.Wandb.Features == nil {
		wandb.Spec.Wandb.Features = make(map[string]bool)
	}

	if wandb.Spec.Wandb.ManifestRepository == "" {
		wandb.Spec.Wandb.ManifestRepository = "oci://us-docker.pkg.dev/wandb-production/public/wandb/server-manifest"
	}

	if !strings.Contains(wandb.Spec.Wandb.ManifestRepository, "://") {
		// Prepend a default scheme (e.g., oci://) to ensure proper parsing of the host.
		wandb.Spec.Wandb.ManifestRepository = "oci://" + wandb.Spec.Wandb.ManifestRepository
	}

	if wandb.Spec.Wandb.InternalServiceAuth.Enabled == nil {
		wandb.Spec.Wandb.InternalServiceAuth.Enabled = ptr.To(true)
	}

	if wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer == "" {
		wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer = "https://kubernetes.default.svc.cluster.local"
	}

	if wandb.Spec.Wandb.ServiceAccount.Create == nil {
		wandb.Spec.Wandb.ServiceAccount.Create = ptr.To(true)
	}

	if wandb.Spec.Wandb.ServiceAccount.ServiceAccountName == "" {
		wandb.Spec.Wandb.ServiceAccount.ServiceAccountName = "wandb-app"
	}

	if wandb.Status.Wandb.Applications == nil {
		wandb.Status.Wandb.Applications = make(map[string]appsv2.ApplicationStatus)
	}

	applyMySQLDefaults(wandb)
	applyRedisDefaults(wandb)
	applyKafkaDefaults(wandb)
	applyObjectStoreDefaults(wandb)
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
	_, log := logx.WithSlog(ctx, logx.ValidatingWebhook)
	weightsandbiases, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	log.Info("Validation for WeightsAndBiases upon deletion", "name", weightsandbiases.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}

// instanceResourceName builds the default name for a managed infra instance.
// The reserved default instance keeps the historical "<cr>-<suffix>" name for
// backward compatibility; other instances are suffixed with their key.
func instanceResourceName(wandb *appsv2.WeightsAndBiases, suffix, key string) string {
	if key == appsv2.DefaultInstanceName {
		return fmt.Sprintf("%s-%s", wandb.Name, suffix)
	}
	return fmt.Sprintf("%s-%s-%s", wandb.Name, suffix, key)
}

func applyMySQLDefaults(wandb *appsv2.WeightsAndBiases) {
	if wandb.Spec.MySQL == nil {
		wandb.Spec.MySQL = map[string]appsv2.MySQLSpec{}
	}
	if len(wandb.Spec.MySQL) == 0 {
		wandb.Spec.MySQL[appsv2.DefaultInstanceName] = appsv2.MySQLSpec{ManagedMysql: &appsv2.ManagedMysqlSpec{}}
	}

	for key, spec := range wandb.Spec.MySQL {
		if spec.ExternalMysql != nil {
			continue
		}
		if spec.ManagedMysql == nil {
			spec.ManagedMysql = &appsv2.ManagedMysqlSpec{}
		}
		if spec.ManagedMysql.Name == "" {
			spec.ManagedMysql.Name = instanceResourceName(wandb, "mysql", key)
		}
		if spec.ManagedMysql.Namespace == "" {
			spec.ManagedMysql.Namespace = wandb.Namespace
		}
		wandb.Spec.MySQL[key] = spec
	}
}

func applyRedisDefaults(wandb *appsv2.WeightsAndBiases) {
	if wandb.Spec.Redis == nil {
		wandb.Spec.Redis = map[string]appsv2.RedisSpec{}
	}
	if len(wandb.Spec.Redis) == 0 {
		wandb.Spec.Redis[appsv2.DefaultInstanceName] = appsv2.RedisSpec{ManagedRedis: &appsv2.ManagedRedisSpec{}}
	}

	for key, spec := range wandb.Spec.Redis {
		if spec.ExternalRedis != nil {
			continue
		}
		if spec.ManagedRedis == nil {
			spec.ManagedRedis = &appsv2.ManagedRedisSpec{}
		}
		if spec.ManagedRedis.Name == "" {
			spec.ManagedRedis.Name = instanceResourceName(wandb, "redis", key)
		}
		if spec.ManagedRedis.Namespace == "" {
			spec.ManagedRedis.Namespace = wandb.Namespace
		}
		if wandb.Spec.Size != appsv2.SizeDev {
			spec.ManagedRedis.Sentinel.Enabled = true
		}
		wandb.Spec.Redis[key] = spec
	}
}

func applyKafkaDefaults(wandb *appsv2.WeightsAndBiases) {
	if wandb.Spec.Kafka.ManagedKafka == nil {
		if wandb.Spec.Kafka.ExternalKafka != nil {
			return
		}
		wandb.Spec.Kafka.ManagedKafka = &appsv2.ManagedKafkaSpec{}
	}

	spec := wandb.Spec.Kafka.ManagedKafka

	if spec.Name == "" {
		spec.Name = fmt.Sprintf("%s-kafka", wandb.Name)
	}

	if spec.Namespace == "" {
		spec.Namespace = wandb.Namespace
	}
}

func applyObjectStoreDefaults(wandb *appsv2.WeightsAndBiases) {
	if wandb.Spec.ObjectStore == nil {
		wandb.Spec.ObjectStore = map[string]appsv2.ObjectStoreSpec{}
	}
	if len(wandb.Spec.ObjectStore) == 0 {
		wandb.Spec.ObjectStore[appsv2.DefaultInstanceName] = appsv2.ObjectStoreSpec{ManagedObjectStore: &appsv2.ManagedObjectStoreSpec{}}
	}

	for key, spec := range wandb.Spec.ObjectStore {
		if spec.ExternalObjectStore != nil {
			continue
		}
		if spec.ManagedObjectStore == nil {
			spec.ManagedObjectStore = &appsv2.ManagedObjectStoreSpec{}
		}
		managed := spec.ManagedObjectStore
		if managed.Name == "" {
			managed.Name = instanceResourceName(wandb, "seaweedfs", key)
		}
		if managed.Namespace == "" {
			managed.Namespace = wandb.Namespace
		}
		if managed.Config.AccessKey == "" && managed.Config.RootUser != "" { //nolint:staticcheck
			managed.Config.AccessKey = managed.Config.RootUser //nolint:staticcheck
		}
		if managed.Config.AccessKey == "" {
			managed.Config.AccessKey = "admin"
		}
		wandb.Spec.ObjectStore[key] = spec
	}
}

func applyClickHouseDefaults(wandb *appsv2.WeightsAndBiases) {
	if wandb.Spec.ClickHouse == nil {
		wandb.Spec.ClickHouse = map[string]appsv2.ClickHouseSpec{}
	}
	if len(wandb.Spec.ClickHouse) == 0 {
		wandb.Spec.ClickHouse[appsv2.DefaultInstanceName] = appsv2.ClickHouseSpec{ManagedClickHouse: &appsv2.ManagedClickHouseSpec{}}
	}

	for key, spec := range wandb.Spec.ClickHouse {
		if spec.ExternalClickHouse != nil {
			continue
		}
		if spec.ManagedClickHouse == nil {
			spec.ManagedClickHouse = &appsv2.ManagedClickHouseSpec{}
		}
		if spec.ManagedClickHouse.Name == "" {
			spec.ManagedClickHouse.Name = instanceResourceName(wandb, "clickhouse", key)
		}
		if spec.ManagedClickHouse.Namespace == "" {
			spec.ManagedClickHouse.Namespace = wandb.Namespace
		}
		wandb.Spec.ClickHouse[key] = spec
	}
}

func validateSpec(_ context.Context, newWandb *appsv2.WeightsAndBiases) (admission.Warnings, error) {
	var allErrors field.ErrorList
	var warnings admission.Warnings

	allErrors = append(allErrors, validateMySQLSpec(newWandb)...)
	allErrors = append(allErrors, validateRedisSpec(newWandb)...)
	allErrors = append(allErrors, validateKafkaSpec(newWandb)...)
	allErrors = append(allErrors, validateObjectStoreSpec(newWandb)...)
	allErrors = append(allErrors, validateClickHouseSpec(newWandb)...)
	networkingErrors, networkingWarnings := validateNetworkingSpec(newWandb)
	allErrors = append(allErrors, networkingErrors...)
	warnings = append(warnings, networkingWarnings...)

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

// validateHasDefaultInstance reports an error when a multi-instance infra type
// defines at least one instance but is missing the reserved default key, which
// the env-var fallback relies on.
func validateHasDefaultInstance[T any](m map[string]T, path *field.Path) field.ErrorList {
	if len(m) == 0 {
		return nil
	}
	if _, ok := m[appsv2.DefaultInstanceName]; ok {
		return nil
	}
	return field.ErrorList{field.Required(
		path.Key(appsv2.DefaultInstanceName),
		fmt.Sprintf("a %q instance is required when other instances are defined", appsv2.DefaultInstanceName),
	)}
}

func validateMySQLSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	mysqlPath := field.NewPath("spec").Child("mysql")

	errors = append(errors, validateHasDefaultInstance(wandb.Spec.MySQL, mysqlPath)...)

	for key, spec := range wandb.Spec.MySQL {
		instancePath := mysqlPath.Key(key)
		if spec.ManagedMysql != nil && spec.ExternalMysql != nil {
			errors = append(errors, field.Invalid(
				instancePath,
				"",
				"managedMysql and externalMysql are mutually exclusive",
			))
		}
		if managed := spec.ManagedMysql; managed != nil {
			if managed.Replicas != 0 && managed.Replicas%2 == 0 {
				errors = append(errors, field.Invalid(
					instancePath.Child("managedMysql").Child("replicas"),
					managed.Replicas,
					"replicas must be an odd number (Moco enforces quorum-based replication)",
				))
			}
		}
	}

	return errors
}

func validateRedisSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	redisPath := field.NewPath("spec").Child("redis")

	errors = append(errors, validateHasDefaultInstance(wandb.Spec.Redis, redisPath)...)

	for key, spec := range wandb.Spec.Redis {
		instancePath := redisPath.Key(key)
		if spec.ManagedRedis != nil && spec.ExternalRedis != nil {
			errors = append(errors, field.Invalid(
				instancePath,
				"",
				"managedRedis and externalRedis are mutually exclusive",
			))
		}

		managed := spec.ManagedRedis
		if managed == nil {
			continue
		}

		if managed.StorageSize != "" {
			if _, err := resource.ParseQuantity(managed.StorageSize); err != nil {
				errors = append(errors, field.Invalid(
					instancePath.Child("managedRedis").Child("storageSize"),
					managed.StorageSize,
					"must be a valid resource quantity (e.g., '10Gi')",
				))
			}
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

func validateObjectStoreSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	objectStorePath := field.NewPath("spec").Child("objectStore")

	errors = append(errors, validateHasDefaultInstance(wandb.Spec.ObjectStore, objectStorePath)...)

	for key, spec := range wandb.Spec.ObjectStore {
		if spec.ManagedObjectStore != nil && spec.ExternalObjectStore != nil {
			errors = append(errors, field.Invalid(
				objectStorePath.Key(key),
				"",
				"managedObjectStore and externalObjectStore are mutually exclusive",
			))
		}
	}

	return errors
}

func validateClickHouseSpec(wandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	chPath := field.NewPath("spec").Child("clickhouse")

	errors = append(errors, validateHasDefaultInstance(wandb.Spec.ClickHouse, chPath)...)

	for key, spec := range wandb.Spec.ClickHouse {
		if spec.ManagedClickHouse != nil && spec.ExternalClickHouse != nil {
			errors = append(errors, field.Invalid(
				chPath.Key(key),
				"",
				"managedClickhouse and externalClickhouse are mutually exclusive",
			))
		}
	}

	return errors
}

func validateRedisChanges(newWandb, oldWandb *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	redisPath := field.NewPath("spec").Child("redis")

	for key, newInstance := range newWandb.Spec.Redis {
		newSpec := newInstance.ManagedRedis
		if newSpec == nil {
			continue
		}
		oldInstance, ok := oldWandb.Spec.Redis[key]
		if !ok || oldInstance.ManagedRedis == nil {
			continue
		}
		oldSpec := oldInstance.ManagedRedis
		instancePath := redisPath.Key(key).Child("managedRedis")

		if oldSpec.StorageSize != "" &&
			oldSpec.StorageSize != newSpec.StorageSize {
			errors = append(errors, field.Invalid(
				instancePath.Child("storageSize"),
				newSpec.StorageSize,
				"storageSize may not be changed",
			))
		}

		if oldSpec.Namespace != newSpec.Namespace {
			errors = append(errors, field.Invalid(
				instancePath.Child("namespace"),
				newSpec.Namespace,
				"namespace may not be changed",
			))
		}

		if oldSpec.Sentinel.Enabled != newSpec.Sentinel.Enabled {
			errors = append(errors, field.Invalid(
				instancePath.Child("sentinel").Child("enabled"),
				newSpec.Sentinel.Enabled,
				"Redis Sentinel cannot be toggled between enabled and disabled (yet)",
			))
		}
	}

	return errors
}

func validateNetworkingSpec(wandb *appsv2.WeightsAndBiases) (field.ErrorList, admission.Warnings) {
	var errors field.ErrorList
	var warnings admission.Warnings
	netPath := field.NewPath("spec").Child("networking")
	spec := wandb.Spec.Networking

	if spec.Mode == appsv2.NetworkingModeNone {
		return errors, warnings
	}

	if spec.Mode == appsv2.NetworkingModeIngress && spec.GatewayAPI != nil {
		errors = append(errors, field.Invalid(
			netPath.Child("gatewayAPI"),
			spec.GatewayAPI,
			"gatewayAPI must not be set when mode is Ingress",
		))
	}

	if spec.Mode == appsv2.NetworkingModeGatewayAPI && spec.Ingress != nil {
		errors = append(errors, field.Invalid(
			netPath.Child("ingress"),
			spec.Ingress,
			"ingress must not be set when mode is GatewayAPI",
		))
	}

	if spec.Mode == appsv2.NetworkingModeGatewayAPI {
		if spec.GatewayAPI == nil {
			errors = append(errors, field.Required(
				netPath.Child("gatewayAPI"),
				"gatewayAPI is required when mode is GatewayAPI",
			))
		} else {
			gwPath := netPath.Child("gatewayAPI").Child("gateway")
			gw := spec.GatewayAPI.Gateway

			if gw.Managed {
				if gw.GatewayClassName == nil || *gw.GatewayClassName == "" {
					errors = append(errors, field.Required(
						gwPath.Child("gatewayClassName"),
						"gatewayClassName is required when gateway.managed is true",
					))
				}
				if gw.GatewayRef != nil {
					errors = append(errors, field.Invalid(
						gwPath.Child("gatewayRef"),
						gw.GatewayRef,
						"gatewayRef must not be set when gateway.managed is true",
					))
				}
			} else {
				if gw.GatewayRef == nil {
					errors = append(errors, field.Required(
						gwPath.Child("gatewayRef"),
						"gatewayRef is required when gateway.managed is false",
					))
				} else if gw.GatewayRef.Name == "" {
					errors = append(errors, field.Required(
						gwPath.Child("gatewayRef").Child("name"),
						"gatewayRef.name is required",
					))
				}
			}
		}
	}

	if spec.TLS != nil && spec.TLS.CertManager != nil && spec.Mode == "" {
		warnings = append(warnings, "networking.tls.certManager annotations are only applied when using Ingress or GatewayAPI")
	}

	return errors, warnings
}
