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

package controller

import (
	"context"

	"github.com/wandb/operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wandbv2 "github.com/wandb/operator/api/v2"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.wandb.com,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.wandb.com,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.wandb.com,resources=applications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Application object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var app wandbv2.Application
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "unable to fetch Application")
			return ctrl.Result{}, err
		}
		logger.Info("Application not found. Ignoring since object must be deleted.")
		return ctrl.Result{}, nil
	}

	logger.Info("Handling Application", "Application", app.Name)

	switch app.Spec.Kind {
	case "Deployment":
		return r.reconcileDeployment(ctx, &app)
	case "Rollout":
		return r.reconcileRollout(ctx, &app)
	case "StatefulSet":
		return r.reconcileStatefulSet(ctx, &app)
	case "DaemonSet":
		return r.reconcileDaemonSet(ctx, &app)
	case "Job":
		return r.reconcileJob(ctx, &app)
	case "CronJob":
		return r.reconcileCronJob(ctx, &app)
	default:
		logger.Info("Unsupported application kind", "Kind", app.Spec.Kind)
		return ctrl.Result{}, nil
	}
}

// reconcileDeployment handles Deployment type applications
func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Deployment", "Application", app.Name)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, deployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get Deployment")
			return ctrl.Result{}, err
		}
		logger.Info("Deployment not found", "Deployment", app.Name)
	}

	deployment.Name = app.Name
	deployment.Namespace = app.Namespace

	deployment.Spec.Template.Spec = *app.Spec.PodTemplate.Spec.DeepCopy()
	deployment.Spec.Template.ObjectMeta.Labels =
		utils.MergeMapsStringString(
			deployment.Spec.Template.ObjectMeta.Labels,
			app.Spec.MetaTemplate.Labels,
			app.Spec.PodTemplate.ObjectMeta.Labels,
		)

	deployment.Spec.Template.ObjectMeta.Annotations =
		utils.MergeMapsStringString(
			deployment.Spec.Template.ObjectMeta.Annotations,
			app.Spec.MetaTemplate.Annotations,
			app.Spec.PodTemplate.ObjectMeta.Annotations,
		)

	if deployment.CreationTimestamp.IsZero() {
		if err := r.Create(ctx, deployment); err != nil {
			logger.Error(err, "Failed to create Deployment")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Update(ctx, deployment); err != nil {
			logger.Error(err, "Failed to update Deployment")
			return ctrl.Result{}, err
		}
	}

	logger.Info("Successfully reconciled Deployment", "Deployment", deployment.Name)
	return ctrl.Result{}, nil
}

// reconcileRollout handles Rollout type applications
func (r *ApplicationReconciler) reconcileRollout(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Rollout", "Application", app.Name)
	return ctrl.Result{}, nil
}

// reconcileStatefulSet handles StatefulSet type applications
func (r *ApplicationReconciler) reconcileStatefulSet(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling StatefulSet", "Application", app.Name)
	return ctrl.Result{}, nil
}

// reconcileDaemonSet handles DaemonSet type applications
func (r *ApplicationReconciler) reconcileDaemonSet(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling DaemonSet", "Application", app.Name)
	return ctrl.Result{}, nil
}

// reconcileJob handles Job type applications
func (r *ApplicationReconciler) reconcileJob(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Job", "Application", app.Name)
	return ctrl.Result{}, nil
}

// reconcileCronJob handles CronJob type applications
func (r *ApplicationReconciler) reconcileCronJob(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling CronJob", "Application", app.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wandbv2.Application{}).
		Named("application").
		Complete(r)
}
