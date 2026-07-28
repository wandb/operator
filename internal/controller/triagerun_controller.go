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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	wandbv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultTriageAction         = "default"
	defaultTriageTimeoutSeconds = int64(300)
	maxTriageOutputBytes        = int64(512 * 1024)
	triageContainerName         = "triage"
	triageConditionSucceeded    = "Succeeded"
	triageRunLabel              = "apps.wandb.com/triage-run"
	triageApplicationLabel      = "apps.wandb.com/triage-application"
)

// TriagePodLogReader reads the structured output from a completed triage pod.
// It is an interface so controller behavior can be tested without a live API
// server's pod log subresource.
type TriagePodLogReader interface {
	ReadPodLogs(
		ctx context.Context,
		namespace string,
		podName string,
		containerName string,
		maxBytes int64,
	) ([]byte, error)
}

type KubernetesTriagePodLogReader struct {
	CoreV1 corev1client.CoreV1Interface
}

func (r *KubernetesTriagePodLogReader) ReadPodLogs(
	ctx context.Context,
	namespace string,
	podName string,
	containerName string,
	maxBytes int64,
) ([]byte, error) {
	stream, err := r.CoreV1.Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	}).Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	output, err := io.ReadAll(io.LimitReader(stream, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(output)) > maxBytes {
		return nil, &triageOutputTooLargeError{maxBytes: maxBytes}
	}
	return output, nil
}

// TriageRunReconciler turns each immutable TriageRun into exactly one Job and
// records the Job's structured JSONL output on the run status.
type TriageRunReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	PodLogs TriagePodLogReader
}

// +kubebuilder:rbac:groups=apps.wandb.com,resources=triageruns,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.wandb.com,resources=triageruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.wandb.com,resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get

func (r *TriageRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var run wandbv2.TriageRun
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if isTerminalTriagePhase(run.Status.Phase) {
		return ctrl.Result{}, nil
	}

	var application wandbv2.Application
	applicationKey := types.NamespacedName{
		Namespace: run.Namespace,
		Name:      run.Spec.ApplicationRef.Name,
	}
	if err := r.Get(ctx, applicationKey, &application); err != nil {
		if apierrors.IsNotFound(err) {
			return r.failRun(ctx, &run, "ApplicationNotFound",
				fmt.Sprintf("Application %q does not exist in namespace %q", applicationKey.Name, applicationKey.Namespace))
		}
		return ctrl.Result{}, err
	}

	actionName := run.Spec.Action
	if actionName == "" {
		actionName = defaultTriageAction
	}
	action, err := resolveTriageAction(&application, actionName)
	if err != nil {
		return r.failRun(ctx, &run, "InvalidAction", err.Error())
	}

	sourceContainer, err := selectTriageContainer(&application, action.ContainerName)
	if err != nil {
		return r.failRun(ctx, &run, "InvalidContainer", err.Error())
	}

	timeoutSeconds := action.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultTriageTimeoutSeconds
	}
	resolved := resolvedTriageExecution(&application, sourceContainer, action, timeoutSeconds)
	jobName := common.FitDefaultInfraName(run.Name, "-triage", 63)

	var job batchv1.Job
	err = r.Get(ctx, types.NamespacedName{Namespace: run.Namespace, Name: jobName}, &job)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if apierrors.IsNotFound(err) {
		if run.Status.JobRef != nil {
			return r.failRun(ctx, &run, "JobMissing",
				fmt.Sprintf("Job %q disappeared before the run completed", run.Status.JobRef.Name))
		}

		job = *buildTriageJob(&run, &application, sourceContainer, action, timeoutSeconds, jobName)
		if err := controllerutil.SetControllerReference(&run, &job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, &job); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return ctrl.Result{}, err
			}
			if err := r.Get(ctx, types.NamespacedName{Namespace: run.Namespace, Name: jobName}, &job); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if !metav1.IsControlledBy(&job, &run) {
		return r.failRun(ctx, &run, "JobNameCollision",
			fmt.Sprintf("Job %q already exists and is not owned by this TriageRun", job.Name))
	}

	if jobFailed(&job) {
		message := jobConditionMessage(&job, batchv1.JobFailed)
		if message == "" {
			message = fmt.Sprintf("Job %q failed", job.Name)
		}
		if results, collectErr := r.collectTriageResults(ctx, &job); collectErr == nil {
			run.Status.Results = results
			run.Status.Summary = summarizeTriageResults(results)
		}
		return r.failRunWithJob(ctx, &run, &job, resolved, "JobFailed", message)
	}

	if jobComplete(&job) {
		results, err := r.collectTriageResults(ctx, &job)
		if err != nil {
			var unavailable *triageOutputUnavailableError
			if errors.As(err, &unavailable) {
				return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
			}
			return r.failRunWithJob(ctx, &run, &job, resolved, "InvalidOutput", err.Error())
		}
		return r.completeRun(ctx, &run, &job, resolved, results)
	}

	return r.markRunRunning(ctx, &run, &job, resolved)
}

