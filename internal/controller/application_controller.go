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
	"fmt"

	"github.com/wandb/operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	var result ctrl.Result
	var err error

	switch app.Spec.Kind {
	case "Deployment":
		result, err = r.reconcileDeployment(ctx, &app)
	case "Rollout":
		result, err = r.reconcileRollout(ctx, &app)
	case "StatefulSet":
		result, err = r.reconcileStatefulSet(ctx, &app)
	case "DaemonSet":
		result, err = r.reconcileDaemonSet(ctx, &app)
	case "Job":
	case "CronJob":
		break
	default:
		logger.Info("Unsupported application kind", "Kind", app.Spec.Kind)
	}

	if err != nil {
		return result, err
	}

	// Always reconcile Jobs regardless of the main application type
	if err := r.reconcileJobs(ctx, &app); err != nil {
		logger.Error(err, "Failed to reconcile Jobs")
		return ctrl.Result{}, err
	}

	// Always reconcile CronJobs regardless of the main application type
	if err := r.reconcileCronJobs(ctx, &app); err != nil {
		logger.Error(err, "Failed to reconcile CronJobs")
		return ctrl.Result{}, err
	}

	return result, nil
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

	selectorLabels := map[string]string{
		"app.kubernetes.io/name":     app.Name,
		"app.kubernetes.io/instance": app.Name,
	}

	deployment.Name = app.Name
	deployment.Namespace = app.Namespace

	deployment.Spec.Template.Spec = *app.Spec.PodTemplate.Spec.DeepCopy()
	deployment.Spec.Template.ObjectMeta.Labels =
		utils.MergeMapsStringString(
			deployment.Spec.Template.ObjectMeta.Labels,
			app.Spec.MetaTemplate.Labels,
			app.Spec.PodTemplate.ObjectMeta.Labels,
			selectorLabels,
		)

	deployment.Spec.Template.ObjectMeta.Annotations =
		utils.MergeMapsStringString(
			deployment.Spec.Template.ObjectMeta.Annotations,
			app.Spec.MetaTemplate.Annotations,
			app.Spec.PodTemplate.ObjectMeta.Annotations,
		)

	deployment.Spec.Selector = &v1.LabelSelector{
		MatchLabels: selectorLabels,
	}

	logger.Info("Deployment spec", "Deployment", deployment.Name, "Spec", deployment.Spec)

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

// reconcileJobs handles multiple Job resources defined in the Application spec
func (r *ApplicationReconciler) reconcileJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := log.FromContext(ctx)

	for i, job := range app.Spec.Jobs {
		jobName := job.Name
		if jobName == "" {
			jobName = fmt.Sprintf("%s-job-%d", app.Name, i)
		}

		logger.Info("Reconciling Job", "Application", app.Name, "Job", jobName)

		currentJob := &batchv1.Job{}
		err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: jobName}, currentJob)

		// Create or update the job
		if err != nil && client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get Job", "Job", jobName)
			return err
		}

		// Set up the job with the provided spec
		jobToReconcile := job.DeepCopy()
		jobToReconcile.Name = jobName
		jobToReconcile.Namespace = app.Namespace

		// Ensure the job has proper labels
		if jobToReconcile.Labels == nil {
			jobToReconcile.Labels = make(map[string]string)
		}
		jobToReconcile.Labels["app.kubernetes.io/name"] = app.Name
		jobToReconcile.Labels["app.kubernetes.io/instance"] = app.Name
		jobToReconcile.Labels["app.kubernetes.io/managed-by"] = "application-controller"

		if currentJob.CreationTimestamp.IsZero() {
			if err := r.Create(ctx, jobToReconcile); err != nil {
				logger.Error(err, "Failed to create Job", "Job", jobName)
				return err
			}
			logger.Info("Successfully created Job", "Job", jobName)
		} else {
			// Update existing job
			jobToReconcile.ResourceVersion = currentJob.ResourceVersion
			if err := r.Update(ctx, jobToReconcile); err != nil {
				logger.Error(err, "Failed to update Job", "Job", jobName)
				return err
			}
			logger.Info("Successfully updated Job", "Job", jobName)
		}
	}

	return nil
}

// reconcileCronJobs handles multiple CronJob resources defined in the Application spec
func (r *ApplicationReconciler) reconcileCronJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := log.FromContext(ctx)

	for i, cronJob := range app.Spec.CronJobs {
		cronJobName := cronJob.Name
		if cronJobName == "" {
			cronJobName = fmt.Sprintf("%s-cronjob-%d", app.Name, i)
		}

		logger.Info("Reconciling CronJob", "Application", app.Name, "CronJob", cronJobName)

		currentCronJob := &batchv1.CronJob{}
		err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: cronJobName}, currentCronJob)

		// Check if we got an error that's not a NotFound
		if err != nil && client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get CronJob", "CronJob", cronJobName)
			return err
		}

		// Set up the cronjob with the provided spec
		cronJobToReconcile := cronJob.DeepCopy()
		cronJobToReconcile.Name = cronJobName
		cronJobToReconcile.Namespace = app.Namespace

		// Ensure the cronjob has proper labels
		if cronJobToReconcile.Labels == nil {
			cronJobToReconcile.Labels = make(map[string]string)
		}
		cronJobToReconcile.Labels["app.kubernetes.io/name"] = app.Name
		cronJobToReconcile.Labels["app.kubernetes.io/instance"] = app.Name
		cronJobToReconcile.Labels["app.kubernetes.io/managed-by"] = "application-controller"

		if currentCronJob.CreationTimestamp.IsZero() {
			if err := r.Create(ctx, cronJobToReconcile); err != nil {
				logger.Error(err, "Failed to create CronJob", "CronJob", cronJobName)
				return err
			}
			logger.Info("Successfully created CronJob", "CronJob", cronJobName)
		} else {
			// Update existing cronjob
			cronJobToReconcile.ResourceVersion = currentCronJob.ResourceVersion
			if err := r.Update(ctx, cronJobToReconcile); err != nil {
				logger.Error(err, "Failed to update CronJob", "CronJob", cronJobName)
				return err
			}
			logger.Info("Successfully updated CronJob", "CronJob", cronJobName)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wandbv2.Application{}).
		Named("application").
		Complete(r)
}
