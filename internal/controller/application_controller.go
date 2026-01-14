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
	"reflect"

	wandbv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	v1alpha1 "github.com/wandb/operator/pkg/vendored/argo-rollouts/argoproj.io.rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const applicationFinalizer = "applications.apps.wandb.com/finalizer"

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EnableRollouts bool
}

// +kubebuilder:rbac:groups=apps.wandb.com,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.wandb.com,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.wandb.com,resources=applications/finalizers,verbs=update
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

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
	ctx, logger := logx.IntoContext(ctx, logx.ReconcileAppV2)

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

	// Add finalizer if it doesn't exist
	if app.DeletionTimestamp == nil {
		if !utils.ContainsString(app.ObjectMeta.Finalizers, applicationFinalizer) {
			app.ObjectMeta.Finalizers = append(app.ObjectMeta.Finalizers, applicationFinalizer)
			if err := r.Update(ctx, &app); err != nil {
				logger.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
			logger.Info("Added finalizer to Application")
			return ctrl.Result{}, nil
		}
	}

	if app.DeletionTimestamp != nil {
		logger.Info("Application is being deleted")

		// Check if finalizer is present
		if utils.ContainsString(app.ObjectMeta.Finalizers, applicationFinalizer) {
			// Perform cleanup based on application kind
			switch app.Spec.Kind {
			case "Deployment":
				if err := r.deleteDeployment(ctx, &app); err != nil {
					logger.Error(err, "Failed to delete Deployment during finalization")
					return ctrl.Result{}, err
				}
			case "StatefulSet":
				if err := r.deleteStatefulSet(ctx, &app); err != nil {
					logger.Error(err, "Failed to delete StatefulSet during finalization")
					return ctrl.Result{}, err
				}
			case "Rollout":
				if err := r.deleteRollout(ctx, &app); err != nil {
					logger.Error(err, "Failed to delete Rollout during finalization")
					return ctrl.Result{}, err
				}
			}

			// Delete Service if present
			if err := r.deleteService(ctx, &app); err != nil {
				logger.Error(err, "Failed to delete Service during finalization")
				return ctrl.Result{}, err
			}

			// Delete HPA if present
			if err := r.deleteHPA(ctx, &app); err != nil {
				logger.Error(err, "Failed to delete HPA during finalization")
				return ctrl.Result{}, err
			}

			// Delete Jobs
			if err := r.deleteJobs(ctx, &app); err != nil {
				logger.Error(err, "Failed to delete Jobs during finalization")
				return ctrl.Result{}, err
			}

			// Delete CronJobs
			if err := r.deleteCronJobs(ctx, &app); err != nil {
				logger.Error(err, "Failed to delete CronJobs during finalization")
				return ctrl.Result{}, err
			}

			// Remove finalizer
			app.ObjectMeta.Finalizers = utils.RemoveString(app.ObjectMeta.Finalizers, applicationFinalizer)
			if err := r.Update(ctx, &app); err != nil {
				logger.Error(err, "Failed to remove finalizer")
				return ctrl.Result{}, err
			}
			logger.Info("Removed finalizer from Application")
		}

		return ctrl.Result{}, nil
	}

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

	// Reconcile Service if specified
	if err := r.reconcileService(ctx, &app); err != nil {
		logger.Error(err, "Failed to reconcile Service")
		return ctrl.Result{}, err
	}

	// Reconcile HPA if specified
	if err := r.reconcileHPA(ctx, &app); err != nil {
		logger.Error(err, "Failed to reconcile HPA")
		return ctrl.Result{}, err
	}

	app.Status.Ready = false
	if app.Status.DeploymentStatus != nil {
		if app.Status.DeploymentStatus.ReadyReplicas == app.Status.DeploymentStatus.Replicas &&
			app.Status.DeploymentStatus.Replicas > 0 {
			app.Status.Ready = true
		}
	} else if app.Status.StatefulSetStatus != nil {
		if app.Status.StatefulSetStatus.ReadyReplicas == app.Status.StatefulSetStatus.Replicas &&
			app.Status.StatefulSetStatus.Replicas > 0 {
			app.Status.Ready = true
		}
	} else if app.Status.RolloutStatus != nil {
		if app.Status.RolloutStatus.ReadyReplicas == app.Status.RolloutStatus.Replicas &&
			app.Status.RolloutStatus.Replicas > 0 {
			app.Status.Ready = true
		}
	}

	if err := r.Status().Update(ctx, &app); err != nil {
		logger.Error(err, "Failed to update Application status")
		return ctrl.Result{}, err
	}

	return result, nil
}

