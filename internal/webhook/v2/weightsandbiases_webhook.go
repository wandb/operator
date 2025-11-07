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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	appsv2 "github.com/wandb/operator/api/v2"
)

var whLog = logf.Log.WithName("wandb-v2-webhook")

type WeightsAndBiasesCustomValidator struct{}

func (v *WeightsAndBiasesCustomValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appsv2.WeightsAndBiases{}).
		WithValidator(v).
		Complete()
}

func (v *WeightsAndBiasesCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	wandb, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	whLog.Info("validate create", "name", wandb.Name)

	return v.validateSpec(ctx, wandb)
}

func (v *WeightsAndBiasesCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	var specWarnings, changeWarnings admission.Warnings
	var err error

	newWandb, ok := newObj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object for the newObj but got %T", newObj)
	}
	whLog.Info("validate update", "name", newWandb.Name)

	oldWandb, ok := oldObj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object for the oldObj but got %T", oldObj)
	}

	if specWarnings, err = v.validateSpec(ctx, newWandb); err != nil {
		return specWarnings, err
	}
	changeWarnings, err = v.validateChanges(ctx, oldWandb, newWandb)
	return append(specWarnings, changeWarnings...), err
}

func (v *WeightsAndBiasesCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	wandb, ok := obj.(*appsv2.WeightsAndBiases)
	if !ok {
		return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
	}
	whLog.Info("validate delete", "name", wandb.Name)

	return nil, nil
}

func (v *WeightsAndBiasesCustomValidator) validateSpec(ctx context.Context, newWandb *appsv2.WeightsAndBiases) (admission.Warnings, error) {
	var allErrors field.ErrorList
	var warnings admission.Warnings

	allErrors = append(allErrors, v.validateRedisSpec(newWandb)...)

	if len(allErrors) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(
		schema.GroupKind{Group: "apps.wandb.com", Kind: "WeightsAndBiases"},
		newWandb.Name,
		allErrors,
	)
}

func (v *WeightsAndBiasesCustomValidator) validateChanges(ctx context.Context, newWandb *appsv2.WeightsAndBiases, oldWandb *appsv2.WeightsAndBiases) (admission.Warnings, error) {
	var allErrors field.ErrorList
	var warnings admission.Warnings

	allErrors = append(allErrors, v.validateRedisChanges(newWandb, oldWandb)...)

	if len(allErrors) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(
		schema.GroupKind{Group: "apps.wandb.com", Kind: "WeightsAndBiases"},
		newWandb.Name,
		allErrors,
	)
}

var _ webhook.CustomValidator = &WeightsAndBiasesCustomValidator{}
