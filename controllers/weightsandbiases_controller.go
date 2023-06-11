/*
Copyright 2023.

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

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/controllers/internal/ctrlqueue"
	"github.com/wandb/operator/pkg/wandb/cdk8s"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"k8s.io/apimachinery/pkg/api/errors"
)

// WeightsAndBiasesReconciler reconciles a WeightsAndBiases object
type WeightsAndBiasesReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WeightsAndBiases object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WeightsAndBiasesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	wandb := &apiv1.WeightsAndBiases{}
	if err := r.Get(ctx, req.NamespacedName, wandb); err != nil {
		if errors.IsNotFound(err) {
			return ctrlqueue.DoNotRequeue()
		}
		return ctrlqueue.Requeue(err)
	}

	log.Info("Found Weights & Biases instance, processing the spec...", "Spec", wandb.Spec)

	// 	config := map[string]interface{}{
	// 		"mysql": map[string]interface{}{
	// 			"database": "wandb_local",
	// 			"port":     3306,
	// 			"user":     "wandb",
	// 			"host":     "localhost",
	// 			"password": map[string]interface{}{
	// 				"secret": "mysql",
	// 				"key":    "password",
	// 			},
	// 		},
	// 		"bucket": map[string]interface{}{
	// 			"connectionString": "s3://wandb-local",
	// 			"region":           "us-east-1",
	// 		},
	// 	}

	cdkManager := cdk8s.NewManager(ctx, wandb)
	configManager := config.NewManager(ctx, r.Client, wandb, r.Scheme)

	wantedRelease, err := cdkManager.GetLatestSupportedRelease()
	if err != nil {
		wantedRelease, _ = cdkManager.GetDownloadedRelease()
		if wantedRelease == nil {
			log.Error(err, "Failed to find any release already downloaded.")
			return ctrlqueue.Requeue(err)
		}
	}

	if wandb.Spec.Cdk8sVersion != "" {
		wantedRelease = cdkManager.GetSpecRelease()
		if wantedRelease == nil {
			log.Error(err, "Failed to find any release already downloaded.")
			return ctrlqueue.Requeue(err)
		}
	}

	if err != nil {
		log.Error(err, "Failed to get a release")
		return ctrlqueue.Requeue(err)
	}

	cfg, err := configManager.GetDesiredState()
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Failed to get config")
			return ctrlqueue.Requeue(err)
		}

		log.Info("No config found. Creating config")
		configManager.CreateLatest(wantedRelease, nil)
	}

	if cfg.Release == nil || cfg.Config == nil {
		log.Error(err, "No valid config found. Admin console has set config values.")
		return ctrlqueue.Requeue(err)
	}

	if err := configManager.Reconcile(cfg); err != nil {
		log.Error(err, "Failed to apply config changes")
		return ctrlqueue.DoNotRequeue()
	}

	if _, err := configManager.Backup(cfg); err != nil {
		log.Error(err, "Failed to backup config")
		return ctrlqueue.Requeue(err)
	}

	// Version has chanaged
	if wantedRelease.Version() != cfg.Release.Version() {
		log.Info(
			"version changed",
			"current", cfg.Release.Version(),
			"desired version", wantedRelease.Version(),
		)
		cfg.SetRelease(wantedRelease)

		if err := configManager.Reconcile(cfg); err != nil {
			log.Error(err, "Failed to upgrade")
			return ctrlqueue.DoNotRequeue()
		}

		if _, err := configManager.Backup(cfg); err != nil {
			log.Error(err, "Failed to backup config with updated version")
			return ctrlqueue.Requeue(err)
		}
	}

	return ctrlqueue.DoNotRequeue()
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightsAndBiasesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.WeightsAndBiases{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		WithEventFilter(predicate.GenerationChangedPredicate{})
	return builder.Complete(r)
}
