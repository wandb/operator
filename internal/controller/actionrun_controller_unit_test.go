package controller

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	wandbv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func TestActionRunCreatesBoundedJobFromApplication(t *testing.T) {
	t.Parallel()

	testScheme := newActionTestScheme(t)
	run := testActionRun()
	application := testActionApplication()
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&wandbv2.ActionRun{}, &batchv1.Job{}).
		WithObjects(run, application).
		Build()
	reconciler := &ActionRunReconciler{
		Client:  fakeClient,
		Scheme:  testScheme,
		PodLogs: &staticActionLogReader{},
	}

	if _, err := reconciler.Reconcile(context.Background(), requestFor(run)); err != nil {
		t.Fatalf("reconcile ActionRun: %v", err)
	}

	var job batchv1.Job
	if err := fakeClient.Get(context.Background(), types.NamespacedName{
		Namespace: run.Namespace,
		Name:      "weave-check-action",
	}, &job); err != nil {
		t.Fatalf("get action Job: %v", err)
	}
	if job.Spec.Template.Spec.ServiceAccountName != application.Spec.PodTemplate.Spec.ServiceAccountName {
		t.Fatalf("service account = %q, want parent application SA %q",
			job.Spec.Template.Spec.ServiceAccountName,
			application.Spec.PodTemplate.Spec.ServiceAccountName)
	}
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Fatalf("backoffLimit = %v, want 0", job.Spec.BackoffLimit)
	}
	if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Fatalf("restartPolicy = %q, want Never", job.Spec.Template.Spec.RestartPolicy)
	}
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("containers = %d, want exactly one", len(job.Spec.Template.Spec.Containers))
	}
	container := job.Spec.Template.Spec.Containers[0]
	if container.Name != actionContainerName || container.Image != "weave:sha256-test" {
		t.Fatalf("container = %#v, want action container with inherited image", container)
	}
	if got := container.Resources.Requests.Cpu().String(); got != "100m" {
		t.Fatalf("CPU request = %q, want small default 100m", got)
	}
	if got := container.Resources.Requests.Memory().String(); got != "128Mi" {
		t.Fatalf("memory request = %q, want small default 128Mi", got)
	}
	if container.Resources.Requests.Memory().Cmp(
		*application.Spec.PodTemplate.Spec.Containers[0].Resources.Requests.Memory(),
	) >= 0 {
		t.Fatal("action memory request must be smaller than the parent application request")
	}
	if got, want := container.Args, []string{"python", "-m", "weave_triage"}; !slices.Equal(got, want) {
		t.Fatalf("args = %#v, want default runner args %#v", got, want)
	}
	if got := envValue(container.Env, "PYTHONPATH"); got != "/weave/src" {
		t.Fatalf("PYTHONPATH = %q, want action override", got)
	}
	if got := envValue(container.Env, "DATABASE_URL"); got != "mysql://wandb" {
		t.Fatalf("DATABASE_URL = %q, want inherited env", got)
	}
	if got := envValue(container.Env, actionTypeEnv); got != string(wandbv2.ActionTypeTriage) {
		t.Fatalf("%s = %q, want %q", actionTypeEnv, got, wandbv2.ActionTypeTriage)
	}
	if got := envValue(container.Env, actionNameEnv); got != "default" {
		t.Fatalf("%s = %q, want default", actionNameEnv, got)
	}
	if len(container.Ports) != 0 || container.ReadinessProbe != nil || container.LivenessProbe != nil {
		t.Fatal("action container must not inherit serving ports or probes")
	}
	if len(container.VolumeMounts) != 1 || len(job.Spec.Template.Spec.Volumes) != 1 {
		t.Fatal("action Job must inherit the selected container's mounts and parent pod volumes")
	}
	if !metav1.IsControlledBy(&job, run) {
		t.Fatal("action Job is not controlled by the ActionRun")
	}

	var updatedRun wandbv2.ActionRun
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get updated ActionRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.ActionRunPhaseRunning {
		t.Fatalf("phase = %q, want Running", updatedRun.Status.Phase)
	}
	if updatedRun.Status.JobRef == nil || updatedRun.Status.JobRef.Name != job.Name {
		t.Fatalf("jobRef = %#v, want %q", updatedRun.Status.JobRef, job.Name)
	}
	if updatedRun.Status.ResolvedExecution == nil ||
		updatedRun.Status.ResolvedExecution.ContainerName != "weave-trace" {
		t.Fatalf("resolved execution = %#v, want weave-trace container", updatedRun.Status.ResolvedExecution)
	}
}

