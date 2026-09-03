package reconciler

import (
	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
)

func resolveApplicationTriage(triage *serverManifest.ApplicationTriage) *apiv2.ApplicationTriageSpec {
	if triage == nil {
		return nil
	}

	var env []corev1.EnvVar
	if len(triage.Env) > 0 {
		env = make([]corev1.EnvVar, len(triage.Env))
		for i := range triage.Env {
			env[i] = *triage.Env[i].DeepCopy()
		}
	}
	actions := make([]apiv2.ApplicationActionSpec, len(triage.Actions))
	for i := range triage.Actions {
		actions[i] = apiv2.ApplicationActionSpec{
			Name:        apiv2.ActionName(triage.Actions[i].Name),
			Description: triage.Actions[i].Description,
			Args:        append([]string(nil), triage.Actions[i].Args...),
		}
	}
	resolved := &apiv2.ApplicationTriageSpec{
		ContainerName:  triage.ContainerName,
		Command:        append([]string(nil), triage.Command...),
		Args:           append([]string(nil), triage.Args...),
		Env:            env,
		TimeoutSeconds: triage.TimeoutSeconds,
		Actions:        actions,
	}
	if triage.Resources != nil {
		resolved.Resources = triage.Resources.DeepCopy()
	}
	return resolved
}
