/*
Copyright 2026.

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

package controller

import (
	"context"
	"time"

	apiv1 "github.com/wandb/operator/api/v1"
	operatormetrics "github.com/wandb/operator/internal/metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *WeightsAndBiasesReconciler) ensureApplyPendingSince(
	ctx context.Context,
	wandb *apiv1.WeightsAndBiases,
	now time.Time,
) error {
	pendingSince, err := time.Parse(time.RFC3339Nano, wandb.Annotations[operatormetrics.ApplyPendingSinceAnnotation])
	if err == nil && !pendingSince.After(now) {
		return nil
	}

	original := wandb.DeepCopy()
	if wandb.Annotations == nil {
		wandb.Annotations = map[string]string{}
	}
	wandb.Annotations[operatormetrics.ApplyPendingSinceAnnotation] = now.UTC().Format(time.RFC3339Nano)
	return r.Patch(ctx, wandb, client.MergeFrom(original))
}

func (r *WeightsAndBiasesReconciler) clearApplyPendingSince(
	ctx context.Context,
	wandb *apiv1.WeightsAndBiases,
) error {
	if _, exists := wandb.Annotations[operatormetrics.ApplyPendingSinceAnnotation]; !exists {
		return nil
	}

	original := wandb.DeepCopy()
	delete(wandb.Annotations, operatormetrics.ApplyPendingSinceAnnotation)
	return r.Patch(ctx, wandb, client.MergeFrom(original))
}
