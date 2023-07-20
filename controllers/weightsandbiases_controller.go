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
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/controllers/internal/ctrlqueue"
	"github.com/wandb/operator/pkg/wandb/cdk8s"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"github.com/wandb/operator/pkg/wandb/status"
	"k8s.io/apimachinery/pkg/api/errors"
)

// WeightsAndBiasesReconciler reconciles a WeightsAndBiases object
type WeightsAndBiasesReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets;configmaps;configmaps,verbs=update;delete;get;list;create;watch
//+kubebuilder:rbac:groups="",resources=pods;services;nodes,verbs=update;delete;get;list;create;watch
//+kubebuilder:rbac:groups=app,resources=deployments,verbs=update;delete;get;list;create;watch

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

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "Reconciling", "Reconciling")

	configManager := config.NewManager(ctx, r.Client, wandb, r.Scheme)
	usersConfig, err := configManager.GetDesiredState()

	var license string
	if usersConfig != nil {
		if l, exists := usersConfig.Config["license"]; exists {
			if ls, ok := l.(string); ok {
				license = ls
			}
		}
	}
	if wandb.Spec.License != "" {
		license = wandb.Spec.License
	}

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "LoadingConfig", "Loading desired configuration")
	// Apply configs in least to most priority order
	desiredState := config.Merge(
		usersConfig,
		cdk8s.Github(),
		cdk8s.Deployer(license),
		cdk8s.Operator(wandb, r.Scheme),
	)

	log.Info("Desired config", "version", desiredState.Release.Version(), "config", desiredState.Config)

	if err != nil {
		if errors.IsNotFound(err) {
			r.Recorder.Event(wandb, corev1.EventTypeNormal, "CreatingConfig", "Creating config")
			log.Info("No config found. Creating config...", "name", configManager.LatestConfigName())
			usersConfig, err = configManager.CreateLatest(
				desiredState.Release,
				map[string]interface{}{},
			)
			if err != nil {
				statusManager.Set(status.InvalidConfig)
				log.Error(err, "Failed to create config")
				return ctrlqueue.Requeue(err)
			}
		} else {
			log.Error(err, "Failed to get desired config")
			return ctrlqueue.DoNotRequeue()
		}
	}

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "ConfigBackup", "Creating a copy of config")
	configManager.BackupLatest()

	log.Info("Config values:", "version", desiredState.Release.Version(), "config", desiredState.Config)
	if err != nil {
		statusManager.Set(status.InvalidConfig)
		log.Error(err, "Failed to get config")
		return ctrlqueue.Requeue(err)
	}

	statusManager.Set(status.Loading)
	log.Info("Applying config changes...", "version", usersConfig.Release.Version())
	if err := r.applyConfig(ctx, wandb, statusManager, usersConfig.Release, desiredState.Config); err != nil {
		statusManager.Set(status.InvalidConfig)
		r.Recorder.Event(wandb, corev1.EventTypeNormal, "ApplyFailed", "Valid to apply config changes")
		// TODO: Implement rollback
		log.Error(err, "Failed to apply config changes.")
		return ctrlqueue.DoNotRequeue()
	}
	log.Info("Successfully applied config", "version", usersConfig.Release.Version())

	if desiredState.Release.Version() != usersConfig.Release.Version() {
		r.Recorder.Event(wandb, corev1.EventTypeNormal, "VersionChangeDetected", "Version change detected ("+usersConfig.Release.Version()+" -> "+desiredState.Release.Version()+")")
		log.Info(
			"Version changed. Applying...",
			"current", usersConfig.Release.Version(),
			"desired version", desiredState.Release.Version(),
		)

		statusManager.Set(status.Loading)
		usersConfig.SetRelease(desiredState.Release)

		if err := r.applyConfig(ctx, wandb, statusManager, usersConfig.Release, desiredState.Config); err != nil {
			statusManager.Set(status.InvalidVersion)
			r.Recorder.Event(wandb, corev1.EventTypeNormal, "VersionApplyFailed", "Valid to apply version changes")
			// TODO: Implement rollback
			log.Error(err, "Failed to upgrade to new version.")
			return ctrlqueue.DoNotRequeue()
		}

		// Only if successful do we save the state to a config map.
		if err := configManager.SetDesiredState(usersConfig); err != nil {
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
			builder.WithPredicates(annotationChangedPredicate{}),
		).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{})
	return builder.Complete(r)
}

// annotationChangedPredicate implements the Predicate interface. It is used to
// watch changes in the annotation of the Custom Resource.
type annotationChangedPredicate struct {
	predicate.Funcs
}

// It checks if there is an update event and only returns true if the
// annotations have changed. We use annotations to trigger a reconcile
// functions.
func (a annotationChangedPredicate) Update(e event.UpdateEvent) bool {
	return !reflect.DeepEqual(
		e.ObjectOld.GetAnnotations(),
		e.ObjectNew.GetAnnotations(),
	)
}
