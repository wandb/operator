package reconciler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/observability/supportbundle"
	"github.com/wandb/operator/internal/observability/telemetry"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	supportBundleTestNamespace = "sb-install"
	supportBundleTelemetryNS   = "sb-telemetry"
)

func supportBundleEnvtest(t *testing.T) ctrlClient.WithWatch {
	t.Helper()
	c, _ := supportBundleEnvtestWithConfig(t)
	return c
}

func supportBundleEnvtestWithConfig(t *testing.T) (ctrlClient.WithWatch, *rest.Config) {
	t.Helper()
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		dirs, _ := filepath.Glob(filepath.Join("..", "..", "..", "bin", "k8s", "*"))
		if len(dirs) == 0 {
			t.Skip("envtest binaries not found; run make setup-envtest")
		}
		assets = dirs[0]
	}

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: assets,
		Scheme:                scheme,
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, env.Stop()) })

	c, err := ctrlClient.NewWithWatch(cfg, ctrlClient.Options{Scheme: scheme})
	require.NoError(t, err)
	for _, ns := range []string{supportBundleTestNamespace, supportBundleTelemetryNS, "sb-elsewhere"} {
		require.NoError(t, c.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}))
	}
	return c, cfg
}

func createSupportBundleWandb(t *testing.T, c ctrlClient.Client, name string) *apiv2.WeightsAndBiases {
	t.Helper()
	ctx := context.Background()
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: supportBundleTestNamespace}}
	wandb.Spec.Wandb.Hostname = "https://wandb.example.com"
	wandb.Spec.Wandb.Features = map[string]bool{}
	wandb.Spec.SupportBundle.Enabled = ptr.To(true)
	require.NoError(t, c.Create(ctx, wandb))

	wandb.Status.TelemetryStatus.Ready = true
	wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{
		apiv2.DefaultInstanceName: {Connection: apiv2.ObjectStoreConnection{
			URL: apiv2.ValueOrSecret{ValueFrom: &apiv2.SecretValueSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: name + "-objectstore-connection"},
				Key:                  "url",
			}}},
		}},
	}
	require.NoError(t, c.Status().Update(ctx, wandb))
	return wandb
}

var supportBundleTelemetry = telemetry.TelemetryRuntimeConfig{Enabled: true, Mode: "full", Namespace: supportBundleTelemetryNS}

type supportBundleObjects struct {
	application        *apiv2.Application
	configMap          *corev1.ConfigMap
	serviceAccount     *corev1.ServiceAccount
	role               *rbacv1.Role
	roleBinding        *rbacv1.RoleBinding
	telemetryRole      *rbacv1.Role
	telemetryBinding   *rbacv1.RoleBinding
	dnsTestRole        *rbacv1.Role
	dnsTestBinding     *rbacv1.RoleBinding
	dnsSystemRole      *rbacv1.Role
	dnsSystemBinding   *rbacv1.RoleBinding
	clusterRole        *rbacv1.ClusterRole
	clusterRoleBinding *rbacv1.ClusterRoleBinding
	podPolicy          *admissionregistrationv1.ValidatingAdmissionPolicy
	podPolicyBinding   *admissionregistrationv1.ValidatingAdmissionPolicyBinding
}

func (o supportBundleObjects) all() []ctrlClient.Object {
	return []ctrlClient.Object{
		o.application, o.configMap, o.serviceAccount, o.role, o.roleBinding,
		o.telemetryRole, o.telemetryBinding, o.dnsTestRole, o.dnsTestBinding, o.dnsSystemRole, o.dnsSystemBinding,
		o.clusterRole, o.clusterRoleBinding, o.podPolicy, o.podPolicyBinding,
	}
}

