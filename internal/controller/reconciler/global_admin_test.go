package reconciler

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func adminFixture(t *testing.T) (*apiv2.WeightsAndBiases, serverManifest.Manifest, ctrlclient.Client) {
	t.Helper()
	w := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "default", UID: "instance", Generation: 1},
		Spec:       apiv2.WeightsAndBiasesSpec{Wandb: apiv2.WandbAppSpec{Version: "test"}},
	}
	m := serverManifest.Manifest{
		FeatureBindings:  map[string]serverManifest.FeatureBinding{"admin": {Field: "spec.wandb.enableGlobalAdminAPIKey"}},
		GeneratedSecrets: []serverManifest.GeneratedSecret{{Name: "global-admin-api-key", Length: 40, CharacterType: "hex", Features: []string{"admin"}}},
		CommonEnvvars:    map[string][]serverManifest.EnvVar{"admin": {{Name: "GLOBAL_ADMIN_API_KEY", Features: []string{"admin"}, Sources: []serverManifest.EnvSource{{Name: "global-admin-api-key", Type: "generatedSecret"}}}}},
		Migrations: map[string]serverManifest.MigrationJob{
			"gorilla": {Image: serverManifest.ImageRef{Repository: "test", Tag: "v1"}, CommonEnvs: []string{"admin"}},
			"other":   {Image: serverManifest.ImageRef{Repository: "test", Tag: "v1"}},
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(w, &batchv1.Job{}).WithObjects(w).Build()
	return w, m, c
}

func TestGeneratedAdminSecretLifecycle(t *testing.T) {
	w, source, c := adminFixture(t)
	ctx := context.Background()
	reconcile := func(enabled bool) {
		w.Spec.Wandb.EnableGlobalAdminAPIKey = enabled
		require.NoError(t, c.Update(ctx, w))
		m, err := resolveManifestFeatures(w, source)
		require.NoError(t, err)
		_, err = generateSecrets(ctx, c, w, m)
		require.NoError(t, err)
	}
	reconcile(false)
	var secrets corev1.SecretList
	require.NoError(t, c.List(ctx, &secrets))
	require.Empty(t, secrets.Items)
	reconcile(true)
	var secret corev1.Secret
	key := ctrlclient.ObjectKey{Namespace: w.Namespace, Name: "wandb-global-admin-api-key"}
	require.NoError(t, c.Get(ctx, key, &secret))
	value := string(secret.Data["key"])
	require.Regexp(t, "^[0-9a-f]{40}$", value)
	require.True(t, isOwnedBy(&secret, w))
	reconcile(false)
	require.Empty(t, w.Status.GeneratedSecrets)
	reconcile(true)
	require.NoError(t, c.Get(ctx, key, &secret))
	require.Equal(t, value, string(secret.Data["key"]))
	secret.Data["key"] = []byte(strings.Repeat("z", 40))
	require.NoError(t, c.Update(ctx, &secret))
	m, err := resolveManifestFeatures(w, source)
	require.NoError(t, err)
	_, err = generateSecrets(ctx, c, w, m)
	require.ErrorContains(t, err, "refusing to rotate")
	secret.Data["key"] = []byte(value)
	secret.OwnerReferences = nil
	require.NoError(t, c.Update(ctx, &secret))
	_, err = generateSecrets(ctx, c, w, m)
	require.ErrorContains(t, err, "not owned")
	for _, length := range []int{-1, 0, 39} {
		_, err := generateSecretValue(serverManifest.GeneratedSecret{CharacterType: "hex", Length: length})
		require.Error(t, err)
	}
}

func finishMigration(t *testing.T, c ctrlclient.Client, name string) {
	t.Helper()
	var job batchv1.Job
	require.NoError(t, c.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "default", Name: "wandb-" + name}, &job))
	job.Status.Succeeded = 1
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	require.NoError(t, c.Status().Update(context.Background(), &job))
}

