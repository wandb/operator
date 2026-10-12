package common

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
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
// on behalf of a deployment. Keep these legacy keys stable: PVC deletion
// selectors and existing installations depend on them.
func BuildWandbLabels(owner client.Object, componentName string) map[string]string {
	return map[string]string{
		WandbNameLabel:      owner.GetName(),
		WandbNamespaceLabel: owner.GetNamespace(),
		WandbComponentLabel: componentName,
	}
}
