package v2

// WorkloadSecurityProfile configures image-dependent workload security settings.
// Unset fields inherit from the lower-priority profile; explicit false overrides it.
type WorkloadSecurityProfile struct {
	// +optional
	RunAsNonRoot *bool `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	// +optional
	ReadOnlyRootFilesystem *bool `json:"readOnlyRootFilesystem,omitempty" yaml:"readOnlyRootFilesystem,omitempty"`
}

// WithOverrides returns a profile with explicitly supplied fields replaced.
// Neither input is modified.
func (profile WorkloadSecurityProfile) WithOverrides(overrides WorkloadSecurityProfile) WorkloadSecurityProfile {
	if overrides.RunAsNonRoot != nil {
		value := *overrides.RunAsNonRoot
		profile.RunAsNonRoot = &value
	}
	if overrides.ReadOnlyRootFilesystem != nil {
		value := *overrides.ReadOnlyRootFilesystem
		profile.ReadOnlyRootFilesystem = &value
	}
	return profile
}