// reconcileDeployment handles Deployment type applications
func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := logx.FromContext(ctx)
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

	if app.Spec.HpaTemplate != nil {
		if deployment.CreationTimestamp.IsZero() {
			deployment.Spec.Replicas = app.Spec.HpaTemplate.MinReplicas
		}
		// Do not update replicas if HPA is managing them
	} else {
		deployment.Spec.Replicas = app.Spec.Replicas
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

	app.Status.DeploymentStatus = &deployment.Status

	logger.Info("Successfully reconciled Deployment", "Deployment", deployment.Name)
	return ctrl.Result{}, nil
}

// deleteDeployment deletes the Deployment associated with the Application
func (r *ApplicationReconciler) deleteDeployment(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)
	logger.Info("Deleting Deployment", "Application", app.Name)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Deployment not found, nothing to delete", "Deployment", app.Name)
			return nil
		}
		logger.Error(err, "Failed to get Deployment")
		return err
	}

	deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
	if err := r.Delete(ctx, deployment, deletePolicy); err != nil {
		logger.Error(err, "Failed to delete Deployment", "Deployment", app.Name)
		return err
	}
	logger.Info("Successfully deleted Deployment", "Deployment", app.Name)

	return nil
}

// reconcileRollout handles Rollout type applications
func (r *ApplicationReconciler) reconcileRollout(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := logx.FromContext(ctx)
	logger.Info("Reconciling Rollout", "Application", app.Name)

	rollout := &v1alpha1.Rollout{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, rollout)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get Rollout")
			return ctrl.Result{}, err
		}
		logger.Info("Rollout not found", "Rollout", app.Name)
	}

	selectorLabels := map[string]string{
		"app.kubernetes.io/name":     app.Name,
		"app.kubernetes.io/instance": app.Name,
	}

	rollout.Name = app.Name
	rollout.Namespace = app.Namespace

	rollout.Spec.Template.Spec = *app.Spec.PodTemplate.Spec.DeepCopy()
	rollout.Spec.Template.ObjectMeta.Labels =
		utils.MergeMapsStringString(
			rollout.Spec.Template.ObjectMeta.Labels,
			app.Spec.MetaTemplate.Labels,
			app.Spec.PodTemplate.ObjectMeta.Labels,
			selectorLabels,
		)

	rollout.Spec.Template.ObjectMeta.Annotations =
		utils.MergeMapsStringString(
			rollout.Spec.Template.ObjectMeta.Annotations,
			app.Spec.MetaTemplate.Annotations,
			app.Spec.PodTemplate.ObjectMeta.Annotations,
		)

	rollout.Spec.Selector = &v1.LabelSelector{
		MatchLabels: selectorLabels,
	}

	if app.Spec.HpaTemplate != nil {
		if rollout.CreationTimestamp.IsZero() {
			rollout.Spec.Replicas = app.Spec.HpaTemplate.MinReplicas
		}
		// Do not update replicas if HPA is managing them
	} else {
		rollout.Spec.Replicas = app.Spec.Replicas
	}

	logger.Info("Rollout spec", "Rollout", rollout.Name, "Spec", rollout.Spec)

	if rollout.CreationTimestamp.IsZero() {
		if err := r.Create(ctx, rollout); err != nil {
			logger.Error(err, "Failed to create Rollout")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Update(ctx, rollout); err != nil {
			logger.Error(err, "Failed to update Rollout")
			return ctrl.Result{}, err
		}
	}

	app.Status.RolloutStatus = &rollout.Status

	logger.Info("Successfully reconciled Rollout", "Rollout", rollout.Name)
	return ctrl.Result{}, nil
}

