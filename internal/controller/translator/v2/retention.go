package v2

import (
	wandbv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/labels"
)
import "github.com/wandb/operator/internal/controller/translator"

func ToOnDeleteRule(
	wandb *wandbv2.WeightsAndBiases,
	retentionPolicy wandbv2.WBRetentionPolicy,
	moduleName string,
) translator.OnDeleteRule {
	policy := translator.Preserve
	if retentionPolicy.OnDelete == wandbv2.WBPurgeOnDelete {
		policy = translator.Purge
	}
	selector := labels.SelectorFromSet(BuildWandbLabels(wandb, moduleName))
	return translator.OnDeleteRule{
		Policy:   policy,
		Selector: selector,
	}
}

func BuildWandbLabels(wandb *wandbv2.WeightsAndBiases, moduleName string) map[string]string {
	return map[string]string{
		translator.WandbNameLabel:      wandb.Name,
		translator.WandbNamespaceLabel: wandb.Namespace,
		translator.WandbModuleLabel:    moduleName,
	}
}
