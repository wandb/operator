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
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var defaultRequeueMinutes = 1
var defaultRequeueDuration = time.Duration(defaultRequeueMinutes) * time.Minute

// Reconcile for V2 of WandB as the assumed object
func Reconcile(ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases) (ctrl.Result, error) {
	var err error

	/////////////////////////
	// Resource CRUD
	if err = redisResourceReconcile(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = mysqlResourceReconcile(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = kafkaResourceReconcile(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = minioResourceReconcile(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = clickHouseResourceReconcile(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	/////////////////////////
	// Status Update
	if err = redisStatusUpdate(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = mysqlStatusUpdate(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = kafkaStatusUpdate(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = minioStatusUpdate(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = clickHouseStatusUpdate(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	/////////////////////////

	if err = inferState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
}

func inferState(
	ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	redisStatus := wandb.Status.RedisStatus

	wandb.Status.State = redisStatus.State

	if err := client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status")
		return err
	}
	return nil
}
