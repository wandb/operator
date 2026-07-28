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

	actions := make(map[string]apiv2.TriageActionSpec, len(triage.Actions))
	for name, action := range triage.Actions {
		env := make([]corev1.EnvVar, len(action.Env))
		for i := range action.Env {
			env[i] = *action.Env[i].DeepCopy()
		}
		resolved := apiv2.TriageActionSpec{
			ContainerName:  action.ContainerName,
			Command:        append([]string(nil), action.Command...),
			Args:           append([]string(nil), action.Args...),
			Env:            env,
			TimeoutSeconds: action.TimeoutSeconds,
		}
		if action.Resources != nil {
			resolved.Resources = action.Resources.DeepCopy()
		}
		actions[name] = resolved
	}

	return &apiv2.ApplicationTriageSpec{Actions: actions}
}