func supportBundleObjectKeys(wandb *apiv2.WeightsAndBiases) supportBundleObjects {
	name := supportbundle.Name(wandb)
	clusterName := supportbundle.ClusterScopedName(wandb)
	ns := metav1.ObjectMeta{Name: name, Namespace: wandb.Namespace}
	telemetryNS := metav1.ObjectMeta{Name: clusterName, Namespace: supportBundleTelemetryNS}
	dnsTestNS := metav1.ObjectMeta{Name: clusterName, Namespace: "default"}
	dnsSystemNS := metav1.ObjectMeta{Name: clusterName, Namespace: "kube-system"}
	return supportBundleObjects{
		application:        &apiv2.Application{ObjectMeta: ns},
		configMap:          &corev1.ConfigMap{ObjectMeta: ns},
		serviceAccount:     &corev1.ServiceAccount{ObjectMeta: ns},
		role:               &rbacv1.Role{ObjectMeta: ns},
		roleBinding:        &rbacv1.RoleBinding{ObjectMeta: ns},
		telemetryRole:      &rbacv1.Role{ObjectMeta: telemetryNS},
		telemetryBinding:   &rbacv1.RoleBinding{ObjectMeta: telemetryNS},
		dnsTestRole:        &rbacv1.Role{ObjectMeta: dnsTestNS},
		dnsTestBinding:     &rbacv1.RoleBinding{ObjectMeta: dnsTestNS},
		dnsSystemRole:      &rbacv1.Role{ObjectMeta: dnsSystemNS},
		dnsSystemBinding:   &rbacv1.RoleBinding{ObjectMeta: dnsSystemNS},
		clusterRole:        &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		clusterRoleBinding: &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		podPolicy:          &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		podPolicyBinding:   &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
	}
}

func requireAllPresent(t *testing.T, c ctrlClient.Client, objects supportBundleObjects) {
	t.Helper()
	for _, obj := range objects.all() {
		require.NoError(t, c.Get(context.Background(), ctrlClient.ObjectKeyFromObject(obj), obj), "%T %s", obj, obj.GetName())
	}
}

func requireAllAbsent(t *testing.T, c ctrlClient.Client, objects supportBundleObjects) {
	t.Helper()
	for _, obj := range objects.all() {
		err := c.Get(context.Background(), ctrlClient.ObjectKeyFromObject(obj), obj)
		require.True(t, apierrors.IsNotFound(err), "%T %s should be deleted, got %v", obj, obj.GetName(), err)
	}
}

func launchSupportBundleJob(t *testing.T, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases, name string) *batchv1.Job {
	t.Helper()
	app := &apiv2.Application{}
	require.NoError(t, c.Get(context.Background(), ctrlClient.ObjectKey{Name: supportbundle.Name(wandb), Namespace: wandb.Namespace}, app))
	template := app.Spec.CronJobs[0].Spec.JobTemplate
	job := &batchv1.Job{
		Name: name, Namespace: wandb.Namespace, Labels: template.Labels,
		Spec: *template.Spec.DeepCopy(),
	}
	require.NoError(t, c.Create(context.Background(), job))
	return job
}

func failJob(t *testing.T, c ctrlClient.Client, job *batchv1.Job) {
	t.Helper()
	now := metav1.Now()
	job.Status.StartTime = &now
	job.Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, LastTransitionTime: now},
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: now},
	}
	require.NoError(t, c.Status().Update(context.Background(), job))
}

