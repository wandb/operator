package v2

// BestEffortResources reports whether the operator should drop every resource
// request and limit that it would otherwise derive from the server manifest
// sizing tables, manifest container specs, or its own built-in defaults, so
// that all pods run with BestEffort QoS.
//
// This is the policy for spec.size dev: a dev install is meant to fit on
// whatever a laptop or a single Kind node can spare, so no workload — app,
// managed infrastructure, or sidecar — gets a scheduler-visible footprint.
// Values the user sets explicitly on the CR (legacyOverrides resources, managed
// infra config.resources) are not affected; they remain the escape hatch.
func (s *WeightsAndBiasesSpec) BestEffortResources() bool {
	return s.Size == SizeDev
}
