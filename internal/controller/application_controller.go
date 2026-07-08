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

	gkeGatewayApiNetworkingv1 "github.com/GoogleCloudPlatform/gke-gateway-api/apis/networking/v1"
	wandbv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	v1alpha1 "github.com/wandb/operator/pkg/vendored/argo-rollouts/argoproj.io.rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
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
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get
// +kubebuilder:rbac:groups=networking.gke.io,resources=healthcheckpolicies,verbs=update;delete;get;list;create;patch;watch

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
	ctx, logger := logx.WithSlog(ctx, logx.ReconcileAppV2)

	var app wandbv2.Application
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error("unable to fetch Application", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
		logger.Info("Application not found. Ignoring since object must be deleted.")
		return ctrl.Result{}, nil
	}

	logger.Info("Handling Application", "Application", app.Name)

	// Add finalizer if it doesn't exist
	if app.DeletionTimestamp == nil {
		if !utils.ContainsString(app.GetFinalizers(), applicationFinalizer) {
			app.SetFinalizers(append(app.GetFinalizers(), applicationFinalizer))
			if err := r.Update(ctx, &app); err != nil {
				logger.Error("Failed to add finalizer", logx.ErrAttr(err))
				return ctrl.Result{}, err
			}
			logger.Info("Added finalizer to Application")
			return ctrl.Result{}, nil
		}
	}

	if app.DeletionTimestamp != nil {
		logger.Info("Application is being deleted")

		// Check if finalizer is present
		if utils.ContainsString(app.GetFinalizers(), applicationFinalizer) {
			// Perform cleanup based on application kind
			switch app.Spec.Kind {
			case "Deployment":
				if err := r.deleteDeployment(ctx, &app); err != nil {
					logger.Error("Failed to delete Deployment during finalization", logx.ErrAttr(err))
					return ctrl.Result{}, err
				}
			case "StatefulSet":
				if err := r.deleteStatefulSet(ctx, &app); err != nil {
					logger.Error("Failed to delete StatefulSet during finalization", logx.ErrAttr(err))
					return ctrl.Result{}, err
				}
			case "Rollout":
				if err := r.deleteRollout(ctx, &app); err != nil {
					logger.Error("Failed to delete Rollout during finalization", logx.ErrAttr(err))
					return ctrl.Result{}, err
				}
			}

			// Delete Service if present
			if err := r.deleteService(ctx, &app); err != nil {
				logger.Error("Failed to delete Service during finalization", logx.ErrAttr(err))
				return ctrl.Result{}, err
			}

			// Delete HPA if present
			if err := r.deleteHPA(ctx, &app); err != nil {
				logger.Error("Failed to delete HPA during finalization", logx.ErrAttr(err))
				return ctrl.Result{}, err
			}

			// Delete Jobs
			if err := r.deleteJobs(ctx, &app); err != nil {
				logger.Error("Failed to delete Jobs during finalization", logx.ErrAttr(err))
				return ctrl.Result{}, err
			}

			// Delete CronJobs
			if err := r.deleteCronJobs(ctx, &app); err != nil {
				logger.Error("Failed to delete CronJobs during finalization", logx.ErrAttr(err))
				return ctrl.Result{}, err
			}

			if err := r.deleteHTTPRoute(ctx, &app); err != nil {
				logger.Error("Failed to delete HTTPRoute during finalization", logx.ErrAttr(err))
				return ctrl.Result{}, err
			}

			// Remove finalizer
			app.SetFinalizers(utils.RemoveString(app.GetFinalizers(), applicationFinalizer))
			if err := r.Update(ctx, &app); err != nil {
				logger.Error("Failed to remove finalizer", logx.ErrAttr(err))
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
		logger.Error("Failed to reconcile Jobs", logx.ErrAttr(err))
		return ctrl.Result{}, err
	}

	// Always reconcile CronJobs regardless of the main application type
	if err := r.reconcileCronJobs(ctx, &app); err != nil {
		logger.Error("Failed to reconcile CronJobs", logx.ErrAttr(err))
		return ctrl.Result{}, err
	}

	// Reconcile Service if specified
	if err := r.reconcileService(ctx, &app); err != nil {
		logger.Error("Failed to reconcile Service", logx.ErrAttr(err))
		return ctrl.Result{}, err
	}

	// Reconcile HPA if specified
	if err := r.reconcileHPA(ctx, &app); err != nil {
		logger.Error("Failed to reconcile HPA", logx.ErrAttr(err))
		return ctrl.Result{}, err
	}

	if err := r.reconcileHTTPRoute(ctx, &app); err != nil {
		logger.Error("Failed to reconcile HTTPRoute", logx.ErrAttr(err))
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
		logger.Error("Failed to update Application status", logx.ErrAttr(err))
		return ctrl.Result{}, err
	}

	return result, nil
}

// reconcileDeployment handles Deployment type applications
func (r *ApplicationReconciler) reconcileDeployment(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := logx.GetSlog(ctx)
	logger.Info("Reconciling Deployment", "Application", app.Name)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, deployment)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error("Failed to get Deployment", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
		logger.Info("Deployment not found", "Deployment", app.Name)
	}

	selectorLabels := getSelectorLabels(app)

	// spec.selector is immutable. If it drifted (e.g. label-standard migration),
	// delete the existing Deployment and recreate it on the next reconcile.
	if !deployment.CreationTimestamp.IsZero() && selectorChanged(deployment.Spec.Selector, selectorLabels) {
		logger.Info("Deployment selector changed; deleting for recreate", "Deployment", app.Name)
		if err := r.Delete(ctx, deployment, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	deployment.Name = app.Name
	deployment.Namespace = app.Namespace

	deployment.Spec.Template.Spec = *app.Spec.PodTemplate.Spec.DeepCopy()
	deployment.Spec.Template.SetLabels(
		utils.MergeMapsStringString(
			deployment.Spec.Template.GetLabels(),
			app.Spec.MetaTemplate.Labels,
			app.Spec.PodTemplate.GetLabels(),
			selectorLabels,
		))

	deployment.Spec.Template.SetAnnotations(
		utils.MergeMapsStringString(
			deployment.Spec.Template.GetAnnotations(),
			app.Spec.MetaTemplate.Annotations,
			app.Spec.PodTemplate.GetAnnotations(),
		))

	deployment.Spec.Selector = &metav1.LabelSelector{
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

	if err = controllerutil.SetControllerReference(app, deployment, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	logger.Debug("Deployment spec", "Deployment", deployment.Name, "Spec", deployment.Spec)

	if deployment.CreationTimestamp.IsZero() {
		if err := r.Create(ctx, deployment); err != nil {
			logger.Error("Failed to create Deployment", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Update(ctx, deployment); err != nil {
			logger.Error("Failed to update Deployment", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
	}

	app.Status.DeploymentStatus = &deployment.Status

	logger.Info("Successfully reconciled Deployment", "Deployment", deployment.Name)
	return ctrl.Result{}, nil
}

// getSelectorLabels returns the immutable label set used for a workload's
// spec.selector. It derives the selector from the operator/ownership family
// (weightsandbiases.apps.wandb.com/*) stamped onto the pod template by the
// WeightsAndBiases reconciler, which is stable and collision-free. Applications
// created before that family existed fall back to the legacy app.kubernetes.io
// selector so their live workloads keep matching.
func getSelectorLabels(app *wandbv2.Application) map[string]string {
	podLabels := app.Spec.PodTemplate.GetLabels()
	if name, ok := podLabels[common.WandbNameLabel]; ok && name != "" {
		selector := map[string]string{common.WandbNameLabel: name}
		if component, ok := podLabels[common.WandbComponentLabel]; ok && component != "" {
			selector[common.WandbComponentLabel] = component
		}
		return selector
	}
	// Legacy fallback (pre-operator-family Applications).
	return map[string]string{
		"app.kubernetes.io/name":     app.Name,
		"app.kubernetes.io/instance": app.Namespace,
	}
}

// recreateOnSelectorChange deletes a workload whose immutable spec.selector no
// longer matches the desired selector so it can be recreated on the next
// reconcile. Returns true when a delete was issued (caller should requeue).
func selectorChanged(current *metav1.LabelSelector, desired map[string]string) bool {
	if current == nil {
		return false
	}
	return !reflect.DeepEqual(current.MatchLabels, desired)
}

// deleteDeployment deletes the Deployment associated with the Application
func (r *ApplicationReconciler) deleteDeployment(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)
	logger.Info("Deleting Deployment", "Application", app.Name)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Deployment not found, nothing to delete", "Deployment", app.Name)
			return nil
		}
		logger.Error("Failed to get Deployment", logx.ErrAttr(err))
		return err
	}

	deletePolicy := client.PropagationPolicy(metav1.DeletePropagationBackground)
	if err := r.Delete(ctx, deployment, deletePolicy); err != nil {
		logger.Error("Failed to delete Deployment", logx.ErrAttr(err), "Deployment", app.Name)
		return err
	}
	logger.Info("Successfully deleted Deployment", "Deployment", app.Name)

	return nil
}

// reconcileRollout handles Rollout type applications
func (r *ApplicationReconciler) reconcileRollout(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := logx.GetSlog(ctx)
	logger.Info("Reconciling Rollout", "Application", app.Name)

	rollout := &v1alpha1.Rollout{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, rollout)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error("Failed to get Rollout", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
		logger.Info("Rollout not found", "Rollout", app.Name)
	}

	selectorLabels := getSelectorLabels(app)

	if !rollout.CreationTimestamp.IsZero() && selectorChanged(rollout.Spec.Selector, selectorLabels) {
		logger.Info("Rollout selector changed; deleting for recreate", "Rollout", app.Name)
		if err := r.Delete(ctx, rollout, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	rollout.Name = app.Name
	rollout.Namespace = app.Namespace

	rollout.Spec.Template.Spec = *app.Spec.PodTemplate.Spec.DeepCopy()
	rollout.Spec.Template.SetLabels(
		utils.MergeMapsStringString(
			rollout.Spec.Template.GetLabels(),
			app.Spec.MetaTemplate.Labels,
			app.Spec.PodTemplate.GetLabels(),
			selectorLabels,
		))

	rollout.Spec.Template.SetAnnotations(
		utils.MergeMapsStringString(
			rollout.Spec.Template.GetAnnotations(),
			app.Spec.MetaTemplate.Annotations,
			app.Spec.PodTemplate.GetAnnotations(),
		))

	rollout.Spec.Selector = &metav1.LabelSelector{
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

	if err = controllerutil.SetControllerReference(app, rollout, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Rollout spec", "Rollout", rollout.Name, "Spec", rollout.Spec)

	if rollout.CreationTimestamp.IsZero() {
		if err := r.Create(ctx, rollout); err != nil {
			logger.Error("Failed to create Rollout", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Update(ctx, rollout); err != nil {
			logger.Error("Failed to update Rollout", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
	}

	app.Status.RolloutStatus = &rollout.Status

	logger.Info("Successfully reconciled Rollout", "Rollout", rollout.Name)
	return ctrl.Result{}, nil
}

// deleteRollout deletes the Rollout associated with the Application
func (r *ApplicationReconciler) deleteRollout(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)
	logger.Info("Deleting Rollout", "Application", app.Name)

	rollout := &v1alpha1.Rollout{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, rollout)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Rollout not found, nothing to delete", "Rollout", app.Name)
			return nil
		}
		logger.Error("Failed to get Rollout", logx.ErrAttr(err))
		return err
	}

	deletePolicy := client.PropagationPolicy(metav1.DeletePropagationBackground)
	if err := r.Delete(ctx, rollout, deletePolicy); err != nil {
		logger.Error("Failed to delete Rollout", logx.ErrAttr(err), "Rollout", app.Name)
		return err
	}
	logger.Info("Successfully deleted Rollout", "Rollout", app.Name)

	return nil
}

// reconcileStatefulSet handles StatefulSet type applications
func (r *ApplicationReconciler) reconcileStatefulSet(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := logx.GetSlog(ctx)
	logger.Info("Reconciling StatefulSet", "Application", app.Name)

	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, statefulSet)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error("Failed to get StatefulSet", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
		logger.Info("StatefulSet not found", "StatefulSet", app.Name)
	}

	selectorLabels := getSelectorLabels(app)

	if !statefulSet.CreationTimestamp.IsZero() && selectorChanged(statefulSet.Spec.Selector, selectorLabels) {
		logger.Info("StatefulSet selector changed; deleting for recreate", "StatefulSet", app.Name)
		if err := r.Delete(ctx, statefulSet, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	statefulSet.Name = app.Name
	statefulSet.Namespace = app.Namespace

	statefulSet.Spec.Template.Spec = *app.Spec.PodTemplate.Spec.DeepCopy()
	statefulSet.Spec.Template.SetLabels(
		utils.MergeMapsStringString(
			statefulSet.Spec.Template.GetLabels(),
			app.Spec.MetaTemplate.Labels,
			app.Spec.PodTemplate.GetLabels(),
			selectorLabels,
		))

	statefulSet.Spec.Template.SetAnnotations(
		utils.MergeMapsStringString(
			statefulSet.Spec.Template.GetAnnotations(),
			app.Spec.MetaTemplate.Annotations,
			app.Spec.PodTemplate.GetAnnotations(),
		))

	statefulSet.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: selectorLabels,
	}

	// ServiceName governs the StatefulSet's stable pod DNS. It is immutable once
	// set, so only apply it at creation time.
	if statefulSet.CreationTimestamp.IsZero() && app.Spec.ServiceName != "" {
		statefulSet.Spec.ServiceName = app.Spec.ServiceName
	}

	// VolumeClaimTemplates are immutable once a StatefulSet exists, so only set
	// them at creation time to avoid update errors from the apiserver.
	if statefulSet.CreationTimestamp.IsZero() && len(app.Spec.VolumeClaimTemplates) > 0 {
		templates := make([]corev1.PersistentVolumeClaim, len(app.Spec.VolumeClaimTemplates))
		for i := range app.Spec.VolumeClaimTemplates {
			templates[i] = *app.Spec.VolumeClaimTemplates[i].DeepCopy()
		}
		statefulSet.Spec.VolumeClaimTemplates = templates
	}

	if app.Spec.HpaTemplate != nil {
		if statefulSet.CreationTimestamp.IsZero() {
			statefulSet.Spec.Replicas = app.Spec.HpaTemplate.MinReplicas
		}
		// Do not update replicas if HPA is managing them
	} else {
		statefulSet.Spec.Replicas = app.Spec.Replicas
	}

	if err = controllerutil.SetControllerReference(app, statefulSet, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("StatefulSet spec", "StatefulSet", statefulSet.Name, "Spec", statefulSet.Spec)

	if statefulSet.CreationTimestamp.IsZero() {
		if err := r.Create(ctx, statefulSet); err != nil {
			logger.Error("Failed to create StatefulSet", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
	} else {
		if err := r.Update(ctx, statefulSet); err != nil {
			logger.Error("Failed to update StatefulSet", logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
	}

	app.Status.StatefulSetStatus = &statefulSet.Status

	logger.Info("Successfully reconciled StatefulSet", "StatefulSet", statefulSet.Name)
	return ctrl.Result{}, nil
}

// deleteStatefulSet deletes the StatefulSet associated with the Application
func (r *ApplicationReconciler) deleteStatefulSet(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)
	logger.Info("Deleting StatefulSet", "Application", app.Name)

	statefulSet := &appsv1.StatefulSet{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, statefulSet)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("StatefulSet not found, nothing to delete", "StatefulSet", app.Name)
			return nil
		}
		logger.Error("Failed to get StatefulSet", logx.ErrAttr(err))
		return err
	}

	deletePolicy := client.PropagationPolicy(metav1.DeletePropagationBackground)
	if err := r.Delete(ctx, statefulSet, deletePolicy); err != nil {
		logger.Error("Failed to delete StatefulSet", logx.ErrAttr(err), "StatefulSet", app.Name)
		return err
	}
	logger.Info("Successfully deleted StatefulSet", "StatefulSet", app.Name)

	return nil
}

// reconcileDaemonSet handles DaemonSet type applications
func (r *ApplicationReconciler) reconcileDaemonSet(ctx context.Context, app *wandbv2.Application) (ctrl.Result, error) {
	logger := logx.GetSlog(ctx)
	logger.Info("Reconciling DaemonSet", "Application", app.Name)
	return ctrl.Result{}, nil
}

// reconcileJobs handles multiple Job resources defined in the Application spec
func (r *ApplicationReconciler) reconcileJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)

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
			logger.Error("Failed to get Job", logx.ErrAttr(err), "Job", jobName)
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
		jobToReconcile.Labels["app.kubernetes.io/instance"] = app.Namespace
		jobToReconcile.Labels["app.kubernetes.io/managed-by"] = "application-controller"

		if err = controllerutil.SetControllerReference(app, jobToReconcile, r.Scheme); err != nil {
			return err
		}

		if currentJob.CreationTimestamp.IsZero() {
			if err := r.Create(ctx, jobToReconcile); err != nil {
				logger.Error("Failed to create Job", logx.ErrAttr(err), "Job", jobName)
				return err
			}
			logger.Info("Successfully created Job", "Job", jobName)
		} else {
			// Jobs cannot be updated, so we need to check if the spec has changed
			// and delete + recreate if necessary
			if !reflect.DeepEqual(currentJob.Spec, jobToReconcile.Spec) {
				logger.Info("Job spec has changed, deleting and recreating", "Job", jobName)

				// Delete the existing job
				deletePolicy := client.PropagationPolicy(metav1.DeletePropagationBackground)
				if err := r.Delete(ctx, currentJob, deletePolicy); err != nil {
					logger.Error("Failed to delete Job", logx.ErrAttr(err), "Job", jobName)
					return err
				}
				logger.Info("Successfully deleted Job", "Job", jobName)

				// Recreate the job with new spec
				if err := r.Create(ctx, jobToReconcile); err != nil {
					logger.Error("Failed to recreate Job", logx.ErrAttr(err), "Job", jobName)
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
	logger := logx.GetSlog(ctx)
	logger.Info("Deleting Jobs", "Application", app.Name)

	jobList := &batchv1.JobList{}
	listOpts := []client.ListOption{
		client.InNamespace(app.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/name":       app.Name,
			"app.kubernetes.io/instance":   app.Namespace,
			"app.kubernetes.io/managed-by": "application-controller",
		},
	}

	if err := r.List(ctx, jobList, listOpts...); err != nil {
		logger.Error("Failed to list Jobs", logx.ErrAttr(err))
		return err
	}

	deletePolicy := client.PropagationPolicy(metav1.DeletePropagationBackground)
	for _, job := range jobList.Items {
		logger.Info("Deleting Job", "Job", job.Name)
		if err := r.Delete(ctx, &job, deletePolicy); err != nil {
			logger.Error("Failed to delete Job", logx.ErrAttr(err), "Job", job.Name)
			return err
		}
		logger.Info("Successfully deleted Job", "Job", job.Name)
	}

	return nil
}

// reconcileCronJobs handles multiple CronJob resources defined in the Application spec
func (r *ApplicationReconciler) reconcileCronJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)

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
			logger.Error("Failed to get CronJob", logx.ErrAttr(err), "CronJob", cronJobName)
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
		cronJobToReconcile.Labels["app.kubernetes.io/instance"] = app.Namespace
		cronJobToReconcile.Labels["app.kubernetes.io/managed-by"] = "application-controller"

		if err = controllerutil.SetControllerReference(app, cronJobToReconcile, r.Scheme); err != nil {
			return err
		}

		if currentCronJob.CreationTimestamp.IsZero() {
			if err := r.Create(ctx, cronJobToReconcile); err != nil {
				logger.Error("Failed to create CronJob", logx.ErrAttr(err), "CronJob", cronJobName)
				return err
			}
			logger.Info("Successfully created CronJob", "CronJob", cronJobName)
		} else {
			// Update existing cronjob
			cronJobToReconcile.ResourceVersion = currentCronJob.ResourceVersion
			if err := r.Update(ctx, cronJobToReconcile); err != nil {
				logger.Error("Failed to update CronJob", logx.ErrAttr(err), "CronJob", cronJobName)
				return err
			}
			logger.Info("Successfully updated CronJob", "CronJob", cronJobName)
		}
	}

	return nil
}

// deleteCronJobs deletes all CronJobs associated with the Application
func (r *ApplicationReconciler) deleteCronJobs(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)
	logger.Info("Deleting CronJobs", "Application", app.Name)

	cronJobList := &batchv1.CronJobList{}
	listOpts := []client.ListOption{
		client.InNamespace(app.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/name":       app.Name,
			"app.kubernetes.io/instance":   app.Namespace,
			"app.kubernetes.io/managed-by": "application-controller",
		},
	}

	if err := r.List(ctx, cronJobList, listOpts...); err != nil {
		logger.Error("Failed to list CronJobs", logx.ErrAttr(err))
		return err
	}

	deletePolicy := client.PropagationPolicy(metav1.DeletePropagationBackground)
	for _, cronJob := range cronJobList.Items {
		logger.Info("Deleting CronJob", "CronJob", cronJob.Name)
		if err := r.Delete(ctx, &cronJob, deletePolicy); err != nil {
			logger.Error("Failed to delete CronJob", logx.ErrAttr(err), "CronJob", cronJob.Name)
			return err
		}
		logger.Info("Successfully deleted CronJob", "CronJob", cronJob.Name)
	}

	return nil
}

// reconcileService ensures a Service exists/updated when specified in the Application spec
func (r *ApplicationReconciler) reconcileService(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)

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
	selectorLabels := getSelectorLabels(app)
	// Merge provided selector with our standard labels
	desired.Spec.Selector = utils.MergeMapsStringString(desired.Spec.Selector, selectorLabels)
	// Also add selector labels to the Service's metadata labels so they are queryable on the Service itself
	desired.Labels = utils.MergeMapsStringString(desired.Labels, selectorLabels)

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return err
	}

	current := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, current)
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Error("Failed to get Service", logx.ErrAttr(err))
			return err
		}
		// Create path
		logger.Info("Creating Service", "Service", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			logger.Error("Failed to create Service", logx.ErrAttr(err))
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
		logger.Error("Failed to update Service", logx.ErrAttr(err))
		return err
	}

	app.Status.ServiceStatus = &current.Status

	logger.Info("Successfully updated Service", "Service", current.Name)
	return nil
}

// deleteService deletes the Service associated with the Application
func (r *ApplicationReconciler) deleteService(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)
	svc := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, svc); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	deletePolicy := client.PropagationPolicy(metav1.DeletePropagationBackground)
	if err := r.Delete(ctx, svc, deletePolicy); err != nil {
		logger.Error("Failed to delete Service", logx.ErrAttr(err), "Service", app.Name)
		return err
	}
	logger.Info("Successfully deleted Service", "Service", app.Name)
	return nil
}

// reconcileHPA handles HorizontalPodAutoscaler resources defined in the Application spec
func (r *ApplicationReconciler) reconcileHPA(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)

	if app.Spec.HpaTemplate == nil {
		// If HPA template is not specified, ensure any existing HPA owned by the Application is deleted
		return r.deleteHPA(ctx, app)
	}

	desired := &autoscalingv2.HorizontalPodAutoscaler{}
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

	desired.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{
		APIVersion: groupVersion,
		Kind:       kind,
		Name:       app.Name,
	}

	if err := controllerutil.SetControllerReference(app, desired, r.Scheme); err != nil {
		return err
	}

	current := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, current)
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Error("Failed to get HPA", logx.ErrAttr(err))
			return err
		}
		// Create path
		logger.Info("Creating HPA", "HPA", desired.Name)
		if err := r.Create(ctx, desired); err != nil {
			logger.Error("Failed to create HPA", logx.ErrAttr(err))
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
		logger.Error("Failed to update HPA", logx.ErrAttr(err))
		return err
	}

	app.Status.HPAStatus = &current.Status
	logger.Info("Successfully updated HPA", "HPA", desired.Name)
	return nil
}

