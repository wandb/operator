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
	WandbNameLabel      = "apps.wandb.com/name"
	WandbNamespaceLabel = "apps.wandb.com/namespace"
	WandbModuleLabel    = "apps.wandb.com/module"
)

type OnDeleteRule struct {
	Policy   OnDeletePolicy
	Selector labels.Selector
}
