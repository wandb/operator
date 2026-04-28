package common

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/labels"
)

type OnDeletePolicy string

const (
	Purge  OnDeletePolicy = "purge"
	Detach OnDeletePolicy = "detach"
)

type OnDeleteRule struct {
	Policy   OnDeletePolicy
	Selector labels.Selector
}

// ToOnDeleteRule maps the user-facing RetentionPolicy onto an OnDeleteRule
// scoped to resources labelled with the given component name.
func ToOnDeleteRule(
	wandb *apiv2.WeightsAndBiases,
	retentionPolicy apiv2.RetentionPolicy,
	componentName string,
) OnDeleteRule {
	policy := Detach
	if retentionPolicy.OnDelete == apiv2.PurgeOnDelete {
		policy = Purge
	}
	return OnDeleteRule{
		Policy:   policy,
		Selector: labels.SelectorFromSet(BuildWandbLabels(wandb, componentName)),
	}
}