// deleteHPA deletes the HPA associated with the Application
func (r *ApplicationReconciler) deleteHPA(ctx context.Context, app *wandbv2.Application) error {
	logger := logx.GetSlog(ctx)
	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, hpa); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	deletePolicy := client.PropagationPolicy(metav1.DeletePropagationBackground)
	if err := r.Delete(ctx, hpa, deletePolicy); err != nil {
		logger.Error("Failed to delete HPA", logx.ErrAttr(err), "HPA", app.Name)
		return err
	}
	logger.Info("Successfully deleted HPA", "HPA", app.Name)
	return nil
}

func (r *ApplicationReconciler) reconcileHTTPRoute(ctx context.Context, app *wandbv2.Application) error {
	if app.Spec.HTTPRouteTemplate == nil {
		if err := r.deleteHTTPRoute(ctx, app); err != nil {
			return err
		}
		app.Status.HTTPRouteStatus = nil
		return nil
	}

	logger := logx.GetSlog(ctx)

	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
	}
	op, err := ctrl.CreateOrUpdate(ctx, r.Client, httpRoute, func() error {
		httpRoute.Labels = utils.MergeMapsStringString(httpRoute.Labels, app.Spec.MetaTemplate.Labels)
		httpRoute.Annotations = utils.MergeMapsStringString(httpRoute.Annotations, app.Spec.MetaTemplate.Annotations)
		httpRoute.Spec.ParentRefs = app.Spec.HTTPRouteTemplate.ParentRefs
		httpRoute.Spec.Hostnames = app.Spec.HTTPRouteTemplate.Hostnames
		httpRoute.Spec.Rules = buildHTTPRouteRules(app)
		if err := ctrl.SetControllerReference(app, httpRoute, r.Scheme); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	app.Status.HTTPRouteStatus = summarizeHTTPRouteStatus(httpRoute)
	logger.Info(fmt.Sprintf("Successfully %s HTTPRoute", op), "HTTPRoute", httpRoute.Name)

	if len(httpRoute.Status.Parents) > 0 && httpRoute.Status.Parents[0].ControllerName == "networking.gke.io/gateway" {
		healthCheckPolicy := &gkeGatewayApiNetworkingv1.HealthCheckPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app.Name,
				Namespace: app.Namespace,
			},
		}
		op, err = ctrl.CreateOrUpdate(ctx, r.Client, healthCheckPolicy, func() error {
			healthCheckPolicy.Labels = utils.MergeMapsStringString(healthCheckPolicy.Labels, app.Spec.MetaTemplate.Labels)
			healthCheckPolicy.Annotations = utils.MergeMapsStringString(healthCheckPolicy.Annotations, app.Spec.MetaTemplate.Annotations)
			healthCheckPolicy.Spec.Default = &gkeGatewayApiNetworkingv1.HealthCheckPolicyConfig{
				CheckIntervalSec:   ptr.Int64(5),
				TimeoutSec:         ptr.Int64(5),
				UnhealthyThreshold: ptr.Int64(3),
				HealthyThreshold:   ptr.Int64(2),
				Config: &gkeGatewayApiNetworkingv1.HealthCheck{
					Type: gkeGatewayApiNetworkingv1.HTTP,
					HTTP: &gkeGatewayApiNetworkingv1.HTTPHealthCheck{
						CommonHealthCheck: gkeGatewayApiNetworkingv1.CommonHealthCheck{},
						CommonHTTPHealthCheck: gkeGatewayApiNetworkingv1.CommonHTTPHealthCheck{
							RequestPath: ptr.String(app.Spec.PodTemplate.Spec.Containers[0].ReadinessProbe.HTTPGet.Path),
						},
					},
				},
			}
			healthCheckPolicy.Spec.TargetRef = gatewayv1alpha2.NamespacedPolicyTargetReference{
				Kind: "Service",
				Name: gatewayv1alpha2.ObjectName(app.Name),
			}
			if err != nil {
				return err
			}

			if err := ctrl.SetControllerReference(app, healthCheckPolicy, r.Scheme); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			logger.Error("Failed to create or update health check policy", "HealthCheckPolicy", healthCheckPolicy.Name, logx.ErrAttr(err))
			return err
		}
		logger.Info(fmt.Sprintf("Successfully %s HealthCheckPolicy", op), "HealthCheckPolicy", healthCheckPolicy.Name)
	}

	return nil
}