func TestActionRunSelectsOneNamedAction(t *testing.T) {
	t.Parallel()

	testScheme := newActionTestScheme(t)
	run := testActionRun()
	run.Spec.Action.Name = "deep"
	application := testActionApplication()
	application.Spec.Triage.Actions = append(application.Spec.Triage.Actions,
		wandbv2.ApplicationActionSpec{
			Name:        "deep",
			Description: "Run deeper diagnostics",
			Args:        []string{"--verbose"},
		})
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&wandbv2.ActionRun{}, &batchv1.Job{}).
		WithObjects(run, application).
		Build()
	reconciler := &ActionRunReconciler{Client: fakeClient, Scheme: testScheme}

	if _, err := reconciler.Reconcile(context.Background(), requestFor(run)); err != nil {
		t.Fatalf("reconcile named ActionRun: %v", err)
	}
	var jobs batchv1.JobList
	if err := fakeClient.List(context.Background(), &jobs, client.InNamespace(run.Namespace)); err != nil {
		t.Fatalf("list Jobs: %v", err)
	}
	if len(jobs.Items) != 1 {
		t.Fatalf("Jobs = %d, want exactly one", len(jobs.Items))
	}
	job := jobs.Items[0]
	if job.Annotations[actionTypeAnnotation] != "triage" ||
		job.Annotations[actionNameAnnotation] != "deep" {
		t.Fatalf("Job annotations = %#v", job.Annotations)
	}
	container := job.Spec.Template.Spec.Containers[0]
	if got, want := container.Args, []string{"python", "-m", "weave_triage", "--verbose"}; !slices.Equal(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	if got := envValue(container.Env, actionNameEnv); got != "deep" {
		t.Fatalf("%s = %q, want deep", actionNameEnv, got)
	}
}

func TestActionRunRejectsUnsupportedType(t *testing.T) {
	t.Parallel()

	testScheme := newActionTestScheme(t)
	run := testActionRun()
	run.Spec.Type = wandbv2.ActionTypeMaintenance
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&wandbv2.ActionRun{}, &batchv1.Job{}).
		WithObjects(run).
		Build()
	reconciler := &ActionRunReconciler{Client: fakeClient, Scheme: testScheme}

	if _, err := reconciler.Reconcile(context.Background(), requestFor(run)); err != nil {
		t.Fatalf("reconcile maintenance ActionRun: %v", err)
	}
	var updatedRun wandbv2.ActionRun
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get updated ActionRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.ActionRunPhaseFailed {
		t.Fatalf("phase = %q, want Failed", updatedRun.Status.Phase)
	}
	if conditionReason(updatedRun.Status.Conditions, actionConditionSucceeded) != "UnsupportedActionType" {
		t.Fatalf("conditions = %#v, want UnsupportedActionType", updatedRun.Status.Conditions)
	}
	var jobs batchv1.JobList
	if err := fakeClient.List(context.Background(), &jobs); err != nil {
		t.Fatalf("list Jobs: %v", err)
	}
	if len(jobs.Items) != 0 {
		t.Fatalf("Jobs = %d, want none", len(jobs.Items))
	}
}

func TestActionRunCollectsFailedCheckAsSuccessfulExecution(t *testing.T) {
	t.Parallel()

	testScheme := newActionTestScheme(t)
	run := testActionRun()
	application := testActionApplication()
	logReader := &staticActionLogReader{}
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&wandbv2.ActionRun{}, &batchv1.Job{}).
		WithObjects(run, application).
		Build()
	reconciler := &ActionRunReconciler{Client: fakeClient, Scheme: testScheme, PodLogs: logReader}

	if _, err := reconciler.Reconcile(context.Background(), requestFor(run)); err != nil {
		t.Fatalf("create action Job: %v", err)
	}
	var job batchv1.Job
	if err := fakeClient.Get(context.Background(), types.NamespacedName{
		Namespace: run.Namespace,
		Name:      "weave-check-action",
	}, &job); err != nil {
		t.Fatalf("get action Job: %v", err)
	}
	job.Status.Conditions = []batchv1.JobCondition{{
		Type: batchv1.JobComplete, Status: corev1.ConditionTrue,
	}}
	if err := fakeClient.Status().Update(context.Background(), &job); err != nil {
		t.Fatalf("mark Job complete: %v", err)
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "weave-check-action-pod",
		Namespace: run.Namespace,
		Labels:    map[string]string{"batch.kubernetes.io/job-name": job.Name},
	}}
	if err := fakeClient.Create(context.Background(), pod); err != nil {
		t.Fatalf("create Job pod: %v", err)
	}
	logReader.output = []byte(
		`{"name":"starter-project","severity":"pass","message":"reachable"}` + "\n" +
			`{"name":"starter-object","severity":"fail","evidence":{"missing":2},"remediation":"create starters"}` + "\n",
	)

	if _, err := reconciler.Reconcile(context.Background(), requestFor(run)); err != nil {
		t.Fatalf("reconcile completed ActionRun: %v", err)
	}
	var updatedRun wandbv2.ActionRun
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get completed ActionRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.ActionRunPhaseSucceeded {
		t.Fatalf("phase = %q, want Succeeded because the command completed", updatedRun.Status.Phase)
	}
	if updatedRun.Status.Summary == nil ||
		updatedRun.Status.Summary.OverallSeverity != wandbv2.ActionSeverityFail ||
		updatedRun.Status.Summary.Fail != 1 {
		t.Fatalf("summary = %#v, want one failed check", updatedRun.Status.Summary)
	}
	if len(updatedRun.Status.Results) != 2 {
		t.Fatalf("results = %#v, want 2", updatedRun.Status.Results)
	}
}

