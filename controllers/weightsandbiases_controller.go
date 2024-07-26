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
	"os"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/controllers/internal/ctrlqueue"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer"
	"github.com/wandb/operator/pkg/wandb/spec/operator"
	"github.com/wandb/operator/pkg/wandb/spec/state"
	"github.com/wandb/operator/pkg/wandb/spec/state/secrets"
	"github.com/wandb/operator/pkg/wandb/spec/utils"
	"github.com/wandb/operator/pkg/wandb/status"

	"k8s.io/apimachinery/pkg/api/errors"
)

const resFinalizer = "finalizer.app.wandb.com"

// WeightsAndBiasesReconciler reconciles a WeightsAndBiases object
type WeightsAndBiasesReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=update;delete;get;list;create;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WeightsAndBiasesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info(
		"=== Reconciling Weights & Biases instance...",
		"NamespacedName", req.NamespacedName,
		"Name", req.Name,
		"start", true,
	)

	wandb := &apiv1.WeightsAndBiases{}
	if err := r.Client.Get(ctx, req.NamespacedName, wandb); err != nil {
		if errors.IsNotFound(err) {
			return ctrlqueue.DoNotRequeue()
		}
		return ctrlqueue.RequeueWithError(err)
	}

	log.Info(
		"Found Weights & Biases instance, processing the spec...",
		"Spec", wandb.Spec,
	)

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "Reconciling", "Reconciling")

	statusManager := status.NewManager(ctx, r.Client, wandb)
	configMapState := secrets.New(ctx, r.Client, wandb, r.Scheme)
	specManager := state.New(ctx, r.Client, wandb, r.Scheme, configMapState)

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "LoadingConfig", "Loading desired configuration")

	userInputSpec, _ := specManager.GetUserInput()
	if userInputSpec == nil {
		log.Info("No user spec found, creating a new one")
		userInputSpec := &spec.Spec{Values: map[string]interface{}{}}
		specManager.SetUserInput(userInputSpec)
	}

	crdSpec := operator.Spec(wandb)

	currentActiveSpec, err := specManager.GetActive()
	if err != nil {
		// This scenario can happen if we have not successfully deploy in the
		// past.
		log.Info("No active spec found.")
	}

	license := utils.GetLicense(currentActiveSpec, crdSpec, userInputSpec)
	log.Info("License", "license", license)

	isAirgapped := strings.EqualFold(os.Getenv("AIRGAPPED"), "true")
	var deployerSpec *spec.Spec
	if !isAirgapped {
		deployerSpec, err = deployer.GetSpec(license, currentActiveSpec)
		if err != nil {
			log.Info("Failed to get spec from deployer", "error", err)
			// This scenario may occur if the user disables networking, or if the deployer
			// is not operational, and a version has been deployed successfully. Rather than
			// reverting to the container defaults, we've stored the most recent successful
			// deployer release in the cache
			if deployerSpec, err = specManager.Get("latest-cached-release"); err != nil {
				log.Error(err, "No cached release found for deployer spec", err, "error")
			}
		}

		if deployerSpec != nil {
			if err := specManager.Set("latest-cached-release", deployerSpec); err != nil {
				r.Recorder.Event(wandb, corev1.EventTypeNormal, "SecretWriteFailed", "Unable to write secret to kubernetes")
				log.Error(err, "Unable to save latest release.")
				return ctrlqueue.DoNotRequeue()
			}
		}
	}

	desiredSpec := new(spec.Spec)
	// First takes precedence
	desiredSpec.Merge(crdSpec)
	desiredSpec.Merge(userInputSpec)
	desiredSpec.Merge(deployerSpec)
	desiredSpec.Merge(operator.Defaults(wandb, r.Scheme))

	log.Info("Desired spec", "spec", desiredSpec.SensitiveValuesMasked())

	hasNotBeenFlaggedForDeletion := wandb.ObjectMeta.DeletionTimestamp.IsZero()
	if hasNotBeenFlaggedForDeletion {
		if currentActiveSpec != nil {
			log.Info("Active spec found", "spec", currentActiveSpec.SensitiveValuesMasked())
			if currentActiveSpec.IsEqual(desiredSpec) {
				log.Info("No changes found")
				statusManager.Set(status.Completed)
				return ctrlqueue.Requeue(desiredSpec)
			}
		}

		if desiredSpec.Chart == nil {
			statusManager.Set(status.InvalidConfig)
			log.Error(err, "No release type was found in the spec")
			return ctrlqueue.Requeue(desiredSpec)
		}
		t := reflect.TypeOf(desiredSpec.Chart)
		typ := t.Name()
		if t.Kind() == reflect.Ptr {
			typ = "*" + t.Elem().Name()
		}
		log.Info("Found release type "+typ, "release", reflect.TypeOf(desiredSpec.Chart))

		statusManager.Set(status.Loading)

		if !ctrlqueue.ContainsString(wandb.GetFinalizers(), resFinalizer) {
			wandb.ObjectMeta.Finalizers = append(wandb.ObjectMeta.Finalizers, resFinalizer)
			if err := r.Client.Update(ctx, wandb); err != nil {
				return ctrlqueue.Requeue(desiredSpec)
			}
		}

		log.Info("Applying spec...", "spec", desiredSpec)
		if err := desiredSpec.Apply(ctx, r.Client, wandb, r.Scheme); err != nil {
			statusManager.Set(status.InvalidConfig)
			r.Recorder.Event(wandb, corev1.EventTypeNormal, "ApplyFailed", "Invalid config for apply")
			log.Error(err, "Failed to apply config changes.")
			return ctrlqueue.Requeue(desiredSpec)
		}
		log.Info("Successfully applied spec", "spec", desiredSpec)

		if err := specManager.SetActive(desiredSpec); err != nil {
			r.Recorder.Event(wandb, corev1.EventTypeNormal, "SetActiveFailed", "Failed to save active state")
			log.Error(err, "Failed to save active successful spec.")
			statusManager.Set(status.InvalidConfig)
			return ctrlqueue.Requeue(desiredSpec)
		}
		log.Info("Successfully saved active spec")

		r.Recorder.Event(wandb, corev1.EventTypeNormal, "Completed", "Completed reconcile successfully")
		statusManager.Set(status.Completed)

		return ctrlqueue.Requeue(desiredSpec)
	}

	if ctrlqueue.ContainsString(wandb.ObjectMeta.Finalizers, resFinalizer) {
		if desiredSpec.Chart != nil {
			log.Info("Deprovisioning", "release", reflect.TypeOf(desiredSpec.Chart))
			if err := desiredSpec.Prune(ctx, r.Client, wandb, r.Scheme); err != nil {
				log.Error(err, "Failed to cleanup deployment.")
			} else {
				log.Info("Successfully cleaned up resources")
			}
		}

		controllerutil.RemoveFinalizer(wandb, resFinalizer)
		r.Client.Update(ctx, wandb)
	}

	return ctrlqueue.DoNotRequeue()
}

