package reconciler

import (
	"context"
	"fmt"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/internal/observability/supportbundle"
	"github.com/wandb/operator/internal/observability/telemetry"
	"github.com/wandb/operator/pkg/utils"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// The operator must hold everything it grants the support-bundle ServiceAccount.
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingadmissionpolicies;validatingadmissionpolicybindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=create;delete
//+kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list
//+kubebuilder:rbac:groups="",resources=limitranges;persistentvolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch
//+kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs=get;list

const supportBundleActiveJobRequeue = time.Minute

// reconcileSupportBundle never fails the installation
func reconcileSupportBundle(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	telemetryConfig telemetry.TelemetryRuntimeConfig,
) ctrl.Result {
	log := logx.GetSlog(ctx)
	statusBefore := wandb.DeepCopy().Status

	var result ctrl.Result
	var err error
	if supportbundle.Enabled(wandb) {
		result, err = applySupportBundle(ctx, c, wandb, telemetryConfig)
	} else {
		result, err = deleteSupportBundle(ctx, c, wandb, true)
	}
	if err != nil {
		log.Error("Failed to reconcile support bundle", logx.ErrAttr(err))
		result.RequeueAfter = defaultRequeueDuration
	}
	if err := updateWandbStatusIfChanged(ctx, c, wandb, statusBefore); err != nil {
		log.Error("Failed to update support bundle status", logx.ErrAttr(err))
		result.RequeueAfter = defaultRequeueDuration
	}
	return result
}

func applySupportBundle(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	telemetryConfig telemetry.TelemetryRuntimeConfig,
) (ctrl.Result, error) {
	status := &apiv2.SupportBundleStatusSummary{
		CronJob: supportbundle.CronJobName(wandb),
		Version: supportbundle.Version(wandb),
		Image:   supportbundle.Image(wandb),
	}
	wandb.Status.SupportBundle = status

	if err := applySupportBundlePodPolicy(ctx, c, wandb); meta.IsNoMatchError(err) {
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	bucket, err := supportbundle.DefaultBucketConnection(wandb)
	if err != nil {
		logx.GetSlog(ctx).Info("Waiting for the object store before installing the support bundle", "reason", err.Error())
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	configMap, err := supportbundle.ConfigMap(wandb, telemetryConfig)
	if err != nil {
		return ctrl.Result{}, err
	}
	currentConfigMap := &corev1.ConfigMap{ObjectMeta: objectKeyMeta(configMap)}
	if err := upsertSupportBundleObject(ctx, c, wandb, currentConfigMap, configMap, true, func() {
		currentConfigMap.Data = configMap.Data
	}); err != nil {
		return ctrl.Result{}, err
	}

	serviceAccount := supportbundle.ServiceAccount(wandb)
	currentServiceAccount := &corev1.ServiceAccount{ObjectMeta: objectKeyMeta(serviceAccount)}
	if err := upsertSupportBundleObject(ctx, c, wandb, currentServiceAccount, serviceAccount, true, func() {
		currentServiceAccount.Annotations = utils.MergeMapsStringString(currentServiceAccount.Annotations, serviceAccount.Annotations)
	}); err != nil {
		return ctrl.Result{}, err
	}

	if err := applySupportBundleRBAC(ctx, c, wandb, telemetryConfig); err != nil {
		return ctrl.Result{}, err
	}

	application := supportbundle.Application(wandb, bucket)
	currentApplication := &apiv2.Application{ObjectMeta: objectKeyMeta(application)}
	if err := upsertSupportBundleObject(ctx, c, wandb, currentApplication, application, true, func() {
		// The CRD schema defaults fields we leave unset, so only compare what we set.
		if !apiequality.Semantic.DeepDerivative(application.Spec, currentApplication.Spec) {
			currentApplication.Spec = application.Spec
		}
	}); err != nil {
		return ctrl.Result{}, err
	}

	current, err := supportBundleCronJobCurrent(ctx, c, currentApplication, application.Spec.CronJobs[0])
	if err != nil {
		return ctrl.Result{}, err
	}
	status.Ready = currentApplication.Status.Ready && current
	if !status.Ready {
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}
	return ctrl.Result{}, nil
}

// supportBundleCronJobCurrent reports whether the Application controller has rolled the template into the CronJob
func supportBundleCronJobCurrent(ctx context.Context, c ctrlClient.Client, app *apiv2.Application, desired batchv1.CronJob) (bool, error) {
	cronJob := &batchv1.CronJob{}
	if err := c.Get(ctx, ctrlClient.ObjectKey{Namespace: app.Namespace, Name: desired.Name}, cronJob); err != nil {
		return false, ctrlClient.IgnoreNotFound(err)
	}
	return metav1.IsControlledBy(cronJob, app) && apiequality.Semantic.DeepDerivative(desired.Spec, cronJob.Spec), nil
}

func applySupportBundlePodPolicy(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	policy := supportbundle.PodAdmissionPolicy(wandb)
	currentPolicy := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: objectKeyMeta(policy)}
	if err := upsertSupportBundleObject(ctx, c, wandb, currentPolicy, policy, false, func() {
		if !apiequality.Semantic.DeepDerivative(policy.Spec, currentPolicy.Spec) {
			currentPolicy.Spec = policy.Spec
		}
	}); err != nil {
		return err
	}
	policyBinding := supportbundle.PodAdmissionPolicyBinding(wandb)
	currentPolicyBinding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: objectKeyMeta(policyBinding)}
	return upsertSupportBundleObject(ctx, c, wandb, currentPolicyBinding, policyBinding, false, func() {
		if !apiequality.Semantic.DeepDerivative(policyBinding.Spec, currentPolicyBinding.Spec) {
			currentPolicyBinding.Spec = policyBinding.Spec
		}
	})
}

func applySupportBundleRBAC(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	telemetryConfig telemetry.TelemetryRuntimeConfig,
) error {
	desiredRoles := map[ctrlClient.ObjectKey]bool{}
	for _, role := range supportbundle.Roles(wandb, telemetryConfig) {
		// Owner references cannot cross namespaces, so a Role elsewhere is tracked by labels.
		controlled := role.Namespace == wandb.Namespace
		desiredRoles[ctrlClient.ObjectKeyFromObject(role)] = true

		currentRole := &rbacv1.Role{ObjectMeta: objectKeyMeta(role)}
		if err := upsertSupportBundleObject(ctx, c, wandb, currentRole, role, controlled, func() {
			currentRole.Rules = role.Rules
		}); err != nil {
			return err
		}

		binding := supportbundle.RoleBinding(wandb, role)
		currentBinding := &rbacv1.RoleBinding{ObjectMeta: objectKeyMeta(binding)}
		if err := upsertSupportBundleObject(ctx, c, wandb, currentBinding, binding, controlled, func() {
			currentBinding.RoleRef = binding.RoleRef
			currentBinding.Subjects = binding.Subjects
		}); err != nil {
			return err
		}
	}
	if err := deleteSupportBundleRoles(ctx, c, wandb, desiredRoles); err != nil {
		return err
	}

	clusterRole := supportbundle.ClusterRole(wandb)
	currentClusterRole := &rbacv1.ClusterRole{ObjectMeta: objectKeyMeta(clusterRole)}
	if err := upsertSupportBundleObject(ctx, c, wandb, currentClusterRole, clusterRole, false, func() {
		currentClusterRole.Rules = clusterRole.Rules
	}); err != nil {
		return err
	}

	clusterBinding := supportbundle.ClusterRoleBinding(wandb)
	currentClusterBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: objectKeyMeta(clusterBinding)}
	return upsertSupportBundleObject(ctx, c, wandb, currentClusterBinding, clusterBinding, false, func() {
		currentClusterBinding.RoleRef = clusterBinding.RoleRef
		currentClusterBinding.Subjects = clusterBinding.Subjects
	})
}

