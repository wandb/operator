package translator

import (
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ConditionExpiry = 2 * time.Hour

// CommonStatus contains the core status fields shared across all resource types
type CommonStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
}

// CommonInfraStatus is a representation of Status that must support round-trip translation
// between any version of WB[Infra]Status and itself -- it _may_ be extended to add more
// fields for some infra
type CommonInfraStatus struct {
	CommonStatus
	Connection InfraConnection
}

///////////////////////////////////////
// Status Builders

func CommonInfraStatusBuilder(
	oldConditions []metav1.Condition,
	protoConditions []*ProtoCondition,
	generation int64,
	connection InfraConnection,
) CommonInfraStatus {
	baseStatus := CommonStatusBuilder(oldConditions, protoConditions, generation)
	return CommonInfraStatus{
		CommonStatus: baseStatus,
		Connection:   connection,
	}
}

func CommonStatusBuilder(
	oldConditions []metav1.Condition,
	protoConditions []*ProtoCondition,
	generation int64,
) CommonStatus {

	ready, state := componentReadyState(protoConditions)

	updatedConditions := updateConditions(oldConditions, protoConditions, generation)

	return CommonStatus{
		Ready:      ready,
		State:      state,
		Conditions: updatedConditions,
	}
}

// componentReadyState considers only *current* (not previous) conditions to determine
// the overall readiness and state of the component
func componentReadyState(protoConditions []*ProtoCondition) (bool, string) {

	// if the condition status is true, the impliedState is valid; otherwise, assume it is
	// unknown for that condition
	impliedStates := lo.Map(protoConditions, func(proto *ProtoCondition, _ int) WbState {
		if proto.Condition.Status == metav1.ConditionTrue {
			return proto.ImpliedState
		}
		return UnknownState
	})

	// if there zero impliedStates or all are UnknownState, the component is UnknownState and not ready
	if len(impliedStates) == 0 || lo.Every(impliedStates, []WbState{UnknownState}) {
		return false, string(UnknownState)
	}

	// if any are ErrorState, the component is an ErrorState and not ready
	if lo.Contains(impliedStates, ErrorState) {
		return false, string(ErrorState)
	}

	// otherwise, if any are PendingState, the component is PendingState and not ready
	if lo.Contains(impliedStates, PendingState) {
		return false, string(PendingState)
	}

	// otherwise, if any are NotInstalledState, the component is NotInstalledState and not ready
	if lo.Contains(impliedStates, NotInstalledState) {
		return false, string(NotInstalledState)
	}

	// otherwise, if any are DegradedState, the component is DegradedState and is *technically ready*
	if lo.Contains(impliedStates, DegradedState) {
		return true, string(DegradedState)
	}

	// otherwise, if any are HealthyState, the component is HealthyState and *ready*
	if lo.Contains(impliedStates, HealthyState) {
		return true, string(HealthyState)
	}

	// otherwise, fall back to UnknownState and not ready
	return false, string(UnknownState)
}

func updateConditions(
	oldConditions []metav1.Condition,
	protoConditions []*ProtoCondition,
	generation int64,
) []metav1.Condition {
	newConditions := lo.Map(protoConditions, func(proto *ProtoCondition, _ int) metav1.Condition {
		result := proto.Condition
		result.ObservedGeneration = generation
		return result
	})

	result := make([]metav1.Condition, 0)

	// find new conditions *without* a matching Type-Reason get added first
	for _, current := range newConditions {
		_, found := findMatchingTypeReason(current, oldConditions)
		if !found {
			result = append(result, current)
		}
	}

	// place old conditions (or updates for old conditions) into results
	for _, old := range oldConditions {
		match, found := findMatchingTypeReason(old, newConditions)

		// if no match found or if match requires no update, use the old condition
		if !found || !requiresUpdate(old, match) {
			result = append(result, old)
			break
		}
		result = append(result, match)
	}

	return lo.Filter(result, func(elem metav1.Condition, _ int) bool {
		return elem.LastTransitionTime.Time.After(time.Now().Add(-ConditionExpiry))
	})
}

func findMatchingTypeReason(c metav1.Condition, arr []metav1.Condition) (metav1.Condition, bool) {
	return lo.Find(arr, func(elem metav1.Condition) bool {
		return c.Type == elem.Type && c.Reason == elem.Reason
	})
}

func requiresUpdate(old metav1.Condition, current metav1.Condition) bool {
	return old.Status != current.Status || old.Message != current.Message
}