func buildHTTPRouteRules(app *wandbv2.Application) []gatewayv1.HTTPRouteRule {
	tmpl := app.Spec.HTTPRouteTemplate

	paths := tmpl.Paths
	if len(paths) == 0 {
		paths = []string{"/"}
	}

	var matches []gatewayv1.HTTPRouteMatch
	for _, p := range paths {
		p := p
		matchType := gatewayv1.PathMatchPathPrefix
		if tmpl.PathType == "Exact" {
			matchType = gatewayv1.PathMatchExact
		}
		matches = append(matches, gatewayv1.HTTPRouteMatch{
			Path: &gatewayv1.HTTPPathMatch{
				Type:  &matchType,
				Value: &p,
			},
		})
	}

	backendRef := gatewayv1.HTTPBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Name: gatewayv1.ObjectName(app.Name),
				Port: tmpl.ServicePort,
			},
		},
	}

	return []gatewayv1.HTTPRouteRule{{
		Matches:     matches,
		BackendRefs: []gatewayv1.HTTPBackendRef{backendRef},
		Filters: []gatewayv1.HTTPRouteFilter{
			{
				Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
				RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
					Remove: []string{"X-Forwarded-Host", "X-Forwarded-Port"},
				},
			},
		},
	}}
}

func (r *ApplicationReconciler) deleteHTTPRoute(ctx context.Context, app *wandbv2.Application) error {
	route := &gatewayv1.HTTPRoute{}
	if !utils.IsRegistered(r.Scheme, route) {
		return nil
	}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, route); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return r.Delete(ctx, route, client.PropagationPolicy(metav1.DeletePropagationBackground))
}

func summarizeHTTPRouteStatus(route *gatewayv1.HTTPRoute) *wandbv2.HTTPRouteStatusSummary {
	if route == nil {
		return nil
	}

	summary := &wandbv2.HTTPRouteStatusSummary{}
	for _, parent := range route.Status.Parents {
		if apimeta.IsStatusConditionTrue(parent.Conditions, string(gatewayv1.RouteConditionAccepted)) {
			summary.Accepted = true
			break
		}
	}

	return summary
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	controller := ctrl.NewControllerManagedBy(mgr).
		For(&wandbv2.Application{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Named("application")

	if utils.IsRegistered(r.Scheme, &v1alpha1.Rollout{}) {
		controller = controller.Owns(&v1alpha1.Rollout{})
	}

	if utils.IsRegistered(r.Scheme, &gatewayv1.Gateway{}) {
		controller = controller.Owns(&gatewayv1.HTTPRoute{})
	}

	return controller.Complete(r)
}