func (r *WeightsAndBiasesReconciler) Delete(e event.DeleteEvent) bool {
	return !e.DeleteStateUnknown
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightsAndBiasesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.WeightsAndBiases{}, builder.WithPredicates(filterWBEvents{})).
		Owns(&corev1.Secret{}, builder.WithPredicates(filterSecretEvents{})).
		Owns(&corev1.ConfigMap{})
	return builder.Complete(r)
}

type filterWBEvents struct {
	predicate.Funcs
}

func (filterWBEvents) Update(e event.UpdateEvent) bool {
	// Checking whether the Object's Generation has changed. If it has not
	// (indicating a non-spec change), it returns false - thus ignoring the
	// event.
	return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
}

func (filterWBEvents) Create(e event.CreateEvent) bool {
	return true
}

func (filterWBEvents) Delete(e event.DeleteEvent) bool {
	return true
}

func (filterWBEvents) Generic(e event.GenericEvent) bool {
	return false
}

type filterSecretEvents struct {
	predicate.Funcs
}

func (filterSecretEvents) Update(e event.UpdateEvent) bool {
	return true
}

func (filterSecretEvents) Create(e event.CreateEvent) bool {
	return false
}

func (filterSecretEvents) Delete(e event.DeleteEvent) bool {
	return true
}

func (filterSecretEvents) Generic(e event.GenericEvent) bool {
	return false
}
