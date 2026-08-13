package common

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
)

const (
	WandbNameLabel      = "weightsandbiases.apps.wandb.com/name"
	WandbNamespaceLabel = "weightsandbiases.apps.wandb.com/namespace"
	WandbComponentLabel = "weightsandbiases.apps.wandb.com/component"

	AppNameLabel      = "app.kubernetes.io/name"
	AppInstanceLabel  = "app.kubernetes.io/instance"
	AppPartOfLabel    = "app.kubernetes.io/part-of"
	AppManagedByLabel = "app.kubernetes.io/managed-by"

	PartOfWandb            = "wandb"
	ManagedByWandbOperator = "wandb-operator"
)

// HasAllLabelKeys reports whether existing has every key in desired.
func HasAllLabelKeys(existing, desired map[string]string) bool {
	for k := range desired {
		if _, ok := existing[k]; !ok {
			return false
		}
	}
	return true
}

// BuildWandbLabels returns ownership labels; componentName is the service id.
func BuildWandbLabels(wandb *apiv2.WeightsAndBiases, componentName string) map[string]string {
	return map[string]string{
		WandbNameLabel:      wandb.Name,
		WandbNamespaceLabel: wandb.Namespace,
		WandbComponentLabel: componentName,
	}
}

// BuildIdentityLabels returns app.kubernetes.io identity labels for a release.
func BuildIdentityLabels(wandb *apiv2.WeightsAndBiases, serviceName string) map[string]string {
	return map[string]string{
		AppNameLabel:      serviceName,
		AppInstanceLabel:  wandb.Name,
		AppPartOfLabel:    PartOfWandb,
		AppManagedByLabel: ManagedByWandbOperator,
	}
}

// BuildApplicationLabels merges ownership+identity for MetaTemplate only.
func BuildApplicationLabels(wandb *apiv2.WeightsAndBiases, serviceName string) map[string]string {
	return utils.MergeMapsStringString(
		BuildWandbLabels(wandb, serviceName),
		BuildIdentityLabels(wandb, serviceName),
	)
}
