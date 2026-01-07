package translator

import (
	"errors"

	"github.com/samber/lo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

// ProtoCondition has information that can be used to create a condition on a component's status
// and a perspective on what this condition may imply about the component's overall state.
// Further processing of all ProtoConditions will:
// * consider all ImpliedStates to determine what the component's State should be
// * find an existing Condition on the CR of the same Type to determine if ProtoCondition is an update
// * if it is a new or updated Condition, create or replace the matching Condition on the CR with this one.
type ProtoCondition struct {
	ImpliedState WbState
	Condition    v1.Condition
}

func NewProtoCondition(
	state WbState,
	condType string,
	reason string,
	message string,
	condStatus *bool,
) *ProtoCondition {
	return &ProtoCondition{
		ImpliedState: state,
		Condition: v1.Condition{
			Type:               condType,
			Status:             toConditionStatusValue(condStatus),
			Reason:             reason,
			Message:            message,
			LastTransitionTime: v1.Now(), // will be propagated only if Condition is new or and update on the CR
			ObservedGeneration: 0,        // fill this in from the reconciling object later, if needed
		},
	}
}

func toConditionStatusValue(conditionStatus *bool) v1.ConditionStatus {
	if conditionStatus == nil {
		return v1.ConditionUnknown
	}
	switch *conditionStatus {
	case true:
		return v1.ConditionTrue
	case false:
		return v1.ConditionFalse
	default:
		return v1.ConditionUnknown
	}
}

/////////////////////////////////////////////////
// Builders

type protoBuilder struct {
	proto *ProtoCondition
}

func ErrorProtoBuilder(condType WbErrorType, reason string) *protoBuilder {
	return &protoBuilder{
		proto: NewProtoCondition(
			ErrorState,
			string(condType),
			reason,
			"",
			ptr.Bool(true), // default to true, the most common case
		),
	}
}

func PendingProtoBuilder(condType WbPendingType, reason string) *protoBuilder {
	return &protoBuilder{
		proto: NewProtoCondition(
			PendingState,
			string(condType),
			reason,
			"",
			ptr.Bool(true), // default to true, the most common case
		),
	}
}

func DegradedProtoBuilder(condType WbDegradedType, reason string) *protoBuilder {
	return &protoBuilder{
		proto: NewProtoCondition(
			DegradedState,
			string(condType),
			reason,
			"",
			ptr.Bool(true), // default to true, the most common case
		),
	}
}

func NotInstalledProtoBuilder(reason string) *protoBuilder {
	return &protoBuilder{
		proto: NewProtoCondition(
			NotInstalledState,
			NotInstalledType,
			reason,
			"",
			ptr.Bool(true), // default to true, the most common case
		),
	}
}

func InstalledProtoBuilder(reason string) *protoBuilder {
	return &protoBuilder{
		proto: NewProtoCondition(
			HealthyState,
			InstalledType,
			reason,
			"",
			ptr.Bool(true), // default to true, the most common case
		),
	}
}

func (p *protoBuilder) Message(message string) *protoBuilder {
	p.proto.Condition.Message = message
	return p
}

func (p *protoBuilder) ConditionStatus(condStatus *bool) *protoBuilder {
	p.proto.Condition.Status = toConditionStatusValue(condStatus)
	return p
}

func (p *protoBuilder) Build() ProtoCondition {
	return *p.proto
}

/////////////////////////////////////////////////
// Utilities

func IsNotReady(candidates []ProtoCondition) bool {
	return lo.SomeBy(candidates, func(elem ProtoCondition) bool {
		return lo.Contains(NotReadyStates, elem.ImpliedState)
	})
}

func FirstError(candidates []ProtoCondition) error {
	for _, elem := range candidates {
		if elem.ImpliedState == ErrorState {
			return errors.New(elem.Condition.Message)
		}
	}
	return nil
}
