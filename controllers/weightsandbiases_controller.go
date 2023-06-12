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
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/controllers/internal/ctrlqueue"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"github.com/wandb/operator/pkg/wandb/status"
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

	log.Info("=== Found Weights & Biases instance, processing the spec...", "Spec", wandb.Spec)

	statusManager := status.NewManager(ctx, r.Client, wandb)
	err := statusManager.Set(status.Initializing)
	if err != nil {
		log.Error(err, "Failed to set status")
	}

	log.Info("Get wanted release version...")
	wantedRelease, err := r.getWantedRelease(ctx, wandb)
	if err != nil {
		log.Error(err, "Failed to get wanted release version")
		return ctrlqueue.DoNotRequeue()
	}
	log.Info("Wanted release found", "version", wantedRelease.Version())

	statusManager.Set(status.Loading)
	configManager := config.NewManager(ctx, r.Client, wandb, r.Scheme)
	cfg, err := configManager.GetDesiredState()
	if err != nil {
		log.Error(err, "Failed to get desiered config")
		return ctrlqueue.DoNotRequeue()
	}

	configManager.BackupLatest()

	log.Info("Config values:", "version", cfg.Release.Version(), "config", cfg.Config)
	if err != nil || cfg == nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Failed to get config")
			return ctrlqueue.Requeue(err)
		}

		log.Info("No config found. Creating config...", "name", configManager.LatestConfigName())
		if _, err := configManager.CreateLatest(wantedRelease, nil); err != nil {
			log.Error(err, "Failed to create config")
			return ctrlqueue.Requeue(err)
		}
	}

	// If we get to this point the above config has been created and something
	// happen to trigger the reconcile function. However the config is not valid
	if cfg.Config == nil {
		log.Info("Config is null (default config).")
		statusManager.Set(status.InvalidConfig)

		// Lets update the version incase it has changed.
		cfg.SetRelease(wantedRelease)
		configManager.SetDesiredState(cfg)

		return ctrlqueue.DoNotRequeue()
	}

	statusManager.Set(status.Loading)
	log.Info("Applying config changes...", "version", cfg.Release.Version())
	if err := r.applyConfig(ctx, wandb, cfg); err != nil {
		// TODO: Implement rollback
		log.Error(err, "Failed to apply config changes.")
		return ctrlqueue.DoNotRequeue()
	}
	log.Info("Succesfully applied config", "version", cfg.Release.Version())

	if wantedRelease.Version() != cfg.Release.Version() {
		log.Info(
			"Version changed. Applying...",
			"current", cfg.Release.Version(),
			"desired version", wantedRelease.Version(),
		)

		statusManager.Set(status.Loading)

		cfg.SetRelease(wantedRelease)

		if err := r.applyConfig(ctx, wandb, cfg); err != nil {
			// TODO: Implement rollback
			log.Error(err, "Failed to upgrade to new version.")
			return ctrlqueue.DoNotRequeue()
		}

		// Only if succesful do we save the state to a config map.
		if err := configManager.SetDesiredState(cfg); err != nil {
			log.Error(err, "Failed to set desired state")
			return ctrlqueue.DoNotRequeue()
		}
	}

	// All done! Everything is up to date.
	statusManager.Set(status.Completed)
	return ctrlqueue.DoNotRequeue()
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightsAndBiasesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(
			&apiv1.WeightsAndBiases{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{})
	return builder.Complete(r)
}