// deleteRollout deletes the Rollout associated with the Application
func (r *ApplicationReconciler) deleteRollout(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)
	logger.Info("Deleting Rollout", "Application", app.Name)

	rollout := &v1alpha1.Rollout{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, rollout)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Rollout not found, nothing to delete", "Rollout", app.Name)
			return nil
		}
		logger.Error(err, "Failed to get Rollout")
		return err
	}

	deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
	if err := r.Delete(ctx, rollout, deletePolicy); err != nil {
		logger.Error(err, "Failed to delete Rollout", "Rollout", app.Name)
		return err
	}
	logger.Info("Successfully deleted Rollout", "Rollout", app.Name)

	return nil
}

// reconcileStatefulSet handles StatefulSet type applications
func (r *ApplicationReconciler) reconcileStatefulSet(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := logx.FromContext(ctx)
	logger.Info("Reconciling StatefulSet", "Application", app.Name)

	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, statefulSet)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get StatefulSet")
			return ctrl.Result{}, err
		}
		logger.Info("StatefulSet not found", "StatefulSet", app.Name)
	}

	selectorLabels := map[string]string{
		"app.kubernetes.io/name":     app.Name,
		"app.kubernetes.io/instance": app.Name,
	}

	statefulSet.Name = app.Name
	statefulSet.Namespace = app.Namespace

	statefulSet.Spec.Template.Spec = *app.Spec.PodTemplate.Spec.DeepCopy()
	statefulSet.Spec.Template.ObjectMeta.Labels =
		utils.MergeMapsStringString(
			statefulSet.Spec.Template.ObjectMeta.Labels,
			app.Spec.MetaTemplate.Labels,
			app.Spec.PodTemplate.ObjectMeta.Labels,
			selectorLabels,
		)

	statefulSet.Spec.Template.ObjectMeta.Annotations =
		utils.MergeMapsStringString(
			statefulSet.Spec.Template.ObjectMeta.Annotations,
			app.Spec.MetaTemplate.Annotations,
			app.Spec.PodTemplate.ObjectMeta.Annotations,
		)

	statefulSet.Spec.Selector = &v1.LabelSelector{
		MatchLabels: selectorLabels,
	}

	if app.Spec.HpaTemplate != nil {
		if statefulSet.CreationTimestamp.IsZero() {
			statefulSet.Spec.Replicas = app.Spec.HpaTemplate.MinReplicas
		}
		// Do not update replicas if HPA is managing them
	} else {
		statefulSet.Spec.Replicas = app.Spec.Replicas
	}

	logger.Info("StatefulSet spec", "StatefulSet", statefulSet.Name, "Spec", statefulSet.Spec)

	if statefulSet.CreationTimestamp.IsZero() {
		if err := r.Create(ctx, statefulSet); err != nil {
			logger.Error(err, "Failed to create StatefulSet")
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Update(ctx, statefulSet); err != nil {
			logger.Error(err, "Failed to update StatefulSet")
			return ctrl.Result{}, err
		}
	}

	app.Status.StatefulSetStatus = &statefulSet.Status

	logger.Info("Successfully reconciled StatefulSet", "StatefulSet", statefulSet.Name)
	return ctrl.Result{}, nil
}

