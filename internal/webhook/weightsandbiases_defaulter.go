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
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	apiv2 "github.com/wandb/operator/api/v2"
)

type WeightsAndBiasesCustomDefaulter struct {
	AssumeV2Cr bool
}

func (d *WeightsAndBiasesCustomDefaulter) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if d.AssumeV2Cr {
		return ctrl.NewWebhookManagedBy(mgr).
			For(&apiv2.WeightsAndBiases{}).
			WithDefaulter(d).
			Complete()
	}
	return ctrl.NewWebhookManagedBy(mgr).
		For(&apiv1.WeightsAndBiases{}).
		WithDefaulter(d).
		Complete()
}

func (d *WeightsAndBiasesCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	if d.AssumeV2Cr {
		wandb, ok := obj.(*apiv2.WeightsAndBiases)
		if !ok {
			return fmt.Errorf("expected a WeightsAndBiases object but got %T", obj)
		}
		return v2.Default(ctx, wandb)
	}

	return nil
}

var _ webhook.CustomDefaulter = &WeightsAndBiasesCustomDefaulter{}