func TestAdminMigrationTransitionsAndRotation(t *testing.T) {
	w, source, c := adminFixture(t)
	ctx := context.Background()
	reconcile := func(enabled bool) {
		w.Spec.Wandb.EnableGlobalAdminAPIKey = enabled
		require.NoError(t, c.Update(ctx, w))
		m, err := resolveManifestFeatures(w, source)
		require.NoError(t, err)
		_, err = generateSecrets(ctx, c, w, m)
		require.NoError(t, err)
		_, err = runMigrations(ctx, c, w, m)
		require.NoError(t, err)
	}
	reconcile(false)
	finishMigration(t, c, "gorilla")
	finishMigration(t, c, "other")
	reconcile(false)
	require.True(t, w.Status.Wandb.Migration.Ready)
	reconcile(false) // completed Jobs are removed; status retains their input hashes
	otherHash := w.Status.Wandb.Migration.Jobs["other"].InputHash
	reconcile(true)
	require.False(t, w.Status.Wandb.Migration.Ready)
	var job batchv1.Job
	key := ctrlclient.ObjectKey{Namespace: w.Namespace, Name: "wandb-gorilla"}
	require.NoError(t, c.Get(ctx, key, &job))
	require.True(t, containsAdminEnv(job.Spec.Template.Spec.Containers[0].Env))
	enabledHash := job.Annotations[workloadInputsAnnotation]
	reconcile(false) // do not kill the in-flight job, even with newer desired inputs
	require.NoError(t, c.Get(ctx, key, &job))
	require.Equal(t, enabledHash, job.Annotations[workloadInputsAnnotation])
	require.Equal(t, "WaitingForPreviousJob", w.Status.Wandb.Migration.Jobs["gorilla"].Reason)
	finishMigration(t, c, "gorilla")
	reconcile(false) // delete the old completed job
	reconcile(false) // create the revocation migration
	require.NoError(t, c.Get(ctx, key, &job))
	require.False(t, containsAdminEnv(job.Spec.Template.Spec.Containers[0].Env))
	finishMigration(t, c, "gorilla")
	reconcile(false)
	require.True(t, w.Status.Wandb.Migration.Ready)
	reconcile(false)
	reconcile(true)
	finishMigration(t, c, "gorilla")
	reconcile(true)
	reconcile(true)
	var secret corev1.Secret
	require.NoError(t, c.Get(ctx, ctrlclient.ObjectKey{Namespace: w.Namespace, Name: "wandb-global-admin-api-key"}, &secret))
	secret.Data["key"] = []byte(strings.Repeat("b", 40))
	require.NoError(t, c.Update(ctx, &secret))
	reconcile(true)
	require.False(t, w.Status.Wandb.Migration.Ready)
	require.NoError(t, c.Get(ctx, key, &job))
	require.NotEqual(t, enabledHash, job.Annotations[workloadInputsAnnotation])
	require.Equal(t, otherHash, w.Status.Wandb.Migration.Jobs["other"].InputHash)
	var jobs batchv1.JobList
	require.NoError(t, c.List(ctx, &jobs))
	require.Len(t, jobs.Items, 1, "unrelated migration must stay cached")
}

func TestAdminMigrationSecurityProfileChangesWaitForNextRun(t *testing.T) {
	w, source, c := adminFixture(t)
	ctx := context.Background()
	delete(source.Migrations, "other")
	reconcile := func(enabled bool, profile serverManifest.WorkloadSecurityProfile) {
		w.Spec.Wandb.EnableGlobalAdminAPIKey = enabled
		require.NoError(t, c.Update(ctx, w))
		task := source.Migrations["gorilla"]
		task.SecurityProfile = profile
		source.Migrations["gorilla"] = task
		m, err := resolveManifestFeatures(w, source)
		require.NoError(t, err)
		_, err = generateSecrets(ctx, c, w, m)
		require.NoError(t, err)
		_, err = runMigrations(ctx, c, w, m)
		require.NoError(t, err)
	}
	profile := serverManifest.WorkloadSecurityProfile{RunAsNonRoot: ptr.To(true), ReadOnlyRootFilesystem: ptr.To(true)}
	reconcile(true, profile)
	var job batchv1.Job
	key := ctrlclient.ObjectKey{Namespace: w.Namespace, Name: "wandb-gorilla"}
	require.NoError(t, c.Get(ctx, key, &job))
	require.Equal(t, profile.RunAsNonRoot, job.Spec.Template.Spec.SecurityContext.RunAsNonRoot)
	require.Equal(t, profile.ReadOnlyRootFilesystem, job.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
	initialHash := job.Annotations[workloadInputsAnnotation]
	finishMigration(t, c, "gorilla")
	reconcile(true, profile)
	require.True(t, w.Status.Wandb.Migration.Ready)
	reconcile(true, profile)
	for _, next := range []serverManifest.WorkloadSecurityProfile{
		{RunAsNonRoot: ptr.To(false), ReadOnlyRootFilesystem: ptr.To(false)}, {},
	} {
		reconcile(true, next)
		require.True(t, w.Status.Wandb.Migration.Ready)
		require.Equal(t, initialHash, w.Status.Wandb.Migration.Jobs["gorilla"].InputHash)
		var jobs batchv1.JobList
		require.NoError(t, c.List(ctx, &jobs))
		require.Empty(t, jobs.Items, "profile changes must not rerun completed migrations")
	}
	profile = serverManifest.WorkloadSecurityProfile{RunAsNonRoot: ptr.To(false), ReadOnlyRootFilesystem: ptr.To(false)}
	reconcile(false, profile)
	require.False(t, w.Status.Wandb.Migration.Ready)
	require.NoError(t, c.Get(ctx, key, &job))
	require.NotEqual(t, initialHash, job.Annotations[workloadInputsAnnotation])
	require.False(t, containsAdminEnv(job.Spec.Template.Spec.Containers[0].Env))
	require.Equal(t, profile.RunAsNonRoot, job.Spec.Template.Spec.SecurityContext.RunAsNonRoot)
	require.Equal(t, profile.ReadOnlyRootFilesystem, job.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem)
}

func TestReadinessWaitsForDesiredApplicationInputs(t *testing.T) {
	w, _, c := adminFixture(t)
	ctx := context.Background()
	app := &apiv2.Application{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: w.Namespace}}
	app.Spec.PodTemplate.Annotations = map[string]string{workloadInputsAnnotation: "new"}
	require.NoError(t, c.Create(ctx, app))
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: w.Namespace}}
	dep.Spec.Replicas = ptr.To(int32(1))
	dep.Spec.Template.Annotations = map[string]string{workloadInputsAnnotation: "old"}
	dep.Status = appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1}
	require.NoError(t, c.Create(ctx, dep))
	healthy, _ := deploymentsHealthy(ctx, c, w.Namespace, map[string]bool{"api": true})
	require.False(t, healthy)
	dep.Spec.Template.Annotations[workloadInputsAnnotation] = "new"
	require.NoError(t, c.Update(ctx, dep))
	healthy, _ = deploymentsHealthy(ctx, c, w.Namespace, map[string]bool{"api": true})
	require.True(t, healthy)
}

