package common

const (
	ErrorState       string = "Error"
	PendingState     string = "Pending"
	HealthyState     string = "Healthy"
	DegradedState    string = "Degraded"
	UnknownState     string = "Unknown"
	UnavailableState string = "Unavailable"
)

var NotReadyStates = []string{
	ErrorState, PendingState, UnavailableState,
}
