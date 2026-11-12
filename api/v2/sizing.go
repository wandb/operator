package v2

// BestEffortResources is true for size dev: drop all derived requests/limits.
func (s *WeightsAndBiasesSpec) BestEffortResources() bool {
	return s.Size == SizeDev
}