func TestSupportBundleLifecycle(t *testing.T) {
	ctx := context.Background()
	c := supportBundleEnvtest(t)
	wandb := createSupportBundleWandb(t, c, "wandb")
	objects := supportBundleObjectKeys(wandb)

	t.Run("installs the suspended template and supporting resources", func(t *testing.T) {
		result := reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.Equal(t, defaultRequeueDuration, result.RequeueAfter, "not ready until the CronJob exists")
		requireAllPresent(t, c, objects)

		require.Equal(t, "CronJob", objects.application.Spec.Kind)
		require.True(t, metav1.IsControlledBy(objects.application, wandb))
		require.True(t, *objects.application.Spec.CronJobs[0].Spec.Suspend)
		require.Contains(t, objects.configMap.Data, "full.yaml")
		require.Contains(t, objects.configMap.Data["full.yaml"], "victoriaStack")
		require.Len(t, objects.telemetryRole.OwnerReferences, 0)
		require.Len(t, objects.dnsSystemRole.OwnerReferences, 0)
		require.Contains(t, objects.configMap.Data["full.yaml"], "dns:")
		require.Len(t, objects.clusterRole.OwnerReferences, 0)

		jobs := &batchv1.JobList{}
		require.NoError(t, c.List(ctx, jobs, ctrlClient.InNamespace(wandb.Namespace)))
		require.Empty(t, jobs.Items, "installing the template must not launch a collection")

		stored := &apiv2.WeightsAndBiases{}
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(wandb), stored))
		require.Equal(t, &apiv2.SupportBundleStatusSummary{
			CronJob: "wandb-support-bundle-collector",
			Version: "0.4.10",
			Image:   "us-docker.pkg.dev/wandb-production/public/wandb/lumen:0.4.10",
		}, stored.Status.SupportBundle)

		simulateApplicationController(t, c, wandb)
		result = reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.Zero(t, result.RequeueAfter)
		require.True(t, wandb.Status.SupportBundle.Ready)
	})

	t.Run("is quiet after creation", func(t *testing.T) {
		writes := 0
		counting := interceptor.NewClient(c, interceptor.Funcs{
			Create: func(ctx context.Context, cl ctrlClient.WithWatch, obj ctrlClient.Object, opts ...ctrlClient.CreateOption) error {
				writes++
				return cl.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, cl ctrlClient.WithWatch, obj ctrlClient.Object, opts ...ctrlClient.UpdateOption) error {
				writes++
				return cl.Update(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, cl ctrlClient.WithWatch, obj ctrlClient.Object, patch ctrlClient.Patch, opts ...ctrlClient.PatchOption) error {
				writes++
				return cl.Patch(ctx, obj, patch, opts...)
			},
			Delete: func(ctx context.Context, cl ctrlClient.WithWatch, obj ctrlClient.Object, opts ...ctrlClient.DeleteOption) error {
				writes++
				return cl.Delete(ctx, obj, opts...)
			},
			SubResourceUpdate: func(ctx context.Context, cl ctrlClient.Client, sub string, obj ctrlClient.Object, opts ...ctrlClient.SubResourceUpdateOption) error {
				writes++
				return cl.SubResource(sub).Update(ctx, obj, opts...)
			},
		})

		requireAllPresent(t, c, objects)
		versions := map[string]string{}
		for _, obj := range objects.all() {
			versions[fmt.Sprintf("%T/%s/%s", obj, obj.GetNamespace(), obj.GetName())] = obj.GetResourceVersion()
		}
		for i := 0; i < 3; i++ {
			reconcileSupportBundle(ctx, counting, wandb, supportBundleTelemetry)
		}
		require.Zero(t, writes, "steady-state reconciles must not write")
		requireAllPresent(t, c, objects)
		for _, obj := range objects.all() {
			require.Equal(t, versions[fmt.Sprintf("%T/%s/%s", obj, obj.GetNamespace(), obj.GetName())], obj.GetResourceVersion())
		}
	})

	t.Run("updates the template when the version changes", func(t *testing.T) {
		wandb.Spec.SupportBundle.Version = "0.4.9"
		require.NoError(t, c.Update(ctx, wandb))
		reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(objects.application), objects.application))
		cron := objects.application.Spec.CronJobs[0]
		require.Equal(t, "us-docker.pkg.dev/wandb-production/public/wandb/lumen:0.4.9", cron.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image)
		require.Equal(t, "0.4.9", cron.Labels["app.kubernetes.io/version"])
		require.True(t, *cron.Spec.Suspend)

		require.False(t, wandb.Status.SupportBundle.Ready, "the CronJob still runs 0.4.10")
		simulateApplicationController(t, c, wandb)
		reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.True(t, wandb.Status.SupportBundle.Ready)
	})

	t.Run("is not ready while the CronJob drifts from the template", func(t *testing.T) {
		cronJob := &batchv1.CronJob{}
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKey{Name: supportbundle.CronJobName(wandb), Namespace: wandb.Namespace}, cronJob))
		cronJob.Spec.Suspend = ptr.To(false)
		require.NoError(t, c.Update(ctx, cronJob))

		result := reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.False(t, wandb.Status.SupportBundle.Ready, "an unsuspended CronJob still has the right image")
		require.Equal(t, defaultRequeueDuration, result.RequeueAfter)

		simulateApplicationController(t, c, wandb)
		reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.True(t, wandb.Status.SupportBundle.Ready)
	})

	t.Run("reports whether a collection is running and when one last succeeded", func(t *testing.T) {
		// Launched by hand, like WSM does: the CronJob controller never lists it in status.active.
		job := launchSupportBundleJob(t, c, wandb, "sb-run")
		reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.True(t, wandb.Status.SupportBundle.Running)
		require.True(t, wandb.Status.SupportBundle.Ready, "a running collection does not make the template unready")

		failJob(t, c, job)
		app := &apiv2.Application{}
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKey{Name: supportbundle.Name(wandb), Namespace: wandb.Namespace}, app))
		lastSuccess := metav1.NewTime(time.Now().Truncate(time.Second))
		app.Status.CronJobStatuses = map[string]batchv1.CronJobStatus{supportbundle.CronJobName(wandb): {LastSuccessfulTime: &lastSuccess}}
		require.NoError(t, c.Status().Update(ctx, app))

		reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.False(t, wandb.Status.SupportBundle.Running)
		require.True(t, wandb.Status.SupportBundle.LastSuccessfulTime.Equal(&lastSuccess))
	})

	t.Run("removes the telemetry namespace Role when telemetry is turned off", func(t *testing.T) {
		reconcileSupportBundle(ctx, c, wandb, telemetry.DefaultTelemetryRuntimeConfig())
		err := c.Get(ctx, ctrlClient.ObjectKeyFromObject(objects.telemetryRole), &rbacv1.Role{})
		require.True(t, apierrors.IsNotFound(err))
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(objects.configMap), objects.configMap))
		require.NotContains(t, objects.configMap.Data["full.yaml"], "victoriaStack")
		reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		requireAllPresent(t, c, objects)
	})

	decoy := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: supportbundle.ClusterScopedName(wandb), Namespace: "sb-elsewhere"}}
	require.NoError(t, c.Create(ctx, decoy))

	var job *batchv1.Job
	t.Run("disable keeps everything while a launched collection runs", func(t *testing.T) {
		job = launchSupportBundleJob(t, c, wandb, "collect-1")
		wandb.Spec.SupportBundle.Enabled = ptr.To(false)
		result := reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.Equal(t, time.Minute, result.RequeueAfter)
		requireAllPresent(t, c, objects)
		require.NotNil(t, wandb.Status.SupportBundle)
	})

	t.Run("disable cleans up once the collection finishes", func(t *testing.T) {
		failJob(t, c, job)
		result := reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		require.Zero(t, result.RequeueAfter)
		requireAllAbsent(t, c, objects)
		require.Nil(t, wandb.Status.SupportBundle)
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(decoy), decoy), "unlabeled same-named Role must survive")
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(job), job), "finished Jobs are left to their TTL")
	})

	t.Run("re-enabling recreates everything", func(t *testing.T) {
		wandb.Spec.SupportBundle.Enabled = ptr.To(true)
		reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
		requireAllPresent(t, c, supportBundleObjectKeys(wandb))
	})

	t.Run("install deletion stops running collections and removes everything", func(t *testing.T) {
		running := launchSupportBundleJob(t, c, wandb, "collect-2")
		_, err := deleteSupportBundle(ctx, c, wandb, false)
		require.NoError(t, err)
		requireAllAbsent(t, c, supportBundleObjectKeys(wandb))
		err = c.Get(ctx, ctrlClient.ObjectKeyFromObject(running), &batchv1.Job{})
		require.True(t, apierrors.IsNotFound(err) || runningJobDeleting(t, c, running))
	})
}