// deleteSupportBundle waits for running collections when disabling, but stops them on install deletion so the finalizer cannot hang.
func deleteSupportBundle(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases, waitForJobs bool) (ctrl.Result, error) {
	jobs := &batchv1.JobList{}
	if err := c.List(ctx, jobs, ctrlClient.InNamespace(wandb.Namespace), ctrlClient.MatchingLabels(supportbundle.SelectorLabels(wandb))); err != nil {
		return ctrl.Result{}, err
	}
	for i := range jobs.Items {
		job := &jobs.Items[i]
		if waitForJobs {
			if !jobFinished(job) {
				logx.GetSlog(ctx).Info("Keeping support bundle resources until a running collection finishes", "job", job.Name)
				return ctrl.Result{RequeueAfter: supportBundleActiveJobRequeue}, nil
			}
			continue
		}
		if err := ctrlClient.IgnoreNotFound(c.Delete(ctx, job, ctrlClient.PropagationPolicy(metav1.DeletePropagationBackground))); err != nil {
			return ctrl.Result{}, err
		}
	}

	name := supportbundle.Name(wandb)
	namespaced := []ctrlClient.Object{
		&apiv2.Application{Name: name, Namespace: wandb.Namespace},
		&corev1.ConfigMap{Name: name, Namespace: wandb.Namespace},
		&corev1.ServiceAccount{Name: name, Namespace: wandb.Namespace},
	}
	for _, obj := range namespaced {
		if err := deleteSupportBundleObject(ctx, c, wandb, obj, true); err != nil {
			return ctrl.Result{}, err
		}
	}
	if err := deleteSupportBundleRoles(ctx, c, wandb, nil); err != nil {
		return ctrl.Result{}, err
	}
	clusterName := supportbundle.ClusterScopedName(wandb)
	for _, obj := range []ctrlClient.Object{
		&rbacv1.ClusterRoleBinding{Name: clusterName},
		&rbacv1.ClusterRole{Name: clusterName},
		&admissionregistrationv1.ValidatingAdmissionPolicyBinding{Name: clusterName},
		&admissionregistrationv1.ValidatingAdmissionPolicy{Name: clusterName},
	} {
		if err := deleteSupportBundleObject(ctx, c, wandb, obj, false); err != nil {
			return ctrl.Result{}, err
		}
	}

	wandb.Status.SupportBundle = nil
	return ctrl.Result{}, nil
}

