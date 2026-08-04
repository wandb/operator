package controller

import (
	"context"
	"fmt"
	"testing"

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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

func TestTriageRunCreatesBoundedJobFromApplication(t *testing.T) {
	t.Parallel()

	testScheme := newTriageTestScheme(t)
	run := testTriageRun()
	application := testTriageApplication()
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&wandbv2.TriageRun{}, &batchv1.Job{}).
		WithObjects(run, application).
		Build()
	logReader := &staticTriageLogReader{}
	reconciler := &TriageRunReconciler{
		Client:  fakeClient,
		Scheme:  testScheme,
		PodLogs: logReader,
	}

	_, err := reconciler.Reconcile(context.Background(), requestFor(run))
	if err != nil {
		t.Fatalf("reconcile TriageRun: %v", err)
	}

	var job batchv1.Job
	if err := fakeClient.Get(context.Background(), types.NamespacedName{
		Namespace: run.Namespace,
		Name:      "weave-check-triage-0",
	}, &job); err != nil {
		t.Fatalf("get triage Job: %v", err)
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
	if container.Image != "weave:sha256-test" {
		t.Fatalf("image = %q, want inherited image", container.Image)
	}
	if got := container.Resources.Requests.Cpu().String(); got != "100m" {
		t.Fatalf("CPU request = %q, want small default 100m", got)
	}
	if got := container.Resources.Requests.Memory().String(); got != "128Mi" {
		t.Fatalf("memory request = %q, want small default 128Mi", got)
	}
	if got := application.Spec.PodTemplate.Spec.Containers[0].Resources.Requests.Memory().String(); got != "8Gi" {
		t.Fatalf("test parent memory request = %q, want 8Gi", got)
	}
	if container.Resources.Requests.Memory().Cmp(
		*application.Spec.PodTemplate.Spec.Containers[0].Resources.Requests.Memory(),
	) >= 0 {
		t.Fatal("triage memory request must be smaller than the parent application request")
	}
	if len(container.Args) != 4 || container.Args[0] != "python" ||
		container.Args[3] != defaultTriageAction {
		t.Fatalf("args = %#v, want triage action args", container.Args)
	}
	if got := envValue(container.Env, "PYTHONPATH"); got != "/weave/src" {
		t.Fatalf("PYTHONPATH = %q, want action override", got)
	}
	if got := envValue(container.Env, "DATABASE_URL"); got != "mysql://wandb" {
		t.Fatalf("DATABASE_URL = %q, want inherited env", got)
	}
	if len(container.Ports) != 0 || container.ReadinessProbe != nil || container.LivenessProbe != nil {
		t.Fatal("triage container must not inherit serving ports or probes")
	}
	if len(container.VolumeMounts) != 1 || len(job.Spec.Template.Spec.Volumes) != 1 {
		t.Fatal("triage Job must inherit the selected container's mounts and parent pod volumes")
	}
	if !metav1.IsControlledBy(&job, run) {
		t.Fatal("triage Job is not controlled by the TriageRun")
	}

	var updatedRun wandbv2.TriageRun
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get updated TriageRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.TriageRunPhaseRunning {
		t.Fatalf("phase = %q, want Running", updatedRun.Status.Phase)
	}
	if len(updatedRun.Status.ActionStatuses) != 1 {
		t.Fatalf("action statuses = %d, want 1", len(updatedRun.Status.ActionStatuses))
	}
	actionStatus := updatedRun.Status.ActionStatuses[0]
	if actionStatus.Action != defaultTriageAction ||
		actionStatus.JobRef == nil || actionStatus.JobRef.Name != job.Name {
		t.Fatalf("action status = %#v, want %q Job %q", actionStatus, defaultTriageAction, job.Name)
	}
	if actionStatus.ResolvedExecution == nil ||
		actionStatus.ResolvedExecution.ContainerName != "weave-trace" {
		t.Fatalf("resolved execution = %#v, want weave-trace container", actionStatus.ResolvedExecution)
	}
}

func TestTriageRunCreatesOneJobPerSelectedAction(t *testing.T) {
	t.Parallel()

	testScheme := newTriageTestScheme(t)
	run := testTriageRun()
	run.Spec.Actions = []wandbv2.TriageActionReference{
		{Name: "default"},
		{Name: "deep"},
	}
	application := testTriageApplication()
	application.Spec.Triage.Actions = append(application.Spec.Triage.Actions,
		wandbv2.TriageActionSpec{
			Name:        "deep",
			Description: "Run deeper diagnostics",
		})
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&wandbv2.TriageRun{}, &batchv1.Job{}).
		WithObjects(run, application).
		Build()
	logReader := &staticTriageLogReader{}
	reconciler := &TriageRunReconciler{
		Client:  fakeClient,
		Scheme:  testScheme,
		PodLogs: logReader,
	}

	if _, err := reconciler.Reconcile(context.Background(), requestFor(run)); err != nil {
		t.Fatalf("reconcile multi-action TriageRun: %v", err)
	}

	for index, action := range []string{"default", "deep"} {
		var job batchv1.Job
		name := fmt.Sprintf("weave-check-triage-%d", index)
		if err := fakeClient.Get(context.Background(), types.NamespacedName{
			Namespace: run.Namespace,
			Name:      name,
		}, &job); err != nil {
			t.Fatalf("get Job for action %q: %v", action, err)
		}
		if job.Annotations[triageActionAnnotation] != action {
			t.Fatalf("Job %q action annotation = %q, want %q",
				job.Name, job.Annotations[triageActionAnnotation], action)
		}
		args := job.Spec.Template.Spec.Containers[0].Args
		if len(args) != 4 || args[3] != action {
			t.Fatalf("Job %q args = %#v, want selected action %q appended", job.Name, args, action)
		}
	}

	var updatedRun wandbv2.TriageRun
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get updated TriageRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.TriageRunPhaseRunning ||
		len(updatedRun.Status.ActionStatuses) != 2 {
		t.Fatalf("status = %#v, want two running actions", updatedRun.Status)
	}
	for i, action := range run.Spec.Actions {
		if updatedRun.Status.ActionStatuses[i].Action != action.Name {
			t.Fatalf("action status %d = %q, want %q",
				i, updatedRun.Status.ActionStatuses[i].Action, action.Name)
		}
	}

	logReader.output = []byte(`{"name":"reachable","severity":"pass"}` + "\n")
	for index := range run.Spec.Actions {
		name := fmt.Sprintf("weave-check-triage-%d", index)
		var job batchv1.Job
		if err := fakeClient.Get(context.Background(), types.NamespacedName{
			Namespace: run.Namespace,
			Name:      name,
		}, &job); err != nil {
			t.Fatalf("get Job %q for completion: %v", name, err)
		}
		job.Status.Conditions = []batchv1.JobCondition{{
			Type: batchv1.JobComplete, Status: corev1.ConditionTrue,
		}}
		if err := fakeClient.Status().Update(context.Background(), &job); err != nil {
			t.Fatalf("mark Job %q complete: %v", name, err)
		}
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-pod",
			Namespace: run.Namespace,
			Labels:    map[string]string{"batch.kubernetes.io/job-name": name},
		}}
		if err := fakeClient.Create(context.Background(), pod); err != nil {
			t.Fatalf("create pod for Job %q: %v", name, err)
		}
	}
	if _, err := reconciler.Reconcile(context.Background(), requestFor(run)); err != nil {
		t.Fatalf("reconcile completed multi-action TriageRun: %v", err)
	}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get completed TriageRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.TriageRunPhaseSucceeded ||
		updatedRun.Status.Summary == nil || updatedRun.Status.Summary.Total != 2 {
		t.Fatalf("completed status = %#v, want two aggregated results", updatedRun.Status)
	}
	for _, status := range updatedRun.Status.ActionStatuses {
		if status.Phase != wandbv2.TriageRunPhaseSucceeded || len(status.Results) != 1 {
			t.Fatalf("action status = %#v, want one successful result", status)
		}
	}
}

