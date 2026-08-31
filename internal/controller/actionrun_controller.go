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
	defaultActionTimeoutSeconds = int64(300)
	maxActionOutputBytes        = int64(512 * 1024)
	actionResultsRetryInterval  = 2 * time.Second
	actionResultsGracePeriod    = 30 * time.Second
	actionContainerName         = "action"
	actionConditionSucceeded    = "Succeeded"
	actionRunLabel              = "apps.wandb.com/action-run"
	actionApplicationLabel      = "apps.wandb.com/action-application"
	actionTypeAnnotation        = "apps.wandb.com/action-type"
	actionNameAnnotation        = "apps.wandb.com/action-name"
	actionTypeEnv               = "WANDB_ACTION_TYPE"
	actionNameEnv               = "WANDB_ACTION_NAME"
)

// ActionPodLogReader reads structured output from a completed action pod. It
// is an interface so controller behavior can be tested without a live API
// server's pod log subresource.
type ActionPodLogReader interface {
	ReadPodLogs(
		ctx context.Context,
		namespace string,
		podName string,
		containerName string,
		maxBytes int64,
	) ([]byte, error)
}

type KubernetesActionPodLogReader struct {
	CoreV1 corev1client.CoreV1Interface
}

func (r *KubernetesActionPodLogReader) ReadPodLogs(
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
		return nil, &actionOutputTooLargeError{maxBytes: maxBytes}
	}
	return output, nil
}

// ActionRunReconciler turns one immutable ActionRun into one bounded Job and
// records the Job's structured JSONL output on the run status.
type ActionRunReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	PodLogs ActionPodLogReader
}

// +kubebuilder:rbac:groups=apps.wandb.com,resources=actionruns,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.wandb.com,resources=actionruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.wandb.com,resources=applications,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get

func (r *ActionRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var run wandbv2.ActionRun
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if isTerminalActionPhase(run.Status.Phase) {
		return ctrl.Result{}, nil
	}
	if run.Spec.Type != wandbv2.ActionTypeTriage {
		return r.failRun(ctx, &run, "UnsupportedActionType",
			fmt.Sprintf("action type %q is not executable in this release", run.Spec.Type))
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

	action, err := resolveRequestedAction(&run, &application)
	if err != nil {
		return r.failRun(ctx, &run, "InvalidAction", err.Error())
	}
	job, err := r.getOrCreateActionJob(ctx, &run, &application, action)
	if err != nil {
		var missing *actionJobMissingError
		if errors.As(err, &missing) {
			return r.failRun(ctx, &run, "JobMissing", missing.Error())
		}
		return ctrl.Result{}, err
	}
	if !metav1.IsControlledBy(job, &run) {
		return r.failRun(ctx, &run, "JobNameCollision",
			fmt.Sprintf("Job %q already exists and is not owned by this ActionRun", job.Name))
	}

	statusBefore := run.DeepCopy().Status
	requeueForOutput := r.updateRunStatus(ctx, &run, action, job)
	if !apiequality.Semantic.DeepEqual(statusBefore, run.Status) {
		if err := r.Status().Update(ctx, &run); err != nil {
			return ctrl.Result{}, err
		}
	}
	if requeueForOutput {
		return ctrl.Result{RequeueAfter: actionResultsRetryInterval}, nil
	}
	return ctrl.Result{}, nil
}

type resolvedAction struct {
	actionType     wandbv2.ActionType
	name           wandbv2.ActionName
	runner         *wandbv2.ApplicationTriageSpec
	spec           wandbv2.ApplicationActionSpec
	source         *corev1.Container
	timeoutSeconds int64
	resolved       *wandbv2.ActionResolvedExecution
	jobName        string
}

func resolveRequestedAction(
	run *wandbv2.ActionRun,
	application *wandbv2.Application,
) (*resolvedAction, error) {
	actionName := run.Spec.Action.Name
	actionSpec, err := resolveTriageAction(application, actionName)
	if err != nil {
		return nil, err
	}
	runner := application.Spec.Triage
	source, err := selectActionContainer(application, runner.ContainerName)
	if err != nil {
		return nil, err
	}
	timeoutSeconds := runner.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultActionTimeoutSeconds
	}
	action := &resolvedAction{
		actionType:     run.Spec.Type,
		name:           actionName,
		runner:         runner,
		spec:           actionSpec,
		source:         source,
		timeoutSeconds: timeoutSeconds,
		jobName:        common.FitDefaultInfraName(run.Name, "-action", 63),
	}
	action.resolved = resolvedActionExecution(application, action)
	return action, nil
}