func TestActionRunWaitsBrieflyForCompletedJobPod(t *testing.T) {
	t.Parallel()

	reconciler, fakeClient, run, job := completedActionRunWithoutPod(t,
		time.Now().Add(-actionResultsGracePeriod/2))

	result, err := reconciler.Reconcile(context.Background(), requestFor(run))
	if err != nil {
		t.Fatalf("reconcile completed ActionRun: %v", err)
	}
	if result.RequeueAfter != actionResultsRetryInterval {
		t.Fatalf("requeueAfter = %s, want %s", result.RequeueAfter, actionResultsRetryInterval)
	}

	var updatedRun wandbv2.ActionRun
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get updated ActionRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.ActionRunPhaseRunning {
		t.Fatalf("phase = %q, want Running", updatedRun.Status.Phase)
	}
	if got := conditionReason(updatedRun.Status.Conditions, actionConditionSucceeded); got != "ResultsPending" {
		t.Fatalf("condition reason = %q, want ResultsPending for Job %q", got, job.Name)
	}
}

func TestActionRunFailsWhenCompletedJobPodRemainsUnavailable(t *testing.T) {
	t.Parallel()

	reconciler, fakeClient, run, _ := completedActionRunWithoutPod(t,
		time.Now().Add(-actionResultsGracePeriod-time.Minute))

	result, err := reconciler.Reconcile(context.Background(), requestFor(run))
	if err != nil {
		t.Fatalf("reconcile completed ActionRun: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("requeueAfter = %s, want no requeue", result.RequeueAfter)
	}

	var updatedRun wandbv2.ActionRun
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get updated ActionRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.ActionRunPhaseFailed {
		t.Fatalf("phase = %q, want Failed", updatedRun.Status.Phase)
	}
	if got := conditionReason(updatedRun.Status.Conditions, actionConditionSucceeded); got != "ResultsUnavailable" {
		t.Fatalf("condition reason = %q, want ResultsUnavailable", got)
	}
	if updatedRun.Status.CompletedAt == nil {
		t.Fatal("completedAt is nil, want the Job completion time")
	}
}

func TestParseActionJSONLRejectsNoisyOutput(t *testing.T) {
	t.Parallel()

	_, err := parseActionJSONL([]byte(
		`{"name":"starter-project","severity":"pass"}` + "\n" +
			"debug: checking object\n",
	))
	if err == nil {
		t.Fatal("expected non-JSON output to be rejected")
	}
}

func TestManifestTriageActionDecodes(t *testing.T) {
	t.Parallel()

	var decoded serverManifest.Manifest
	input := []byte(`
applications:
  weave-trace:
    triage:
      containerName: weave-trace
      args: [python, -m, weave_triage]
      timeoutSeconds: 600
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
      actions:
        - name: default
          description: Run all diagnostics
`)
	if err := yaml.Unmarshal(input, &decoded); err != nil {
		t.Fatalf("decode manifest triage action: %v", err)
	}
	triage := decoded.Applications["weave-trace"].Triage
	if triage.ContainerName != "weave-trace" || triage.TimeoutSeconds != 600 ||
		len(triage.Actions) != 1 || triage.Actions[0].Name != "default" {
		t.Fatalf("decoded triage = %#v", triage)
	}
}

type staticActionLogReader struct {
	output []byte
	err    error
}

func completedActionRunWithoutPod(
	t *testing.T,
	completedAt time.Time,
) (*ActionRunReconciler, client.Client, *wandbv2.ActionRun, *batchv1.Job) {
	t.Helper()

	testScheme := newActionTestScheme(t)
	run := testActionRun()
	application := testActionApplication()
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&wandbv2.ActionRun{}, &batchv1.Job{}).
		WithObjects(run, application).
		Build()
	reconciler := &ActionRunReconciler{
		Client:  fakeClient,
		Scheme:  testScheme,
		PodLogs: &staticActionLogReader{},
	}
	if _, err := reconciler.Reconcile(context.Background(), requestFor(run)); err != nil {
		t.Fatalf("create action Job: %v", err)
	}

	job := &batchv1.Job{}
	if err := fakeClient.Get(context.Background(), types.NamespacedName{
		Namespace: run.Namespace,
		Name:      "weave-check-action",
	}, job); err != nil {
		t.Fatalf("get action Job: %v", err)
	}
	completionTime := metav1.NewTime(completedAt)
	job.Status.CompletionTime = &completionTime
	job.Status.Conditions = []batchv1.JobCondition{{
		Type:               batchv1.JobComplete,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: completionTime,
	}}
	if err := fakeClient.Status().Update(context.Background(), job); err != nil {
		t.Fatalf("mark Job complete: %v", err)
	}
	return reconciler, fakeClient, run, job
}