func resolveTriageAction(application *wandbv2.Application, actionName string) (wandbv2.TriageActionSpec, error) {
	if application.Spec.Triage == nil {
		return wandbv2.TriageActionSpec{}, fmt.Errorf("Application %q does not declare triage actions", application.Name)
	}
	action, ok := application.Spec.Triage.Actions[actionName]
	if !ok {
		return wandbv2.TriageActionSpec{}, fmt.Errorf(
			"Application %q does not declare triage action %q", application.Name, actionName)
	}
	if len(action.Command) == 0 && len(action.Args) == 0 {
		return wandbv2.TriageActionSpec{}, fmt.Errorf(
			"triage action %q must override command or args", actionName)
	}
	return action, nil
}

func selectTriageContainer(application *wandbv2.Application, name string) (*corev1.Container, error) {
	containers := application.Spec.PodTemplate.Spec.Containers
	if name == "" {
		if len(containers) != 1 {
			return nil, fmt.Errorf(
				"triage action must select a container because Application %q has %d containers",
				application.Name, len(containers))
		}
		return &containers[0], nil
	}
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i], nil
		}
	}
	return nil, fmt.Errorf("Application %q has no container named %q", application.Name, name)
}

func buildTriageJob(
	run *wandbv2.TriageRun,
	application *wandbv2.Application,
	source *corev1.Container,
	action wandbv2.TriageActionSpec,
	timeoutSeconds int64,
	jobName string,
) *batchv1.Job {
	backoffLimit := int32(0)
	podSpec := application.Spec.PodTemplate.Spec.DeepCopy()
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.InitContainers = nil
	podSpec.EphemeralContainers = nil
	podSpec.ReadinessGates = nil
	podSpec.Containers = []corev1.Container{buildTriageContainer(source, action)}

	labels := map[string]string{
		triageRunLabel:         common.FitDefaultInfraName(run.Name, "", 63),
		triageApplicationLabel: common.FitDefaultInfraName(application.Name, "", 63),
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: run.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &timeoutSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       *podSpec,
			},
		},
	}
}

func buildTriageContainer(source *corev1.Container, action wandbv2.TriageActionSpec) corev1.Container {
	container := corev1.Container{
		Name:                     triageContainerName,
		Image:                    source.Image,
		ImagePullPolicy:          source.ImagePullPolicy,
		Command:                  append([]string(nil), source.Command...),
		Args:                     append([]string(nil), source.Args...),
		WorkingDir:               source.WorkingDir,
		EnvFrom:                  append([]corev1.EnvFromSource(nil), source.EnvFrom...),
		Env:                      mergeTriageEnv(source.Env, action.Env),
		Resources:                defaultTriageResources(),
		VolumeMounts:             append([]corev1.VolumeMount(nil), source.VolumeMounts...),
		VolumeDevices:            append([]corev1.VolumeDevice(nil), source.VolumeDevices...),
		SecurityContext:          source.SecurityContext.DeepCopy(),
		TerminationMessagePath:   source.TerminationMessagePath,
		TerminationMessagePolicy: source.TerminationMessagePolicy,
	}
	if len(action.Command) > 0 {
		container.Command = append([]string(nil), action.Command...)
	}
	if len(action.Args) > 0 {
		container.Args = append([]string(nil), action.Args...)
	}
	if action.Resources != nil {
		container.Resources = *action.Resources.DeepCopy()
	}
	return container
}