func (r *ActionRunReconciler) getOrCreateActionJob(
	ctx context.Context,
	run *wandbv2.ActionRun,
	application *wandbv2.Application,
	action *resolvedAction,
) (*batchv1.Job, error) {
	var job batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Namespace: run.Namespace, Name: action.jobName}, &job)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if apierrors.IsNotFound(err) {
		if run.Status.JobRef != nil {
			return nil, &actionJobMissingError{name: run.Status.JobRef.Name}
		}
		job = *buildActionJob(run, application, action)
		if err := controllerutil.SetControllerReference(run, &job, r.Scheme); err != nil {
			return nil, err
		}
		if err := r.Create(ctx, &job); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
			if err := r.Get(ctx, types.NamespacedName{
				Namespace: run.Namespace,
				Name:      action.jobName,
			}, &job); err != nil {
				return nil, err
			}
		}
	}
	return &job, nil
}

type actionJobMissingError struct {
	name string
}

func (e *actionJobMissingError) Error() string {
	return fmt.Sprintf("Job %q disappeared before the action completed", e.name)
}

func resolveTriageAction(
	application *wandbv2.Application,
	actionName wandbv2.ActionName,
) (wandbv2.ApplicationActionSpec, error) {
	if application.Spec.Triage == nil {
		return wandbv2.ApplicationActionSpec{}, fmt.Errorf(
			"application %q does not declare triage actions", application.Name)
	}
	if len(application.Spec.Triage.Command) == 0 && len(application.Spec.Triage.Args) == 0 {
		return wandbv2.ApplicationActionSpec{}, fmt.Errorf(
			"application %q triage runner must override command or args", application.Name)
	}
	for i := range application.Spec.Triage.Actions {
		action := application.Spec.Triage.Actions[i]
		if action.Name == actionName {
			return action, nil
		}
	}
	return wandbv2.ApplicationActionSpec{}, fmt.Errorf(
		"application %q does not declare triage action %q", application.Name, actionName)
}

func selectActionContainer(application *wandbv2.Application, name string) (*corev1.Container, error) {
	containers := application.Spec.PodTemplate.Spec.Containers
	if name == "" {
		if len(containers) != 1 {
			return nil, fmt.Errorf(
				"action must select a container because Application %q has %d containers",
				application.Name, len(containers))
		}
		return &containers[0], nil
	}
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i], nil
		}
	}
	return nil, fmt.Errorf("application %q has no container named %q", application.Name, name)
}

func buildActionJob(
	run *wandbv2.ActionRun,
	application *wandbv2.Application,
	action *resolvedAction,
) *batchv1.Job {
	backoffLimit := int32(0)
	podSpec := application.Spec.PodTemplate.Spec.DeepCopy()
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.InitContainers = nil
	podSpec.EphemeralContainers = nil
	podSpec.ReadinessGates = nil
	podSpec.Containers = []corev1.Container{buildActionContainer(action.source, action.runner, action)}

	labels := map[string]string{
		actionRunLabel:         common.FitDefaultInfraName(run.Name, "", 63),
		actionApplicationLabel: common.FitDefaultInfraName(application.Name, "", 63),
	}
	annotations := map[string]string{
		actionTypeAnnotation: string(action.actionType),
		actionNameAnnotation: string(action.name),
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        action.jobName,
			Namespace:   run.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &action.timeoutSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels, Annotations: annotations},
				Spec:       *podSpec,
			},
		},
	}
}