// deleteStatefulSet deletes the StatefulSet associated with the Application
func (r *ApplicationReconciler) deleteStatefulSet(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)
	logger.Info("Deleting StatefulSet", "Application", app.Name)

	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, statefulSet)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("StatefulSet not found, nothing to delete", "StatefulSet", app.Name)
			return nil
		}
		logger.Error(err, "Failed to get StatefulSet")
		return err
	}

	deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
	if err := r.Delete(ctx, statefulSet, deletePolicy); err != nil {
		logger.Error(err, "Failed to delete StatefulSet", "StatefulSet", app.Name)
		return err
	}
	logger.Info("Successfully deleted StatefulSet", "StatefulSet", app.Name)

	return nil
}

// reconcileDaemonSet handles DaemonSet type applications
func (r *ApplicationReconciler) reconcileDaemonSet(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := logx.FromContext(ctx)
	logger.Info("Reconciling DaemonSet", "Application", app.Name)
	return ctrl.Result{}, nil
}

// reconcileJobs handles multiple Job resources defined in the Application spec
func (r *ApplicationReconciler) reconcileJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)

	for i, job := range app.Spec.Jobs {
		jobName := job.Name
		if jobName == "" {
			jobName = fmt.Sprintf("%s-job-%d", app.Name, i)
		}

		logger.Info("Reconciling Job", "Application", app.Name, "Job", jobName)

		currentJob := &batchv1.Job{}
		err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: jobName}, currentJob)

		// Create or update the job
		if err != nil && !errors.IsNotFound(err) {
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

		if errors.IsNotFound(err) {
			if err := r.Create(ctx, jobToReconcile); err != nil {
				logger.Error(err, "Failed to create Job", "Job", jobName)
				return err
			}
			logger.Info("Successfully created Job", "Job", jobName)
		} else {
			// Jobs cannot be updated, so we need to check if the spec has changed
			// and delete + recreate if necessary
			if !reflect.DeepEqual(currentJob.Spec, jobToReconcile.Spec) {
				logger.Info("Job spec has changed, deleting and recreating", "Job", jobName)

				// Delete the existing job
				deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
				if err := r.Delete(ctx, currentJob, deletePolicy); err != nil {
					logger.Error(err, "Failed to delete Job", "Job", jobName)
					return err
				}
				logger.Info("Successfully deleted Job", "Job", jobName)

				// Recreate the job with new spec
				if err := r.Create(ctx, jobToReconcile); err != nil {
					logger.Error(err, "Failed to recreate Job", "Job", jobName)
					return err
				}
				logger.Info("Successfully recreated Job", "Job", jobName)
			} else {
				logger.Info("Job spec unchanged, no action needed", "Job", jobName)
			}
		}
	}

	return nil
}

// deleteJobs deletes all Jobs associated with the Application
func (r *ApplicationReconciler) deleteJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)
	logger.Info("Deleting Jobs", "Application", app.Name)

	jobList := &batchv1.JobList{}
	listOpts := []client.ListOption{
		client.InNamespace(app.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/name":       app.Name,
			"app.kubernetes.io/instance":   app.Name,
			"app.kubernetes.io/managed-by": "application-controller",
		},
	}

	if err := r.List(ctx, jobList, listOpts...); err != nil {
		logger.Error(err, "Failed to list Jobs")
		return err
	}

	deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
	for _, job := range jobList.Items {
		logger.Info("Deleting Job", "Job", job.Name)
		if err := r.Delete(ctx, &job, deletePolicy); err != nil {
			logger.Error(err, "Failed to delete Job", "Job", job.Name)
			return err
		}
		logger.Info("Successfully deleted Job", "Job", job.Name)
	}

	return nil
}

// reconcileCronJobs handles multiple CronJob resources defined in the Application spec
func (r *ApplicationReconciler) reconcileCronJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)

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

