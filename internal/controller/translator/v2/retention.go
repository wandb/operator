package v2

import wandbv2 "github.com/wandb/operator/api/v2"
import "github.com/wandb/operator/internal/controller/translator"

func ToOnDeletePolicy(retentionPolicy wandbv2.WBRetentionPolicy) translator.OnDeletePolicy {
	if retentionPolicy.OnDelete == wandbv2.WBPurgeOnDelete {
		return translator.Purge
	}
	return translator.Preserve
}