func (r *staticActionLogReader) ReadPodLogs(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ int64,
) ([]byte, error) {
	return r.output, r.err
}

func newActionTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	testScheme := runtime.NewScheme()
	for name, add := range map[string]func(*runtime.Scheme) error{
		"apps.wandb.com/v2": wandbv2.AddToScheme,
		"batch/v1":          batchv1.AddToScheme,
		"core/v1":           corev1.AddToScheme,
	} {
		if err := add(testScheme); err != nil {
			t.Fatalf("add %s to scheme: %v", name, err)
		}
	}
	return testScheme
}

func testActionRun() *wandbv2.ActionRun {
	return &wandbv2.ActionRun{
		TypeMeta: metav1.TypeMeta{
			APIVersion: wandbv2.GroupVersion.String(),
			Kind:       "ActionRun",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "weave-check",
			Namespace: "wandb",
			UID:       types.UID("action-run-uid"),
		},
		Spec: wandbv2.ActionRunSpec{
			Type:           wandbv2.ActionTypeTriage,
			ApplicationRef: wandbv2.ApplicationReference{Name: "weave-trace"},
			Action:         wandbv2.ActionReference{Name: "default"},
		},
	}
}

func testActionApplication() *wandbv2.Application {
	return &wandbv2.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "weave-trace",
			Namespace:  "wandb",
			Generation: 7,
		},
		Spec: wandbv2.ApplicationSpec{
			PodTemplate: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "wandb-app",
					ImagePullSecrets:   []corev1.LocalObjectReference{{Name: "registry"}},
					Volumes: []corev1.Volume{{
						Name: "ca",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "custom-ca"},
							},
						},
					}},
					Containers: []corev1.Container{{
						Name:  "weave-trace",
						Image: "weave:sha256-test",
						Args:  []string{"uvicorn", "weave.trace_server.app:app"},
						Env: []corev1.EnvVar{
							{Name: "DATABASE_URL", Value: "mysql://wandb"},
							{Name: "PYTHONPATH", Value: "/parent"},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
							},
						},
						Ports:          []corev1.ContainerPort{{ContainerPort: 8080}},
						ReadinessProbe: &corev1.Probe{},
						LivenessProbe:  &corev1.Probe{},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "ca",
							MountPath: "/etc/ssl/custom-ca.pem",
						}},
					}},
				},
			},
			Triage: &wandbv2.ApplicationTriageSpec{
				ContainerName: "weave-trace",
				Args:          []string{"python", "-m", "weave_triage"},
				Env: []corev1.EnvVar{
					{Name: "PYTHONPATH", Value: "/weave/src"},
					{Name: actionTypeEnv, Value: "manifest-value-must-not-win"},
					{Name: actionNameEnv, Value: "manifest-value-must-not-win"},
				},
				Actions: []wandbv2.ApplicationActionSpec{{
					Name:        "default",
					Description: "Run all diagnostics",
				}},
			},
		},
	}
}

func requestFor(run *wandbv2.ActionRun) ctrl.Request {
	return ctrl.Request{NamespacedName: client.ObjectKeyFromObject(run)}
}

func envValue(env []corev1.EnvVar, name string) string {
	for _, variable := range env {
		if variable.Name == name {
			return variable.Value
		}
	}
	return fmt.Sprintf("<%s not found>", name)
}

func conditionReason(conditions []metav1.Condition, conditionType string) string {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Reason
		}
	}
	return ""
}