// simulateApplicationController stands in for the Application controller, which these tests do not run.
func simulateApplicationController(t *testing.T, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) {
	t.Helper()
	ctx := context.Background()
	app := &apiv2.Application{}
	require.NoError(t, c.Get(ctx, ctrlClient.ObjectKey{Name: supportbundle.Name(wandb), Namespace: wandb.Namespace}, app))
	desired := app.Spec.CronJobs[0].DeepCopy()
	desired.Namespace = app.Namespace
	require.NoError(t, controllerutil.SetControllerReference(app, desired, c.Scheme()))

	current := &batchv1.CronJob{}
	if err := c.Get(ctx, ctrlClient.ObjectKeyFromObject(desired), current); apierrors.IsNotFound(err) {
		require.NoError(t, c.Create(ctx, desired))
	} else {
		require.NoError(t, err)
		current.Spec = desired.Spec
		require.NoError(t, c.Update(ctx, current))
	}
	app.Status.Ready = true
	require.NoError(t, c.Status().Update(ctx, app))
}

// envtest has no garbage collector, so a Job deleted with background propagation may linger with a deletion timestamp.
func runningJobDeleting(t *testing.T, c ctrlClient.Client, job *batchv1.Job) bool {
	t.Helper()
	current := &batchv1.Job{}
	require.NoError(t, c.Get(context.Background(), ctrlClient.ObjectKeyFromObject(job), current))
	return !current.DeletionTimestamp.IsZero()
}