func mergeTriageEnv(inherited, overrides []corev1.EnvVar) []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, len(inherited)+len(overrides))
	overrideNames := make(map[string]struct{}, len(overrides))
	for _, env := range overrides {
		overrideNames[env.Name] = struct{}{}
	}
	for _, env := range inherited {
		if _, overridden := overrideNames[env.Name]; !overridden {
			result = append(result, *env.DeepCopy())
		}
	}
	for _, env := range overrides {
		result = append(result, *env.DeepCopy())
	}
	return result
}

func defaultTriageResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
}

func resolvedTriageExecution(
	application *wandbv2.Application,
	source *corev1.Container,
	action wandbv2.TriageActionSpec,
	timeoutSeconds int64,
) *wandbv2.TriageResolvedExecution {
	container := buildTriageContainer(source, action)
	return &wandbv2.TriageResolvedExecution{
		ApplicationGeneration: application.Generation,
		ContainerName:         source.Name,
		Image:                 container.Image,
		Command:               append([]string(nil), container.Command...),
		Args:                  append([]string(nil), container.Args...),
		TimeoutSeconds:        timeoutSeconds,
	}
}

func (r *TriageRunReconciler) markRunRunning(
	ctx context.Context,
	run *wandbv2.TriageRun,
	job *batchv1.Job,
	resolved *wandbv2.TriageResolvedExecution,
) (ctrl.Result, error) {
	statusBefore := run.DeepCopy().Status
	run.Status.Phase = wandbv2.TriageRunPhaseRunning
	run.Status.ObservedGeneration = run.Generation
	run.Status.JobRef = &corev1.LocalObjectReference{Name: job.Name}
	run.Status.ResolvedExecution = resolved
	if run.Status.StartedAt == nil {
		switch {
		case job.Status.StartTime != nil:
			run.Status.StartedAt = job.Status.StartTime.DeepCopy()
		case !job.CreationTimestamp.IsZero():
			run.Status.StartedAt = job.CreationTimestamp.DeepCopy()
		default:
			now := metav1.Now()
			run.Status.StartedAt = &now
		}
	}
	apimeta.SetStatusCondition(&run.Status.Conditions, metav1.Condition{
		Type:               triageConditionSucceeded,
		Status:             metav1.ConditionUnknown,
		ObservedGeneration: run.Generation,
		Reason:             "JobRunning",
		Message:            fmt.Sprintf("Job %q is running", job.Name),
	})
	if apiequality.Semantic.DeepEqual(statusBefore, run.Status) {
		return ctrl.Result{}, nil
	}
	if err := r.Status().Update(ctx, run); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TriageRunReconciler) completeRun(
	ctx context.Context,
	run *wandbv2.TriageRun,
	job *batchv1.Job,
	resolved *wandbv2.TriageResolvedExecution,
	results []wandbv2.TriageCheckResult,
) (ctrl.Result, error) {
	run.Status.Phase = wandbv2.TriageRunPhaseSucceeded
	run.Status.ObservedGeneration = run.Generation
	run.Status.JobRef = &corev1.LocalObjectReference{Name: job.Name}
	run.Status.ResolvedExecution = resolved
	run.Status.Results = results
	run.Status.Summary = summarizeTriageResults(results)
	run.Status.StartedAt = triageStartTime(run, job)
	run.Status.CompletedAt = triageCompletionTime(job)
	apimeta.SetStatusCondition(&run.Status.Conditions, metav1.Condition{
		Type:               triageConditionSucceeded,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: run.Generation,
		Reason:             "ResultsCollected",
		Message:            fmt.Sprintf("Collected %d triage check results", len(results)),
	})
	if err := r.Status().Update(ctx, run); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TriageRunReconciler) failRun(
	ctx context.Context,
	run *wandbv2.TriageRun,
	reason string,
	message string,
) (ctrl.Result, error) {
	return r.failRunWithJob(ctx, run, nil, run.Status.ResolvedExecution, reason, message)
}

