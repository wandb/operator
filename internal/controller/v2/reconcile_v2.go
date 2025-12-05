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
	"github.com/wandb/operator/internal/controller/translator/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

var defaultRequeueMinutes = 1
var defaultRequeueDuration = time.Duration(defaultRequeueMinutes) * time.Minute

// Reconcile for V2 of WandB as the assumed object
func Reconcile(ctx context.Context, client client.Client, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info(
		"=== Reconciling Weights & Biases V2 instance...",
		"NamespacedName", req.NamespacedName,
		"Name", req.Name,
		"start", true,
	)

	var minioConnection *common.MinioConnection
	var err error

	wandb := &apiv2.WeightsAndBiases{}
	if err := client.Get(ctx, req.NamespacedName, wandb); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info(
		"Found Weights & Biases V2 instance, processing the spec...",
		"Spec", wandb.Spec, "Name", wandb.Name, "UID", wandb.UID, "Generation", wandb.Generation,
	)

	/////////////////////////
	// Write State
	if err = redisWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = mysqlWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = kafkaWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if minioConnection, err = minioWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = clickHouseWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	/////////////////////////
	// Status Update
	if err = redisReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = mysqlReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = kafkaReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = minioReadState(ctx, client, wandb, minioConnection); err != nil {
		return ctrl.Result{}, err
	}
	if err = clickHouseReadState(ctx, client, wandb); err != nil {
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