func TestSupportBundleRefusesUnownedObjects(t *testing.T) {
	ctx := context.Background()
	c := supportBundleEnvtest(t)

	t.Run("namespaced ConfigMap", func(t *testing.T) {
		wandb := createSupportBundleWandb(t, c, "collide")
		foreign := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: supportbundle.Name(wandb), Namespace: wandb.Namespace},
			Data:       map[string]string{"owner": "someone-else"},
		}
		require.NoError(t, c.Create(ctx, foreign))

		result := reconcileSupportBundle(ctx, c, wandb, telemetry.DefaultTelemetryRuntimeConfig())
		require.Equal(t, defaultRequeueDuration, result.RequeueAfter)
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(foreign), foreign))
		require.Equal(t, map[string]string{"owner": "someone-else"}, foreign.Data)
		require.Empty(t, foreign.OwnerReferences)
		err := c.Get(ctx, ctrlClient.ObjectKey{Name: supportbundle.Name(wandb), Namespace: wandb.Namespace}, &apiv2.Application{})
		require.True(t, apierrors.IsNotFound(err))

		_, err = deleteSupportBundle(ctx, c, wandb, false)
		require.NoError(t, err)
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(foreign), foreign), "deletion must not remove an unowned ConfigMap")
	})

	t.Run("cluster-scoped ClusterRole", func(t *testing.T) {
		wandb := createSupportBundleWandb(t, c, "collide-cluster")
		foreign := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: supportbundle.ClusterScopedName(wandb)},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get"}}},
		}
		require.NoError(t, c.Create(ctx, foreign))

		result := reconcileSupportBundle(ctx, c, wandb, telemetry.DefaultTelemetryRuntimeConfig())
		require.Equal(t, defaultRequeueDuration, result.RequeueAfter)
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(foreign), foreign))
		require.Equal(t, []string{"configmaps"}, foreign.Rules[0].Resources)

		_, err := deleteSupportBundle(ctx, c, wandb, false)
		require.NoError(t, err)
		require.NoError(t, c.Get(ctx, ctrlClient.ObjectKeyFromObject(foreign), foreign), "deletion must not remove an unlabeled ClusterRole")
	})

	t.Run("waits for the object store", func(t *testing.T) {
		wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "no-bucket", Namespace: supportBundleTestNamespace}}
		wandb.Spec.Wandb.Hostname = "https://wandb.example.com"
		wandb.Spec.Wandb.Features = map[string]bool{}
		wandb.Spec.SupportBundle.Enabled = ptr.To(true)
		require.NoError(t, c.Create(ctx, wandb))

		result := reconcileSupportBundle(ctx, c, wandb, telemetry.DefaultTelemetryRuntimeConfig())
		require.Equal(t, defaultRequeueDuration, result.RequeueAfter)
		require.False(t, wandb.Status.SupportBundle.Ready)
		err := c.Get(ctx, ctrlClient.ObjectKey{Name: supportbundle.Name(wandb), Namespace: wandb.Namespace}, &apiv2.Application{})
		require.True(t, apierrors.IsNotFound(err))
	})
}

func TestSupportBundleDoesNotAffectInstallReadiness(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "ns"}}
	idle := supportbundle.Application(wandb, supportbundle.BucketConnection{URL: &corev1.SecretKeySelector{Key: "url"}})
	api := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "ns", Generation: 1},
		Status:     appsv1.DeploymentStatus{ObservedGeneration: 1, Replicas: 1, ReadyReplicas: 1},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(idle, api).WithStatusSubresource(idle, api).Build()

	healthy, blocked := deploymentsHealthy(context.Background(), c, "ns", map[string]bool{"api": true})
	require.True(t, healthy, "an idle support-bundle Application must not block install readiness")
	require.Empty(t, blocked)
	require.Contains(t, idle.Labels, common.WandbComponentLabel, "manifest pruning skips Applications with a component label")
}

func TestSupportBundleDisableCleansUpWithoutStatus(t *testing.T) {
	ctx := context.Background()
	c := supportBundleEnvtest(t)
	wandb := createSupportBundleWandb(t, c, "partial")
	objects := supportBundleObjectKeys(wandb)

	reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
	requireAllPresent(t, c, objects)

	// A failed status write leaves the resources installed with no summary recorded.
	wandb.Status.SupportBundle = nil
	wandb.Spec.SupportBundle.Enabled = ptr.To(false)
	result := reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)
	require.Zero(t, result.RequeueAfter)
	requireAllAbsent(t, c, objects)
}