func TestAdminApplicationInputsAreStableAndFollowKeyChanges(t *testing.T) {
	w, source, c := adminFixture(t)
	ctx := context.Background()
	w.Spec.Tolerations = &[]corev1.Toleration{}
	w.Spec.Wandb.EnableGlobalAdminAPIKey = true
	w.Spec.Wandb.ServiceAccount.ServiceAccountName = "wandb"
	w.Status.Wandb.Applications = map[string]apiv2.ApplicationStatus{}
	require.NoError(t, c.Update(ctx, w))
	w.Status.Wandb.Applications = map[string]apiv2.ApplicationStatus{}
	require.NoError(t, c.Status().Update(ctx, w))
	source.Applications = map[string]serverManifest.Application{
		"api": {Name: "api", Image: serverManifest.ImageRef{Repository: "test", Tag: "v1"}, CommonEnvs: []string{"admin"}},
	}
	m, err := resolveManifestFeatures(w, source)
	require.NoError(t, err)
	_, err = generateSecrets(ctx, c, w, m)
	require.NoError(t, err)
	reconcile := func() string {
		if w.Status.Wandb.Applications == nil {
			w.Status.Wandb.Applications = map[string]apiv2.ApplicationStatus{}
		}
		_, err := reconcileApplications(ctx, c, w, m, telemetry.DefaultTelemetryRuntimeConfig())
		require.NoError(t, err)
		var app apiv2.Application
		require.NoError(t, c.Get(ctx, ctrlclient.ObjectKey{Namespace: w.Namespace, Name: "api"}, &app))
		// The fake client does not populate the API server's creation timestamp.
		if app.CreationTimestamp.IsZero() {
			app.CreationTimestamp = metav1.Now()
			require.NoError(t, c.Update(ctx, &app))
		}
		require.True(t, containsAdminEnv(app.Spec.PodTemplate.Spec.Containers[0].Env))
		return app.Spec.PodTemplate.Annotations[workloadInputsAnnotation]
	}
	first := reconcile()
	require.NotEmpty(t, first)
	require.Equal(t, first, reconcile(), "an unchanged reconcile must not trigger another rollout")
	var secret corev1.Secret
	require.NoError(t, c.Get(ctx, ctrlclient.ObjectKey{Namespace: w.Namespace, Name: "wandb-global-admin-api-key"}, &secret))
	secret.Data["key"] = []byte(strings.Repeat("c", 40))
	require.NoError(t, c.Update(ctx, &secret))
	require.NotEqual(t, first, reconcile(), "changed credentials must roll the API even with the same Secret reference")
}
