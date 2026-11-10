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
	"strconv"

	wandbv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	actionRunLabel                        = "apps.wandb.com/action-run"
	actionApplicationLabel                = "apps.wandb.com/action-application"
	actionTypeAnnotation                  = "apps.wandb.com/action-type"
	actionNameAnnotation                  = "apps.wandb.com/action-name"
	actionApplicationGenerationAnnotation = "apps.wandb.com/action-application-generation"
	actionTypeEnv                         = "WANDB_ACTION_TYPE"
	actionNameEnv                         = "WANDB_ACTION_NAME"
)

// actionExecutionPlan is the type-independent input used to construct an
// ActionRun Job. Type-specific resolvers must fully resolve their Application
// configuration before returning a plan.
type actionExecutionPlan struct {
	actionType     wandbv2.ActionType
	name           wandbv2.ActionName
	container      corev1.Container
	timeoutSeconds int64
}

func (r *ActionRunReconciler) getOrCreateActionJob(
	ctx context.Context,
	run *wandbv2.ActionRun,
) (*batchv1.Job, error) {
	jobName := common.FitDefaultInfraName(run.Name, "-action", 63)
	if run.Status.JobRef != nil {
		jobName = run.Status.JobRef.Name
	}
	var job batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Namespace: run.Namespace, Name: jobName}, &job)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if err == nil {
		return &job, nil
	}
	if run.Status.JobRef != nil {
		return nil, &actionJobMissingError{name: run.Status.JobRef.Name}
	}

	var application wandbv2.Application
	applicationKey := types.NamespacedName{
		Namespace: run.Namespace,
		Name:      run.Spec.ApplicationRef.Name,
	}
	if err := r.Get(ctx, applicationKey, &application); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &actionApplicationMissingError{name: applicationKey.Name, namespace: applicationKey.Namespace}
		}
		return nil, err
	}
	plan, err := resolveActionExecution(run, &application)
	if err != nil {
		return nil, &invalidActionError{err: err}
	}
	job = *buildActionJob(run, &application, plan)
	if err := controllerutil.SetControllerReference(run, &job, r.Scheme); err != nil {
		return nil, err
	}
	if err := r.Create(ctx, &job); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: run.Namespace,
			Name:      jobName,
		}, &job); err != nil {
			return nil, err
		}
	}
	return &job, nil
}

type actionApplicationMissingError struct {
	name      string
	namespace string
}

func (e *actionApplicationMissingError) Error() string {
	return fmt.Sprintf("Application %q does not exist in namespace %q", e.name, e.namespace)
}

type invalidActionError struct {
	err error
}

func (e *invalidActionError) Error() string {
	return e.err.Error()
}

func (e *invalidActionError) Unwrap() error {
	return e.err
}

type actionJobMissingError struct {
	name string
}

func (e *actionJobMissingError) Error() string {
	return fmt.Sprintf("Job %q disappeared before the action completed", e.name)
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
	plan *actionExecutionPlan,
) *batchv1.Job {
	backoffLimit := int32(0)
	podSpec := application.Spec.PodTemplate.Spec.DeepCopy()
	podSpec.RestartPolicy = corev1.RestartPolicyNever
	podSpec.InitContainers = nil
	podSpec.EphemeralContainers = nil
	podSpec.ReadinessGates = nil
	actionContainer := plan.container.DeepCopy()
	actionContainer.Env = mergeActionEnv(actionContainer.Env, []corev1.EnvVar{
		{Name: actionTypeEnv, Value: string(plan.actionType)},
		{Name: actionNameEnv, Value: string(plan.name)},
	})
	podSpec.Containers = []corev1.Container{*actionContainer}

	labels := map[string]string{
		actionRunLabel:         common.FitDefaultInfraName(run.Name, "", 63),
		actionApplicationLabel: common.FitDefaultInfraName(application.Name, "", 63),
	}
	annotations := map[string]string{
		actionTypeAnnotation:                  string(plan.actionType),
		actionNameAnnotation:                  string(plan.name),
		actionApplicationGenerationAnnotation: strconv.FormatInt(application.Generation, 10),
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        common.FitDefaultInfraName(run.Name, "-action", 63),
			Namespace:   run.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &plan.timeoutSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels, Annotations: annotations},
				Spec:       *podSpec,
			},
		},
	}
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

func resolvedActionExecutionFromJob(
	job *batchv1.Job,
	previous *wandbv2.ActionResolvedExecution,
) (*wandbv2.ActionResolvedExecution, error) {
	if len(job.Spec.Template.Spec.Containers) != 1 {
		return nil, fmt.Errorf("Job %q must contain exactly one action container, found %d",
			job.Name, len(job.Spec.Template.Spec.Containers))
	}
	container := job.Spec.Template.Spec.Containers[0]
	var applicationGeneration int64
	if encodedGeneration := job.Annotations[actionApplicationGenerationAnnotation]; encodedGeneration != "" {
		var err error
		applicationGeneration, err = strconv.ParseInt(encodedGeneration, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Job %q has invalid %q annotation: %w",
				job.Name, actionApplicationGenerationAnnotation, err)
		}
	} else if previous != nil {
		// Jobs created before the generation annotation was introduced can use
		// the first snapshot already persisted on the run.
		applicationGeneration = previous.ApplicationGeneration
	}
	var timeoutSeconds int64
	if job.Spec.ActiveDeadlineSeconds != nil {
		timeoutSeconds = *job.Spec.ActiveDeadlineSeconds
	}
	return &wandbv2.ActionResolvedExecution{
		ApplicationGeneration: applicationGeneration,
		ContainerName:         container.Name,
		Image:                 container.Image,
		Command:               append([]string(nil), container.Command...),
		Args:                  append([]string(nil), container.Args...),
		TimeoutSeconds:        timeoutSeconds,
	}, nil
}
