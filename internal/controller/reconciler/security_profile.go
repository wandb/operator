package reconciler

import apiv2 "github.com/wandb/operator/api/v2"

// Default runtime settings remain unset until supported server images can use
// them by default. Changing this default does not change the CR override contract.
func defaultWorkloadSecurityProfile() apiv2.WorkloadSecurityProfile {
	return apiv2.WorkloadSecurityProfile{}
}

func resolveWorkloadSecurityProfile(manifest, shared, workload apiv2.WorkloadSecurityProfile) apiv2.WorkloadSecurityProfile {
	return defaultWorkloadSecurityProfile().
		WithOverrides(manifest).
		WithOverrides(shared).
		WithOverrides(workload)
}