func (r *TriageRunReconciler) failRunWithJob(
	ctx context.Context,
	run *wandbv2.TriageRun,
	job *batchv1.Job,
	resolved *wandbv2.TriageResolvedExecution,
	reason string,
	message string,
) (ctrl.Result, error) {
	run.Status.Phase = wandbv2.TriageRunPhaseFailed
	run.Status.ObservedGeneration = run.Generation
	run.Status.ResolvedExecution = resolved
	now := metav1.Now()
	run.Status.CompletedAt = &now
	if job != nil {
		run.Status.JobRef = &corev1.LocalObjectReference{Name: job.Name}
		run.Status.StartedAt = triageStartTime(run, job)
		if completedAt := triageCompletionTime(job); completedAt != nil {
			run.Status.CompletedAt = completedAt
		}
	}
	apimeta.SetStatusCondition(&run.Status.Conditions, metav1.Condition{
		Type:               triageConditionSucceeded,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: run.Generation,
		Reason:             reason,
		Message:            message,
	})
	if err := r.Status().Update(ctx, run); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func triageStartTime(run *wandbv2.TriageRun, job *batchv1.Job) *metav1.Time {
	if job.Status.StartTime != nil {
		return job.Status.StartTime.DeepCopy()
	}
	if run.Status.StartedAt != nil {
		return run.Status.StartedAt.DeepCopy()
	}
	if !job.CreationTimestamp.IsZero() {
		return job.CreationTimestamp.DeepCopy()
	}
	return nil
}

func triageCompletionTime(job *batchv1.Job) *metav1.Time {
	if job.Status.CompletionTime != nil {
		return job.Status.CompletionTime.DeepCopy()
	}
	now := metav1.Now()
	return &now
}

func jobComplete(job *batchv1.Job) bool {
	return jobConditionTrue(job, batchv1.JobComplete)
}

func jobFailed(job *batchv1.Job) bool {
	return jobConditionTrue(job, batchv1.JobFailed)
}

func jobConditionTrue(job *batchv1.Job, conditionType batchv1.JobConditionType) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == conditionType && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func jobConditionMessage(job *batchv1.Job, conditionType batchv1.JobConditionType) string {
	for _, condition := range job.Status.Conditions {
		if condition.Type == conditionType && condition.Status == corev1.ConditionTrue {
			return condition.Message
		}
	}
	return ""
}

func isTerminalTriagePhase(phase wandbv2.TriageRunPhase) bool {
	return phase == wandbv2.TriageRunPhaseSucceeded || phase == wandbv2.TriageRunPhaseFailed
}

func (r *TriageRunReconciler) collectTriageResults(
	ctx context.Context,
	job *batchv1.Job,
) ([]wandbv2.TriageCheckResult, error) {
	if r.PodLogs == nil {
		return nil, errors.New("pod log reader is not configured")
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods,
		client.InNamespace(job.Namespace),
		client.MatchingLabels{"batch.kubernetes.io/job-name": job.Name},
	); err != nil {
		return nil, &triageOutputUnavailableError{
			err: fmt.Errorf("list pods for Job %q: %w", job.Name, err),
		}
	}
	if len(pods.Items) == 0 {
		return nil, &triageOutputUnavailableError{
			err: fmt.Errorf("pod for Job %q is not available yet", job.Name),
		}
	}
	if len(pods.Items) > 1 {
		return nil, fmt.Errorf("expected one pod for Job %q, found %d", job.Name, len(pods.Items))
	}

	output, err := r.PodLogs.ReadPodLogs(
		ctx, job.Namespace, pods.Items[0].Name, triageContainerName, maxTriageOutputBytes)
	if err != nil {
		var tooLarge *triageOutputTooLargeError
		if errors.As(err, &tooLarge) {
			return nil, err
		}
		return nil, &triageOutputUnavailableError{
			err: fmt.Errorf("read triage output: %w", err),
		}
	}
	results, err := parseTriageJSONL(output)
	if err != nil {
		return nil, fmt.Errorf("parse triage output: %w", err)
	}
	return results, nil
}

type triageOutputUnavailableError struct {
	err error
}

func (e *triageOutputUnavailableError) Error() string {
	return e.err.Error()
}

func (e *triageOutputUnavailableError) Unwrap() error {
	return e.err
}

type triageOutputTooLargeError struct {
	maxBytes int64
}

func (e *triageOutputTooLargeError) Error() string {
	return fmt.Sprintf("triage output exceeds %d bytes", e.maxBytes)
}

type triageJSONResult struct {
	Name        string          `json:"name"`
	Umbrella    string          `json:"umbrella,omitempty"`
	Severity    string          `json:"severity"`
	Message     string          `json:"message,omitempty"`
	Evidence    json.RawMessage `json:"evidence,omitempty"`
	Remediation string          `json:"remediation,omitempty"`
	StartedAt   string          `json:"started_at,omitempty"`
	EndedAt     string          `json:"ended_at,omitempty"`
	DurationMS  int64           `json:"duration_ms,omitempty"`
}

func parseTriageJSONL(output []byte) ([]wandbv2.TriageCheckResult, error) {
	if int64(len(output)) > maxTriageOutputBytes {
		return nil, fmt.Errorf("triage output exceeds %d bytes", maxTriageOutputBytes)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 64*1024), int(maxTriageOutputBytes))
	results := make([]wandbv2.TriageCheckResult, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var raw triageJSONResult
		decoder := json.NewDecoder(bytes.NewReader(line))
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("line %d is not valid JSON: %w", lineNumber, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("line %d contains more than one JSON value", lineNumber)
		}
		result, err := raw.toAPIResult()
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		results = append(results, result)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.New("triage command emitted no check results")
	}
	return results, nil
}

