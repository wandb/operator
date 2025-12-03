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

	apiv1 "github.com/wandb/operator/api/v1"
	v2 "github.com/wandb/operator/internal/webhook/v2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	apiv2 "github.com/wandb/operator/api/v2"
)

var whLog = ctrllog.Log.WithName("wandb-v2-webhook")

type WeightsAndBiasesCustomValidator struct {
	AssumeV2Cr bool
}

func (v *WeightsAndBiasesCustomValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if v.AssumeV2Cr {
		return ctrl.NewWebhookManagedBy(mgr).
			For(&apiv2.WeightsAndBiases{}).
			WithValidator(v).
			Complete()
	}
	return ctrl.NewWebhookManagedBy(mgr).
		For(&apiv1.WeightsAndBiases{}).
		WithValidator(v).
		Complete()
}

func (v *WeightsAndBiasesCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	if v.AssumeV2Cr {
		wandb, ok := obj.(*apiv2.WeightsAndBiases)
		if !ok {
			return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
		}
		return v2.ValidateCreate(ctx, wandb)
	}
	return nil, nil
}

func (v *WeightsAndBiasesCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	if v.AssumeV2Cr {
		var ok bool
		oldWandb, ok := oldObj.(*apiv2.WeightsAndBiases)
		if !ok {
			return nil, fmt.Errorf("expected a WeightsAndBiases old object but got %T", oldWandb)
		}
		newWandb, ok := newObj.(*apiv2.WeightsAndBiases)
		if !ok {
			return nil, fmt.Errorf("expected a WeightsAndBiases new object but got %T", oldWandb)
		}
		return v2.ValidateUpdate(ctx, oldWandb, newWandb)
	}
	return nil, nil
}

func (v *WeightsAndBiasesCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	if v.AssumeV2Cr {
		wandb, ok := obj.(*apiv2.WeightsAndBiases)
		if !ok {
			return nil, fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
		}
		return v2.ValidateDelete(ctx, wandb)
	}
	return nil, nil
}

var _ webhook.CustomValidator = &WeightsAndBiasesCustomValidator{}
