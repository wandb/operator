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

var weightsandbiaseslog = logf.Log.WithName("weightsandbiases-v2-validation")

type WeightsAndBiasesCustomValidator struct{}

func (v *WeightsAndBiasesCustomValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appsv2.WeightsAndBiases{}).
		WithValidator(v).
		Complete()
}

func (v *WeightsAndBiasesCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	weightsandbiases, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	weightsandbiaseslog.Info("validate create", "name", weightsandbiases.Name)

	return v.validate(ctx, weightsandbiases, nil)
}

func (v *WeightsAndBiasesCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	weightsandbiases, ok := newObj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object for the newObj but got %T", newObj)
	}
	weightsandbiaseslog.Info("validate update", "name", weightsandbiases.Name)

	oldWeightsAndBiases, ok := oldObj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object for the oldObj but got %T", oldObj)
	}

	return v.validate(ctx, weightsandbiases, oldWeightsAndBiases)
}

func (v *WeightsAndBiasesCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	weightsandbiases, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	weightsandbiaseslog.Info("validate delete", "name", weightsandbiases.Name)

	return nil, nil
}

func (v *WeightsAndBiasesCustomValidator) validate(ctx context.Context, weightsandbiases *appsv2.WeightsAndBiases, old *appsv2.WeightsAndBiases) (admission.Warnings, error) {
	var allErrors field.ErrorList
	var warnings admission.Warnings

	allErrors = append(allErrors, v.validateRedis(weightsandbiases)...)

	if len(allErrors) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(
		schema.GroupKind{Group: "apps.wandb.com", Kind: "WeightsAndBiases"},
		weightsandbiases.Name,
		allErrors,
	)
}

func (v *WeightsAndBiasesCustomValidator) validateRedis(weightsandbiases *appsv2.WeightsAndBiases) field.ErrorList {
	var errors field.ErrorList
	redisPath := field.NewPath("spec").Child("redis")

	if !weightsandbiases.Spec.Redis.Enabled {
		return errors
	}

	if weightsandbiases.Spec.Redis.StorageSize != "" {
		if _, err := resource.ParseQuantity(weightsandbiases.Spec.Redis.StorageSize); err != nil {
			errors = append(errors, field.Invalid(
				redisPath.Child("storageSize"),
				weightsandbiases.Spec.Redis.StorageSize,
				"must be a valid resource quantity (e.g., '10Gi', '1Ti')",
			))
		}
	}

	if weightsandbiases.Spec.Redis.Sentinel != nil && weightsandbiases.Spec.Redis.Sentinel.Enabled {
		if !weightsandbiases.Spec.Redis.Enabled {
			errors = append(errors, field.Invalid(
				redisPath.Child("sentinel").Child("enabled"),
				weightsandbiases.Spec.Redis.Sentinel.Enabled,
				"Redis Sentinel cannot be enabled when Redis is disabled",
			))
		}
	}

	return errors
}

var _ webhook.CustomValidator = &WeightsAndBiasesCustomValidator{}