func buildActionContainer(
	source *corev1.Container,
	runner *wandbv2.ApplicationTriageSpec,
	action *resolvedAction,
) corev1.Container {
	container := corev1.Container{
		Name:                     actionContainerName,
		Image:                    source.Image,
		ImagePullPolicy:          source.ImagePullPolicy,
		Command:                  append([]string(nil), source.Command...),
		Args:                     append([]string(nil), source.Args...),
		WorkingDir:               source.WorkingDir,
		EnvFrom:                  append([]corev1.EnvFromSource(nil), source.EnvFrom...),
		Env:                      mergeActionEnv(source.Env, runner.Env),
		Resources:                defaultActionResources(),
		VolumeMounts:             append([]corev1.VolumeMount(nil), source.VolumeMounts...),
		VolumeDevices:            append([]corev1.VolumeDevice(nil), source.VolumeDevices...),
		SecurityContext:          source.SecurityContext.DeepCopy(),
		TerminationMessagePath:   source.TerminationMessagePath,
		TerminationMessagePolicy: source.TerminationMessagePolicy,
	}
	if len(runner.Command) > 0 {
		container.Command = append([]string(nil), runner.Command...)
		container.Args = nil
	}
	if len(runner.Args) > 0 {
		container.Args = append([]string(nil), runner.Args...)
	}
	container.Args = append(container.Args, action.spec.Args...)
	container.Env = mergeActionEnv(container.Env, []corev1.EnvVar{
		{Name: actionTypeEnv, Value: string(action.actionType)},
		{Name: actionNameEnv, Value: string(action.name)},
	})
	if runner.Resources != nil {
		container.Resources = *runner.Resources.DeepCopy()
	}
	return container
}

