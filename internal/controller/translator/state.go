package translator

type WbState string

const (
	ErrorState        WbState = "Error"
	PendingState      WbState = "Pending"
	HealthyState      WbState = "Healthy"
	DegradedState     WbState = "Degraded"
	UnknownState      WbState = "Unknown"
	NotInstalledState WbState = "NotInstalled"
)

var NotReadyStates = []WbState{
	ErrorState, PendingState, NotInstalledState,
}

type ConditionType struct {
	State string
	Type  string
}

type WbErrorType string

const (
	MachineryErrorType        WbErrorType = "MachineryError"
	ResourceConflictErrorType WbErrorType = "ResourceConflictError"
	OperationalErrorType      WbErrorType = "OperationalError"
)

type WbDegradedType string

const (
	JobFailureDegraded         WbDegradedType = "JobFailure"
	ReplicationFailureDegraded WbDegradedType = "ReplicationFailure"
)

type WbPendingType string

const (
	ConfigurationUpdatePending WbPendingType = "ResourceUpdate"
	ResourceDeletionPending    WbPendingType = "ResourceDeletion"
	ResourceCreatePending      WbPendingType = "ResourceCreation"
)

const (
	NotInstalledType = "NotInstalled"
	InstalledType    = "Installed"
)