func (r triageJSONResult) toAPIResult() (wandbv2.TriageCheckResult, error) {
	if strings.TrimSpace(r.Name) == "" {
		return wandbv2.TriageCheckResult{}, errors.New("name is required")
	}
	severity := wandbv2.TriageSeverity(r.Severity)
	switch severity {
	case wandbv2.TriageSeverityPass,
		wandbv2.TriageSeverityWarn,
		wandbv2.TriageSeverityFail,
		wandbv2.TriageSeverityError:
	default:
		return wandbv2.TriageCheckResult{}, fmt.Errorf("unsupported severity %q", r.Severity)
	}

	result := wandbv2.TriageCheckResult{
		Name:                 r.Name,
		Umbrella:             r.Umbrella,
		Severity:             severity,
		Message:              r.Message,
		Remediation:          r.Remediation,
		DurationMilliseconds: r.DurationMS,
	}
	if len(r.Evidence) > 0 && !bytes.Equal(r.Evidence, []byte("null")) {
		if !json.Valid(r.Evidence) {
			return wandbv2.TriageCheckResult{}, errors.New("evidence is not valid JSON")
		}
		result.Evidence = &apiextensionsv1.JSON{Raw: append([]byte(nil), r.Evidence...)}
	}

	var err error
	result.StartedAt, err = parseTriageTimestamp("started_at", r.StartedAt)
	if err != nil {
		return wandbv2.TriageCheckResult{}, err
	}
	result.EndedAt, err = parseTriageTimestamp("ended_at", r.EndedAt)
	if err != nil {
		return wandbv2.TriageCheckResult{}, err
	}
	return result, nil
}

func parseTriageTimestamp(field, value string) (*metav1.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, fmt.Errorf("%s must be RFC3339: %w", field, err)
	}
	timestamp := metav1.NewTime(parsed)
	return &timestamp, nil
}

func summarizeTriageResults(results []wandbv2.TriageCheckResult) *wandbv2.TriageRunSummary {
	summary := &wandbv2.TriageRunSummary{Total: int32(len(results))}
	for _, result := range results {
		switch result.Severity {
		case wandbv2.TriageSeverityPass:
			summary.Pass++
		case wandbv2.TriageSeverityWarn:
			summary.Warn++
		case wandbv2.TriageSeverityFail:
			summary.Fail++
		case wandbv2.TriageSeverityError:
			summary.Error++
		}
	}
	switch {
	case summary.Error > 0:
		summary.OverallSeverity = wandbv2.TriageSeverityError
	case summary.Fail > 0:
		summary.OverallSeverity = wandbv2.TriageSeverityFail
	case summary.Warn > 0:
		summary.OverallSeverity = wandbv2.TriageSeverityWarn
	default:
		summary.OverallSeverity = wandbv2.TriageSeverityPass
	}
	return summary
}

func (r *TriageRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wandbv2.TriageRun{}).
		Owns(&batchv1.Job{}).
		Named("triagerun").
		Complete(r)
}