func mergeActionEnv(inherited, overrides []corev1.EnvVar) []corev1.EnvVar {
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

func defaultActionResources() corev1.ResourceRequirements {
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

func resolvedActionExecution(
	application *wandbv2.Application,
	action *resolvedAction,
) *wandbv2.ActionResolvedExecution {
	container := buildActionContainer(action.source, action.runner, action)
	return &wandbv2.ActionResolvedExecution{
		ApplicationGeneration: application.Generation,
		ContainerName:         action.source.Name,
		Image:                 container.Image,
		Command:               append([]string(nil), container.Command...),
		Args:                  append([]string(nil), container.Args...),
		TimeoutSeconds:        action.timeoutSeconds,
	}
}

func (r *ActionRunReconciler) updateRunStatus(
	ctx context.Context,
	run *wandbv2.ActionRun,
	action *resolvedAction,
	job *batchv1.Job,
) bool {
	run.Status.Phase = wandbv2.ActionRunPhaseRunning
	run.Status.ObservedGeneration = run.Generation
	run.Status.JobRef = &corev1.LocalObjectReference{Name: job.Name}
	run.Status.ResolvedExecution = action.resolved
	run.Status.StartedAt = actionStartTime(run.Status.StartedAt, job)
	run.Status.CompletedAt = nil
	run.Status.Summary = nil
	run.Status.Results = nil

	if jobFailed(job) {
		run.Status.Phase = wandbv2.ActionRunPhaseFailed
		run.Status.CompletedAt = actionCompletionTime(job)
		message := jobConditionMessage(job, batchv1.JobFailed)
		if message == "" {
			message = fmt.Sprintf("Job %q failed", job.Name)
		}
		if results, err := r.collectActionResults(ctx, job); err == nil {
			run.Status.Results = results
			run.Status.Summary = summarizeActionResults(results)
		}
		setActionCondition(run, metav1.ConditionFalse, "JobFailed", message)
		return false
	}
	if !jobComplete(job) {
		setActionCondition(run, metav1.ConditionUnknown, "ActionRunning",
			fmt.Sprintf("Job %q is running", job.Name))
		return false
	}

	results, err := r.collectActionResults(ctx, job)
	if err != nil {
		var unavailable *actionOutputUnavailableError
		if errors.As(err, &unavailable) {
			if actionResultsGracePeriodExpired(job, time.Now()) {
				run.Status.Phase = wandbv2.ActionRunPhaseFailed
				run.Status.CompletedAt = actionCompletionTime(job)
				setActionCondition(run, metav1.ConditionFalse, "ResultsUnavailable",
					fmt.Sprintf("action results remained unavailable for %s after Job completion: %s",
						actionResultsGracePeriod, unavailable.Error()))
				return false
			}
			setActionCondition(run, metav1.ConditionUnknown, "ResultsPending", unavailable.Error())
			return true
		}
		run.Status.Phase = wandbv2.ActionRunPhaseFailed
		run.Status.CompletedAt = actionCompletionTime(job)
		setActionCondition(run, metav1.ConditionFalse, "InvalidResults", err.Error())
		return false
	}

	run.Status.Phase = wandbv2.ActionRunPhaseSucceeded
	run.Status.CompletedAt = actionCompletionTime(job)
	run.Status.Results = results
	run.Status.Summary = summarizeActionResults(results)
	setActionCondition(run, metav1.ConditionTrue, "ResultsCollected",
		fmt.Sprintf("Collected %d action results", len(results)))
	return false
}

func setActionCondition(
	run *wandbv2.ActionRun,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	apimeta.SetStatusCondition(&run.Status.Conditions, metav1.Condition{
		Type:               actionConditionSucceeded,
		Status:             status,
		ObservedGeneration: run.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func (r *ActionRunReconciler) failRun(
	ctx context.Context,
	run *wandbv2.ActionRun,
	reason string,
	message string,
) (ctrl.Result, error) {
	run.Status.Phase = wandbv2.ActionRunPhaseFailed
	run.Status.ObservedGeneration = run.Generation
	now := metav1.Now()
	run.Status.CompletedAt = &now
	setActionCondition(run, metav1.ConditionFalse, reason, message)
	if err := r.Status().Update(ctx, run); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func actionStartTime(previous *metav1.Time, job *batchv1.Job) *metav1.Time {
	if job.Status.StartTime != nil {
		return job.Status.StartTime.DeepCopy()
	}
	if previous != nil {
		return previous.DeepCopy()
	}
	if !job.CreationTimestamp.IsZero() {
		return job.CreationTimestamp.DeepCopy()
	}
	now := metav1.Now()
	return &now
}

func actionCompletionTime(job *batchv1.Job) *metav1.Time {
	if job.Status.CompletionTime != nil {
		return job.Status.CompletionTime.DeepCopy()
	}
	now := metav1.Now()
	return &now
}

func actionResultsGracePeriodExpired(
	job *batchv1.Job,
	now time.Time,
) bool {
	unavailableSince, ok := actionResultsUnavailableSince(job)
	return !ok || !now.Before(unavailableSince.Add(actionResultsGracePeriod))
}

func actionResultsUnavailableSince(job *batchv1.Job) (time.Time, bool) {
	if job.Status.CompletionTime != nil && !job.Status.CompletionTime.IsZero() {
		return job.Status.CompletionTime.Time, true
	}
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete &&
			condition.Status == corev1.ConditionTrue &&
			!condition.LastTransitionTime.IsZero() {
			return condition.LastTransitionTime.Time, true
		}
	}
	return time.Time{}, false
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

func isTerminalActionPhase(phase wandbv2.ActionRunPhase) bool {
	return phase == wandbv2.ActionRunPhaseSucceeded || phase == wandbv2.ActionRunPhaseFailed
}

func (r *ActionRunReconciler) collectActionResults(
	ctx context.Context,
	job *batchv1.Job,
) ([]wandbv2.ActionResult, error) {
	if r.PodLogs == nil {
		return nil, errors.New("pod log reader is not configured")
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods,
		client.InNamespace(job.Namespace),
		client.MatchingLabels{"batch.kubernetes.io/job-name": job.Name},
	); err != nil {
		return nil, &actionOutputUnavailableError{
			err: fmt.Errorf("list pods for Job %q: %w", job.Name, err),
		}
	}
	if len(pods.Items) == 0 {
		return nil, &actionOutputUnavailableError{
			err: fmt.Errorf("pod for Job %q is not available yet", job.Name),
		}
	}
	if len(pods.Items) > 1 {
		return nil, fmt.Errorf("expected one pod for Job %q, found %d", job.Name, len(pods.Items))
	}

	output, err := r.PodLogs.ReadPodLogs(
		ctx, job.Namespace, pods.Items[0].Name, actionContainerName, maxActionOutputBytes)
	if err != nil {
		var tooLarge *actionOutputTooLargeError
		if errors.As(err, &tooLarge) {
			return nil, err
		}
		return nil, &actionOutputUnavailableError{
			err: fmt.Errorf("read action output: %w", err),
		}
	}
	results, err := parseActionJSONL(output)
	if err != nil {
		return nil, fmt.Errorf("parse action output: %w", err)
	}
	return results, nil
}

type actionOutputUnavailableError struct {
	err error
}

func (e *actionOutputUnavailableError) Error() string {
	return e.err.Error()
}

func (e *actionOutputUnavailableError) Unwrap() error {
	return e.err
}

type actionOutputTooLargeError struct {
	maxBytes int64
}

func (e *actionOutputTooLargeError) Error() string {
	return fmt.Sprintf("action output exceeds %d bytes", e.maxBytes)
}

type actionJSONResult struct {
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

func parseActionJSONL(output []byte) ([]wandbv2.ActionResult, error) {
	if int64(len(output)) > maxActionOutputBytes {
		return nil, fmt.Errorf("action output exceeds %d bytes", maxActionOutputBytes)
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 64*1024), int(maxActionOutputBytes))
	results := make([]wandbv2.ActionResult, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var raw actionJSONResult
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
		return nil, errors.New("action command emitted no results")
	}
	return results, nil
}

func (r actionJSONResult) toAPIResult() (wandbv2.ActionResult, error) {
	if strings.TrimSpace(r.Name) == "" {
		return wandbv2.ActionResult{}, errors.New("name is required")
	}
	severity := wandbv2.ActionSeverity(r.Severity)
	switch severity {
	case wandbv2.ActionSeverityPass,
		wandbv2.ActionSeverityWarn,
		wandbv2.ActionSeverityFail,
		wandbv2.ActionSeverityError:
	default:
		return wandbv2.ActionResult{}, fmt.Errorf("unsupported severity %q", r.Severity)
	}

	result := wandbv2.ActionResult{
		Name:                 r.Name,
		Umbrella:             r.Umbrella,
		Severity:             severity,
		Message:              r.Message,
		Remediation:          r.Remediation,
		DurationMilliseconds: r.DurationMS,
	}
	if len(r.Evidence) > 0 && !bytes.Equal(r.Evidence, []byte("null")) {
		if !json.Valid(r.Evidence) {
			return wandbv2.ActionResult{}, errors.New("evidence is not valid JSON")
		}
		result.Evidence = &apiextensionsv1.JSON{Raw: append([]byte(nil), r.Evidence...)}
	}

	var err error
	result.StartedAt, err = parseActionTimestamp("started_at", r.StartedAt)
	if err != nil {
		return wandbv2.ActionResult{}, err
	}
	result.EndedAt, err = parseActionTimestamp("ended_at", r.EndedAt)
	if err != nil {
		return wandbv2.ActionResult{}, err
	}
	return result, nil
}

func parseActionTimestamp(field, value string) (*metav1.Time, error) {
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

func summarizeActionResults(results []wandbv2.ActionResult) *wandbv2.ActionRunSummary {
	summary := &wandbv2.ActionRunSummary{Total: int32(len(results))}
	for _, result := range results {
		switch result.Severity {
		case wandbv2.ActionSeverityPass:
			summary.Pass++
		case wandbv2.ActionSeverityWarn:
			summary.Warn++
		case wandbv2.ActionSeverityFail:
			summary.Fail++
		case wandbv2.ActionSeverityError:
			summary.Error++
		}
	}
	switch {
	case summary.Error > 0:
		summary.OverallSeverity = wandbv2.ActionSeverityError
	case summary.Fail > 0:
		summary.OverallSeverity = wandbv2.ActionSeverityFail
	case summary.Warn > 0:
		summary.OverallSeverity = wandbv2.ActionSeverityWarn
	default:
		summary.OverallSeverity = wandbv2.ActionSeverityPass
	}
	return summary
}

func (r *ActionRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wandbv2.ActionRun{}).
		Owns(&batchv1.Job{}).
		Named("actionrun").
		Complete(r)
}