// deleteCronJobs deletes all CronJobs associated with the Application
func (r *ApplicationReconciler) deleteCronJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)
	logger.Info("Deleting CronJobs", "Application", app.Name)

	cronJobList := &batchv1.CronJobList{}
	listOpts := []client.ListOption{
		client.InNamespace(app.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/name":       app.Name,
			"app.kubernetes.io/instance":   app.Name,
			"app.kubernetes.io/managed-by": "application-controller",
		},
	}

	if err := r.List(ctx, cronJobList, listOpts...); err != nil {
		logger.Error(err, "Failed to list CronJobs")
		return err
	}

	deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
	for _, cronJob := range cronJobList.Items {
		logger.Info("Deleting CronJob", "CronJob", cronJob.Name)
		if err := r.Delete(ctx, &cronJob, deletePolicy); err != nil {
			logger.Error(err, "Failed to delete CronJob", "CronJob", cronJob.Name)
			return err
		}
		logger.Info("Successfully deleted CronJob", "CronJob", cronJob.Name)
	}

	return nil
}

// reconcileService ensures a Service exists/updated when specified in the Application spec
func (r *ApplicationReconciler) reconcileService(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)

	if app.Spec.ServiceTemplate == nil {
		// Nothing to reconcile
		return nil
	}

	desired := &corev1.Service{}
	desired.Name = app.Name
	desired.Namespace = app.Namespace

	// Merge labels/annotations from meta template
	desired.Labels = utils.MergeMapsStringString(
		desired.Labels,
		app.Spec.MetaTemplate.Labels,
	)
	desired.Annotations = utils.MergeMapsStringString(
		desired.Annotations,
		app.Spec.MetaTemplate.Annotations,
	)

	// Copy spec from template
	desired.Spec = *app.Spec.ServiceTemplate.DeepCopy()

	// Ensure selector targets the application's pods
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":     app.Name,
		"app.kubernetes.io/instance": app.Name,
	}
	// Merge provided selector with our standard labels
	desired.Spec.Selector = utils.MergeMapsStringString(desired.Spec.Selector, selectorLabels)
	// Also add selector labels to the Service's metadata labels so they are queryable on the Service itself
	desired.Labels = utils.MergeMapsStringString(desired.Labels, selectorLabels)

	current := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, current)
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get Service")
			return err
		}
		// Create path
		logger.Info("Creating Service", "Service", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			logger.Error(err, "Failed to create Service")
			return err
		}

		app.Status.ServiceStatus = &desired.Status

		logger.Info("Successfully created Service", "Service", desired.Name)
		return nil
	}

	// Update path: preserve immutable fields
	desired.ResourceVersion = current.ResourceVersion
	desired.Spec.ClusterIP = current.Spec.ClusterIP
	desired.Spec.ClusterIPs = current.Spec.ClusterIPs
	desired.Spec.IPFamilies = current.Spec.IPFamilies
	desired.Spec.IPFamilyPolicy = current.Spec.IPFamilyPolicy
	desired.Spec.HealthCheckNodePort = current.Spec.HealthCheckNodePort

	// Only update if there are changes
	// Apply desired into current to minimize overwrite
	current.Labels = desired.Labels
	current.Annotations = desired.Annotations
	current.Spec.Ports = desired.Spec.Ports
	current.Spec.Type = desired.Spec.Type
	current.Spec.Selector = desired.Spec.Selector
	current.Spec.SessionAffinity = desired.Spec.SessionAffinity
	current.Spec.ExternalTrafficPolicy = desired.Spec.ExternalTrafficPolicy
	current.Spec.InternalTrafficPolicy = desired.Spec.InternalTrafficPolicy
	current.Spec.LoadBalancerClass = desired.Spec.LoadBalancerClass
	current.Spec.AllocateLoadBalancerNodePorts = desired.Spec.AllocateLoadBalancerNodePorts
	current.Spec.ExternalIPs = desired.Spec.ExternalIPs
	current.Spec.LoadBalancerIP = desired.Spec.LoadBalancerIP
	current.Spec.LoadBalancerSourceRanges = desired.Spec.LoadBalancerSourceRanges

	logger.Info("Updating Service", "Service", current.Name)
	if err := r.Update(ctx, current); err != nil {
		logger.Error(err, "Failed to update Service")
		return err
	}

	app.Status.ServiceStatus = &current.Status

	logger.Info("Successfully updated Service", "Service", current.Name)
	return nil
}

