package v2

import (
	wandbv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/labels"
)
import "github.com/wandb/operator/internal/controller/translator"

func ToOnDeleteRule(
	wandb *wandbv2.WeightsAndBiases,
	retentionPolicy wandbv2.RetentionPolicy,
	componentName string,
) translator.OnDeleteRule {
	policy := translator.Preserve
	if retentionPolicy.OnDelete == wandbv2.PurgeOnDelete {
		policy = translator.Purge
	}
	selector := labels.SelectorFromSet(BuildWandbLabels(wandb, componentName))
	return translator.OnDeleteRule{
		Policy:   policy,
		Selector: selector,
	}
}

func BuildWandbLabels(wandb *wandbv2.WeightsAndBiases, componentName string) map[string]string {
	return map[string]string{
		translator.WandbNameLabel:      wandb.Name,
		translator.WandbNamespaceLabel: wandb.Namespace,
		translator.WandbComponentLabel: componentName,
	}
}
