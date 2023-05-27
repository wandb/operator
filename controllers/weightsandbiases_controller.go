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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	apiv1 "github.com/wandb/operator/api/v1"
	wandbcomv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/controllers/internal/ctrlqueue"
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

	log.Info("Reconciling Weights & Biases")

	wandb := &apiv1.WeightsAndBiases{}
	if err := r.Get(ctx, req.NamespacedName, wandb); err != nil {
		if errors.IsNotFound(err) {
			return ctrlqueue.DoNotRequeue()
		}
		// could not get GitLab resource
		return ctrlqueue.Requeue(err)
	}

	return ctrlqueue.DoNotRequeue()
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightsAndBiasesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&wandbcomv1.WeightsAndBiases{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&batchv1.Job{}).
		Owns(&networkingv1.Ingress{}).
		WithEventFilter(predicate.GenerationChangedPredicate{})
	return builder.Complete(r)
}