func TestSupportBundlePodAdmissionPolicy(t *testing.T) {
	ctx := context.Background()
	c, cfg := supportBundleEnvtestWithConfig(t)
	wandb := createSupportBundleWandb(t, c, "policy")
	reconcileSupportBundle(ctx, c, wandb, supportBundleTelemetry)

	for _, name := range []string{"default", "privileged"} {
		require.NoError(t, c.Create(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}))
	}
	collectorCfg := rest.CopyConfig(cfg)
	collectorCfg.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:%s", wandb.Namespace, supportbundle.Name(wandb)),
	}
	collector, err := ctrlClient.New(collectorCfg, ctrlClient.Options{Scheme: c.Scheme()})
	require.NoError(t, err)

	helperPod := func(name, serviceAccount string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: corev1.PodSpec{
				ServiceAccountName:           serviceAccount,
				AutomountServiceAccountToken: ptr.To(false),
				Containers:                   []corev1.Container{{Name: "export", Image: "curlimages/curl:8.21.0"}},
			},
		}
	}

	// The policy takes a moment to become active after it is created.
	require.Eventually(t, func() bool {
		err := collector.Create(ctx, helperPod("as-privileged", "privileged"))
		return err != nil && strings.Contains(err.Error(), "must run as the default ServiceAccount")
	}, 30*time.Second, 500*time.Millisecond)
	require.NoError(t, collector.Create(ctx, helperPod("as-default", "default")))

	hostPath := helperPod("host-path", "default")
	hostPath.Spec.Volumes = []corev1.Volume{{Name: "root", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/"}}}}
	err = collector.Create(ctx, hostPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not mount hostPath volumes")

	// Other identities are not affected by the policy.
	require.NoError(t, c.Create(ctx, helperPod("admin-as-privileged", "privileged")))
}

func TestSupportBundleSkipsClustersWithoutAdmissionPolicies(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: "ns"}}
	wandb.Spec.SupportBundle.Enabled = ptr.To(true)
	// A pre-1.30 API server has no ValidatingAdmissionPolicy type.
	noPolicyAPI := func(obj ctrlClient.Object) error {
		if _, ok := obj.(*admissionregistrationv1.ValidatingAdmissionPolicy); ok {
			return &meta.NoKindMatchError{GroupKind: admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy").GroupKind()}
		}
		return nil
	}
	c := interceptor.NewClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb).WithStatusSubresource(wandb).Build(), interceptor.Funcs{
		Get: func(ctx context.Context, cl ctrlClient.WithWatch, key ctrlClient.ObjectKey, obj ctrlClient.Object, opts ...ctrlClient.GetOption) error {
			if err := noPolicyAPI(obj); err != nil {
				return err
			}
			return cl.Get(ctx, key, obj, opts...)
		},
		Create: func(ctx context.Context, cl ctrlClient.WithWatch, obj ctrlClient.Object, opts ...ctrlClient.CreateOption) error {
			if err := noPolicyAPI(obj); err != nil {
				return err
			}
			return cl.Create(ctx, obj, opts...)
		},
	})

	result := reconcileSupportBundle(context.Background(), c, wandb, telemetry.DefaultTelemetryRuntimeConfig())
	require.Equal(t, defaultRequeueDuration, result.RequeueAfter)
	require.False(t, wandb.Status.SupportBundle.Ready)
	err := c.Get(context.Background(), ctrlClient.ObjectKey{Name: supportbundle.Name(wandb), Namespace: "ns"}, &corev1.ConfigMap{})
	require.True(t, apierrors.IsNotFound(err), "nothing is installed when the policy cannot be")
}

func TestSupportBundleCleanupRunsWhenReconcileFailsEarly(t *testing.T) {
	ctx := context.Background()
	c := supportBundleEnvtest(t)
	wandb := createSupportBundleWandb(t, c, "broken")
	wandb.Spec.SupportBundle.Enabled = ptr.To(false)
	wandb.Spec.Wandb.ManifestRepository = "file:///does-not-exist"
	require.NoError(t, c.Update(ctx, wandb))
	leftover := supportbundle.ClusterRole(wandb)
	require.NoError(t, c.Create(ctx, leftover))

	_, err := Reconcile(ctx, c, record.NewFakeRecorder(10), wandb, telemetry.DefaultTelemetryRuntimeConfig())
	require.Error(t, err, "the manifest cannot be loaded")
	err = c.Get(ctx, ctrlClient.ObjectKeyFromObject(leftover), &rbacv1.ClusterRole{})
	require.True(t, apierrors.IsNotFound(err), "disabled cleanup must not wait for the manifest")
}