func TestTriageRunCollectsFailedCheckAsSuccessfulExecution(t *testing.T) {
	t.Parallel()

	testScheme := newTriageTestScheme(t)
	run := testTriageRun()
	run.Status.Phase = wandbv2.TriageRunPhaseRunning
	run.Status.ActionStatuses = []wandbv2.TriageActionStatus{{
		Action: defaultTriageAction,
		JobRef: &corev1.LocalObjectReference{Name: "weave-check-triage-0"},
	}}
	application := testTriageApplication()
	action := application.Spec.Triage.Actions[0]
	source := &application.Spec.PodTemplate.Spec.Containers[0]
	job := buildTriageJob(
		run, application, source, application.Spec.Triage, action,
		defaultTriageTimeoutSeconds, "weave-check-triage-0")
	if err := controllerutil.SetControllerReference(run, job, testScheme); err != nil {
		t.Fatalf("set Job owner: %v", err)
	}
	job.Status.Conditions = []batchv1.JobCondition{{
		Type:   batchv1.JobComplete,
		Status: corev1.ConditionTrue,
	}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "weave-check-triage-abcde",
			Namespace: run.Namespace,
			Labels:    map[string]string{"batch.kubernetes.io/job-name": job.Name},
		},
	}
	logs := []byte(
		`{"name":"starter-project","severity":"pass","message":"reachable"}` + "\n" +
			`{"name":"starter-object","severity":"fail","evidence":{"missing":2},"remediation":"create starters"}` + "\n",
	)
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&wandbv2.TriageRun{}, &batchv1.Job{}).
		WithObjects(run, application, job, pod).
		Build()
	reconciler := &TriageRunReconciler{
		Client:  fakeClient,
		Scheme:  testScheme,
		PodLogs: &staticTriageLogReader{output: logs},
	}

	_, err := reconciler.Reconcile(context.Background(), requestFor(run))
	if err != nil {
		t.Fatalf("reconcile completed TriageRun: %v", err)
	}

	var updatedRun wandbv2.TriageRun
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(run), &updatedRun); err != nil {
		t.Fatalf("get updated TriageRun: %v", err)
	}
	if updatedRun.Status.Phase != wandbv2.TriageRunPhaseSucceeded {
		t.Fatalf("phase = %q, want Succeeded because the command completed", updatedRun.Status.Phase)
	}
	if updatedRun.Status.Summary == nil ||
		updatedRun.Status.Summary.OverallSeverity != wandbv2.TriageSeverityFail ||
		updatedRun.Status.Summary.Fail != 1 {
		t.Fatalf("summary = %#v, want one failed diagnostic check", updatedRun.Status.Summary)
	}
	if len(updatedRun.Status.ActionStatuses) != 1 ||
		len(updatedRun.Status.ActionStatuses[0].Results) != 2 {
		t.Fatalf("action statuses = %#v, want 2 results", updatedRun.Status.ActionStatuses)
	}
}

