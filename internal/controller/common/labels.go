package common

import (
	apiv2 "github.com/wandb/operator/api/v2"
)

const (
	WandbNameLabel      = "weightsandbiases.apps.wandb.com/name"
	WandbNamespaceLabel = "weightsandbiases.apps.wandb.com/namespace"
	WandbComponentLabel = "weightsandbiases.apps.wandb.com/component"
)

// HasAllLabelKeys reports whether existing contains every key present in desired,
// regardless of value.
func HasAllLabelKeys(existing, desired map[string]string) bool {
	for k := range desired {
		if _, ok := existing[k]; !ok {
			return false
		}
	}
	return true
}

// BuildWandbLabels returns the standard wandb labels for resources managed
// on behalf of the given WeightsAndBiases CR.
func BuildWandbLabels(wandb *apiv2.WeightsAndBiases, componentName string) map[string]string {
	return map[string]string{
		WandbNameLabel:      wandb.Name,
		WandbNamespaceLabel: wandb.Namespace,
		WandbComponentLabel: componentName,
	}
}