// deleteSupportBundleRoles removes labeled Roles and RoleBindings in any namespace that are not in keep.
func deleteSupportBundleRoles(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases, keep map[ctrlClient.ObjectKey]bool) error {
	selector := ctrlClient.MatchingLabels(supportbundle.SelectorLabels(wandb))
	roles := &rbacv1.RoleList{}
	if err := c.List(ctx, roles, selector); err != nil {
		return err
	}
	bindings := &rbacv1.RoleBindingList{}
	if err := c.List(ctx, bindings, selector); err != nil {
		return err
	}
	var stale []ctrlClient.Object
	for i := range bindings.Items {
		stale = append(stale, &bindings.Items[i])
	}
	for i := range roles.Items {
		stale = append(stale, &roles.Items[i])
	}
	for _, obj := range stale {
		if keep[ctrlClient.ObjectKeyFromObject(obj)] {
			continue
		}
		if err := ctrlClient.IgnoreNotFound(c.Delete(ctx, obj)); err != nil {
			return err
		}
	}
	return nil
}

func upsertSupportBundleObject(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	current, desired ctrlClient.Object,
	controlled bool,
	mutate func(),
) error {
	_, err := controllerutil.CreateOrUpdate(ctx, c, current, func() error {
		if current.GetResourceVersion() != "" && !managedBySupportBundle(wandb, current, controlled) {
			return fmt.Errorf("%T %s already exists and is not managed by the support bundle for %s/%s",
				current, ctrlClient.ObjectKeyFromObject(current), wandb.Namespace, wandb.Name)
		}
		mutate()
		current.SetLabels(utils.MergeMapsStringString(current.GetLabels(), desired.GetLabels()))
		if controlled {
			return controllerutil.SetControllerReference(wandb, current, c.Scheme())
		}
		return nil
	})
	return err
}

func deleteSupportBundleObject(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases, obj ctrlClient.Object, controlled bool) error {
	if err := c.Get(ctx, ctrlClient.ObjectKeyFromObject(obj), obj); err != nil {
		return ctrlClient.IgnoreNotFound(err)
	}
	if !managedBySupportBundle(wandb, obj, controlled) {
		return nil
	}
	return ctrlClient.IgnoreNotFound(c.Delete(ctx, obj, ctrlClient.PropagationPolicy(metav1.DeletePropagationBackground)))
}

// managedBySupportBundle requires our controller reference, or our labels where owner references are impossible.
func managedBySupportBundle(wandb *apiv2.WeightsAndBiases, obj ctrlClient.Object, controlled bool) bool {
	if controlled {
		return metav1.IsControlledBy(obj, wandb)
	}
	labels := obj.GetLabels()
	for key, value := range supportbundle.SelectorLabels(wandb) {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func jobFinished(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if (condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed) && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func objectKeyMeta(obj ctrlClient.Object) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}
