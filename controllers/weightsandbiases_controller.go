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
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/operator"
	"github.com/wandb/operator/pkg/wandb/spec/state"
	"github.com/wandb/operator/pkg/wandb/spec/state/configmap"
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
	if err := r.Client.Get(ctx, req.NamespacedName, wandb); err != nil {
		if errors.IsNotFound(err) {
			return ctrlqueue.DoNotRequeue()
		}
		return ctrlqueue.Requeue(err)
	}

	log.Info("=== Found Weights & Biases instance, processing the spec...", "Spec", wandb.Spec)

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "Reconciling", "Reconciling")

	statusManager := status.NewManager(ctx, r.Client, wandb)
	configMapState := configmap.New(ctx, r.Client, wandb, r.Scheme)
	specManager := state.New(ctx, r.Client, wandb, r.Scheme, configMapState)

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "LoadingConfig", "Loading desired configuration")

	// var license string
	// if userInputSpec != nil {
	// 	if l, exists := userInputSpec.Config["license"]; exists {
	// 		if ls, ok := l.(string); ok {
	// 			license = ls
	// 		}
	// 	}
	// }

	// if l, exists := wandb.Spec.Config.Object["license"]; exists {
	// 	if ls, ok := l.(string); ok {
	// 		license = ls
	// 	}
	// }

	// TODO: Implement a way to get the latest config
	// cdk8s.Deployer(license)

	userInputSpec, err := specManager.GetUserInput()
	if userInputSpec == nil {
		log.Info("No user spec found, creating a new one")
		userInputSpec := &spec.Spec{Config: map[string]interface{}{}}
		specManager.SetUserInput(userInputSpec)
	}

	desiredSpec := &spec.Spec{}
	desiredSpec.Merge(operator.Defaults(wandb, r.Scheme))
	desiredSpec.Merge(userInputSpec)
	desiredSpec.Merge(operator.Spec(wandb))

	log.Info("Desired spec", "spec", desiredSpec)
	log.Info("Applying desired spec", "spec", desiredSpec)

	if desiredSpec.Release == nil {
		statusManager.Set(status.InvalidConfig)
		log.Error(err, "No release type was found in the spec")
		return ctrlqueue.DoNotRequeue()
	}

	statusManager.Set(status.Loading)
	log.Info("Applying spec...", "spec", desiredSpec)
	if err := desiredSpec.Apply(ctx, r.Client, wandb, r.Scheme); err != nil {
		statusManager.Set(status.InvalidConfig)
		r.Recorder.Event(wandb, corev1.EventTypeNormal, "ApplyFailed", "Invalid config for apply")
		log.Error(err, "Failed to apply config changes.")
		return ctrlqueue.DoNotRequeue()
	}
	log.Info("Successfully applied spec", "spec", desiredSpec)

	if err := specManager.SetActive(desiredSpec); err != nil {
		r.Recorder.Event(wandb, corev1.EventTypeNormal, "SetActiveFailed", "Failed to save active state")
		log.Error(err, "Failed to save active sucessful spec.")
		return ctrlqueue.DoNotRequeue()
	}
	log.Info("Successfully saved active state")

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "SavingConfig", "Creating a copy of desired state")
	if err := specManager.Backup(desiredSpec); err != nil {
		r.Recorder.Event(wandb, corev1.EventTypeNormal, "BackupFailed", "Failed to backup desired state")
		log.Error(err, "Failed to backup sucessful spec.")
		return ctrlqueue.DoNotRequeue()
	}
	log.Info("Successfully backed up")

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
