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
	"fmt"

	wandbv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
)

const defaultTriageTimeoutSeconds = int64(300)

func resolveTriageExecution(
	run *wandbv2.ActionRun,
	application *wandbv2.Application,
) (*actionExecutionPlan, error) {
	actionName := run.Spec.Action.Name
	actionSpec, err := findTriageAction(application, actionName)
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
		timeoutSeconds = defaultTriageTimeoutSeconds
	}
	return &actionExecutionPlan{
		actionType:     run.Spec.Type,
		name:           actionName,
		container:      buildTriageContainer(source, runner, actionSpec),
		timeoutSeconds: timeoutSeconds,
	}, nil
}

func findTriageAction(
	application *wandbv2.Application,
	actionName wandbv2.ActionName,
) (wandbv2.ApplicationTriageActionSpec, error) {
	if application.Spec.Triage == nil {
		return wandbv2.ApplicationTriageActionSpec{}, fmt.Errorf(
			"application %q does not declare triage actions", application.Name)
	}
	if len(application.Spec.Triage.Command) == 0 && len(application.Spec.Triage.Args) == 0 {
		return wandbv2.ApplicationTriageActionSpec{}, fmt.Errorf(
			"application %q triage runner must override command or args", application.Name)
	}
	for i := range application.Spec.Triage.Actions {
		action := application.Spec.Triage.Actions[i]
		if action.Name == actionName {
			return action, nil
		}
	}
	return wandbv2.ApplicationTriageActionSpec{}, fmt.Errorf(
		"application %q does not declare triage action %q", application.Name, actionName)
}

func buildTriageContainer(
	source *corev1.Container,
	runner *wandbv2.ApplicationTriageSpec,
	action wandbv2.ApplicationTriageActionSpec,
) corev1.Container {
	container := corev1.Container{
		Name:                     source.Name,
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
	container.Args = append(container.Args, action.Args...)
	if runner.Resources != nil {
		container.Resources = *runner.Resources.DeepCopy()
	}
	return container
}