// deleteService deletes the Service associated with the Application
func (r *ApplicationReconciler) deleteService(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)
	svc := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, svc); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
	if err := r.Delete(ctx, svc, deletePolicy); err != nil {
		logger.Error(err, "Failed to delete Service", "Service", app.Name)
		return err
	}
	logger.Info("Successfully deleted Service", "Service", app.Name)
	return nil
}

// reconcileHPA handles HorizontalPodAutoscaler resources defined in the Application spec
func (r *ApplicationReconciler) reconcileHPA(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)

	if app.Spec.HpaTemplate == nil {
		// If HPA template is not specified, ensure any existing HPA owned by the Application is deleted
		return r.deleteHPA(ctx, app)
	}

	desired := &autoscalingv1.HorizontalPodAutoscaler{}
	desired.Name = app.Name
	desired.Namespace = app.Namespace

	// Set owner reference
	if err := ctrl.SetControllerReference(app, desired, r.Scheme); err != nil {
		return err
	}

	// Merge labels/annotations from meta template
	desired.Labels = utils.MergeMapsStringString(
		desired.Labels,
		app.Spec.MetaTemplate.Labels,
	)
	desired.Annotations = utils.MergeMapsStringString(
		desired.Annotations,
		app.Spec.MetaTemplate.Annotations,
	)

	// Copy spec from template
	desired.Spec = *app.Spec.HpaTemplate.DeepCopy()

	// Ensure scaleTargetRef points to our application workload
	var groupVersion string
	var kind string
	switch app.Spec.Kind {
	case "Deployment":
		groupVersion = appsv1.SchemeGroupVersion.String()
		kind = "Deployment"
	case "StatefulSet":
		groupVersion = appsv1.SchemeGroupVersion.String()
		kind = "StatefulSet"
	case "Rollout":
		groupVersion = v1alpha1.SchemeGroupVersion.String()
		kind = "Rollout"
	default:
		return fmt.Errorf("unsupported application kind for HPA: %s", app.Spec.Kind)
	}

	desired.Spec.ScaleTargetRef = autoscalingv1.CrossVersionObjectReference{
		APIVersion: groupVersion,
		Kind:       kind,
		Name:       app.Name,
	}

	current := &autoscalingv1.HorizontalPodAutoscaler{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, current)
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get HPA")
			return err
		}
		// Create path
		logger.Info("Creating HPA", "HPA", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			logger.Error(err, "Failed to create HPA")
			return err
		}

		app.Status.HPAStatus = &desired.Status
		logger.Info("Successfully created HPA", "HPA", desired.Name)
		return nil
	}

	// Update path
	desired.ResourceVersion = current.ResourceVersion
	logger.Info("Updating HPA", "HPA", desired.Name)
	if err := r.Update(ctx, desired); err != nil {
		logger.Error(err, "Failed to update HPA")
		return err
	}

	app.Status.HPAStatus = &current.Status
	logger.Info("Successfully updated HPA", "HPA", desired.Name)
	return nil
}

// deleteHPA deletes the HPA associated with the Application
func (r *ApplicationReconciler) deleteHPA(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.FromContext(ctx)
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, hpa); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	deletePolicy := client.PropagationPolicy(v1.DeletePropagationBackground)
	if err := r.Delete(ctx, hpa, deletePolicy); err != nil {
		logger.Error(err, "Failed to delete HPA", "HPA", app.Name)
		return err
	}
	logger.Info("Successfully deleted HPA", "HPA", app.Name)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller := ctrl.NewControllerManagedBy(mgr).
		For(&wandbv2.Application{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv1.HorizontalPodAutoscaler{}).
		Named("application")

	if r.EnableRollouts {
		controller = controller.Owns(&v1alpha1.Rollout{})
	}

	return controller.Complete(r)
}
