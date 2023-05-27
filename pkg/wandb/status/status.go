package status

type ConditionType string

// Name returns the name of the condition.
func (c ConditionType) Name() string {
	return string(c)
}

const (
	ConditionInitialized ConditionType = "Initialized"
	ConditionUpgrading   ConditionType = "Upgrading"
	ConditionAvailable   ConditionType = "Available"
)
