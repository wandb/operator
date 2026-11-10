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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	maxActionOutputBytes       = int64(512 * 1024)
	maxActionStatusBytes       = 512 * 1024
	actionResultsRetryInterval = 2 * time.Second
	actionResultsGracePeriod   = 30 * time.Second
	actionConditionSucceeded   = "Succeeded"
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

// The controller only reads ActionRuns; create/delete allow the manager to
// delegate those verbs to its install-scoped Watchtower ServiceAccount.
// +kubebuilder:rbac:groups=apps.wandb.com,resources=actionruns,verbs=get;list;watch;create;delete
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

	job, err := r.getOrCreateActionJob(ctx, &run)
	if err != nil {
		var missing *actionJobMissingError
		if errors.As(err, &missing) {
			return r.failRun(ctx, &run, "JobMissing", missing.Error())
		}
		var applicationMissing *actionApplicationMissingError
		if errors.As(err, &applicationMissing) {
			return r.failRun(ctx, &run, "ApplicationNotFound", applicationMissing.Error())
		}
		var invalidAction *invalidActionError
		if errors.As(err, &invalidAction) {
			return r.failRun(ctx, &run, "InvalidAction", invalidAction.Error())
		}
		return ctrl.Result{}, err
	}
	if !metav1.IsControlledBy(job, &run) {
		return r.failRun(ctx, &run, "JobNameCollision",
			fmt.Sprintf("Job %q already exists and is not owned by this ActionRun", job.Name))
	}
	resolved, err := resolvedActionExecutionFromJob(job, run.Status.ResolvedExecution)
	if err != nil {
		return r.failRun(ctx, &run, "InvalidJob", err.Error())
	}

	statusBefore := run.DeepCopy().Status
	requeueForOutput := r.updateRunStatus(ctx, &run, resolved, job)
	if constrainActionStatusSize(&run, job) {
		requeueForOutput = false
	}
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

func resolveActionExecution(
	run *wandbv2.ActionRun,
	application *wandbv2.Application,
) (*actionExecutionPlan, error) {
	switch run.Spec.Type {
	case wandbv2.ActionTypeTriage:
		return resolveTriageExecution(run, application)
	default:
		return nil, fmt.Errorf("action type %q is not executable in this release", run.Spec.Type)
	}
}

func (r *ActionRunReconciler) updateRunStatus(
	ctx context.Context,
	run *wandbv2.ActionRun,
	resolved *wandbv2.ActionResolvedExecution,
	job *batchv1.Job,
) bool {
	run.Status.Phase = wandbv2.ActionRunPhaseRunning
	run.Status.ObservedGeneration = run.Generation
	run.Status.JobRef = &corev1.LocalObjectReference{Name: job.Name}
	run.Status.ResolvedExecution = resolved
	run.Status.StartedAt = actionStartTime(run.Status.StartedAt, job)
	run.Status.CompletedAt = nil
	run.Status.Summary = nil
	run.Status.Results = nil

	if jobFailed(job) {
		message := jobConditionMessage(job, batchv1.JobFailed)
		if message == "" {
			message = fmt.Sprintf("Job %q failed", job.Name)
		}
		results, err := r.collectActionResults(ctx, job)
		if err != nil {
			var unavailable *actionOutputUnavailableError
			if errors.As(err, &unavailable) && !actionResultsGracePeriodExpired(job, time.Now()) {
				setActionCondition(run, metav1.ConditionUnknown, "ResultsPending",
					fmt.Sprintf("%s; waiting for action results: %s", message, unavailable.Error()))
				return true
			}
			run.Status.Phase = wandbv2.ActionRunPhaseFailed
			run.Status.CompletedAt = actionCompletionTime(job)
			if errors.As(err, &unavailable) {
				message = fmt.Sprintf("%s; action results remained unavailable for %s: %s",
					message, actionResultsGracePeriod, unavailable.Error())
			} else {
				message = fmt.Sprintf("%s; action results were invalid: %s", message, err)
			}
			setActionCondition(run, metav1.ConditionFalse, "JobFailed", message)
			return false
		}
		run.Status.Phase = wandbv2.ActionRunPhaseFailed
		run.Status.CompletedAt = actionCompletionTime(job)
		run.Status.Results = results
		run.Status.Summary = summarizeActionResults(results)
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

// constrainActionStatusSize keeps structured output from turning every status
// update into an oversized request. The Job result remains authoritative even
// when the structured details cannot safely fit in the ActionRun object.
func constrainActionStatusSize(run *wandbv2.ActionRun, job *batchv1.Job) bool {
	encoded, err := json.Marshal(run.Status)
	if err == nil && len(encoded) <= maxActionStatusBytes {
		return false
	}

	run.Status.Results = nil
	run.Status.Summary = nil
	detail := fmt.Sprintf("structured action results exceed the %d-byte ActionRun status limit",
		maxActionStatusBytes)
	if err != nil {
		detail = fmt.Sprintf("structured action results could not be encoded: %s", err)
	}
	if jobFailed(job) {
		message := jobConditionMessage(job, batchv1.JobFailed)
		if message == "" {
			message = fmt.Sprintf("Job %q failed", job.Name)
		}
		setActionCondition(run, metav1.ConditionFalse, "JobFailed",
			fmt.Sprintf("%s; results omitted: %s", message, detail))
		return true
	}

	run.Status.Phase = wandbv2.ActionRunPhaseFailed
	run.Status.CompletedAt = actionCompletionTime(job)
	setActionCondition(run, metav1.ConditionFalse, "InvalidResults", detail)
	return true
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
	for _, condition := range job.Status.Conditions {
		if (condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed) &&
			condition.Status == corev1.ConditionTrue && !condition.LastTransitionTime.IsZero() {
			return condition.LastTransitionTime.DeepCopy()
		}
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
		if (condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed) &&
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

	if len(job.Spec.Template.Spec.Containers) != 1 {
		return nil, fmt.Errorf("expected one action container in Job %q, found %d",
			job.Name, len(job.Spec.Template.Spec.Containers))
	}
	output, err := r.PodLogs.ReadPodLogs(
		ctx, job.Namespace, pods.Items[0].Name,
		job.Spec.Template.Spec.Containers[0].Name, maxActionOutputBytes)
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
		// Kubernetes exposes a container's stdout and stderr as one log stream.
		// Ignore ordinary diagnostic lines, but keep JSON-looking lines strict so
		// a malformed result cannot be silently dropped.
		if line[0] != '{' {
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
