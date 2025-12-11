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
	"github.com/wandb/operator/internal/controller/translator"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var defaultRequeueMinutes = 1
var defaultRequeueDuration = time.Duration(defaultRequeueMinutes) * time.Minute

// Reconcile for V2 of WandB as the assumed object
func Reconcile(ctx context.Context, client client.Client, wandb *apiv2.WeightsAndBiases) (ctrl.Result, error) {
	var minioConnection *translator.InfraConnection
	var err error

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

	// Infra is "ok" if either it is not enabled or if it is (enabled and) ready
	redisOk := !wandb.Spec.Redis.Enabled || wandb.Status.RedisStatus.Ready
	minioOk := !wandb.Spec.Minio.Enabled || wandb.Status.MinioStatus.Ready
	mysqlOk := !wandb.Spec.MySQL.Enabled || wandb.Status.MySQLStatus.Ready
	clickHouseOk := !wandb.Spec.ClickHouse.Enabled || wandb.Status.ClickHouseStatus.Ready
	kafkaOk := !wandb.Spec.Kafka.Enabled || wandb.Status.KafkaStatus.Ready

	if redisOk && minioOk && mysqlOk && clickHouseOk && kafkaOk {
		wandb.Status.State = "Ready"
	} else {
		wandb.Status.State = "NotReady"
	}

	if err := client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status")
		return err
	}
	return nil
}