func TestParseTriageJSONLRejectsNoisyOutput(t *testing.T) {
	t.Parallel()

	_, err := parseTriageJSONL([]byte(
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
		len(triage.Actions) != 1 || triage.Actions[0].Name != defaultTriageAction {
		t.Fatalf("decoded triage = %#v", triage)
	}
}

type staticTriageLogReader struct {
	output []byte
	err    error
}

func (r *staticTriageLogReader) ReadPodLogs(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ int64,
) ([]byte, error) {
	return r.output, r.err
}

func newTriageTestScheme(t *testing.T) *runtime.Scheme {
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

func testTriageRun() *wandbv2.TriageRun {
	return &wandbv2.TriageRun{
		TypeMeta: metav1.TypeMeta{
			APIVersion: wandbv2.GroupVersion.String(),
			Kind:       "TriageRun",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "weave-check",
			Namespace: "wandb",
			UID:       types.UID("triage-run-uid"),
		},
		Spec: wandbv2.TriageRunSpec{
			ApplicationRef: wandbv2.TriageApplicationReference{Name: "weave-trace"},
			Actions: []wandbv2.TriageActionReference{{
				Name: defaultTriageAction,
			}},
		},
	}
}

func testTriageApplication() *wandbv2.Application {
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
				Env: []corev1.EnvVar{{
					Name:  "PYTHONPATH",
					Value: "/weave/src",
				}},
				Actions: []wandbv2.TriageActionSpec{{
					Name:        defaultTriageAction,
					Description: "Run all diagnostics",
				}},
			},
		},
	}
}

func requestFor(run *wandbv2.TriageRun) ctrl.Request {
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
