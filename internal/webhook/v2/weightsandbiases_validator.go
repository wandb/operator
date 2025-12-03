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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apiv2 "github.com/wandb/operator/api/v2"
)

var whLog = ctrllog.Log.WithName("wandb-v2-webhook")

func ValidateCreate(ctx context.Context, wandb *apiv2.WeightsAndBiases) (admission.Warnings, error) {
	whLog.Info("validate V2 create", "name", wandb.Name)

	return validateSpec(ctx, wandb)
}

func ValidateUpdate(ctx context.Context, oldWandb, newWandb *apiv2.WeightsAndBiases) (admission.Warnings, error) {
	var specWarnings, changeWarnings admission.Warnings
	var err error

	whLog.Info("validate V2 update", "name", newWandb.Name)

	if specWarnings, err = validateSpec(ctx, newWandb); err != nil {
		return specWarnings, err
	}
	changeWarnings, err = validateChanges(ctx, oldWandb, newWandb)
	return append(specWarnings, changeWarnings...), err
}

func ValidateDelete(ctx context.Context, wandb *apiv2.WeightsAndBiases) (admission.Warnings, error) {
	whLog.Info("validate V2 delete", "name", wandb.Name)

	return nil, nil
}

func validateSpec(ctx context.Context, newWandb *apiv2.WeightsAndBiases) (admission.Warnings, error) {
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

func validateChanges(ctx context.Context, newWandb *apiv2.WeightsAndBiases, oldWandb *apiv2.WeightsAndBiases) (admission.Warnings, error) {
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
