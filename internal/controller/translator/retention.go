package translator

import (
	"k8s.io/apimachinery/pkg/labels"
)

type OnDeletePolicy string

const (
	Purge  OnDeletePolicy = "purge"
	Detach OnDeletePolicy = "detach"
)

const (
	WandbNameLabel      = "weightsandbiases.apps.wandb.com/name"
	WandbNamespaceLabel = "weightsandbiases.apps.wandb.com/namespace"
	WandbComponentLabel = "weightsandbiases.apps.wandb.com/component"
)

type OnDeleteRule struct {
	Policy   OnDeletePolicy
	Selector labels.Selector
}
