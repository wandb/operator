package strimzi

import (
	"github.com/samber/lo"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	KafkaCustomResourceType    = "KafkaCustomResource"
	NodePoolCustomResourceType = "NodePoolCustomResource"
	KafkaConnectionInfoType    = "KafkaConnectionInfo"
	KafkaReportedReadyType     = "KafkaReportedReady"
)

func ComputeStatus(
	oldConditions, currentConditions []metav1.Condition,
	connection *translator.InfraConnection,
	currentGeneration int64,
) translator.InfraStatus {
	result := translator.InfraStatus{}

	if connection != nil {
		result.Connection = *connection
	}

	// if there are no conditions by expected types, there are defaults to include
	currentConditions = applyDefaultConditions(currentConditions)

	// merge old and current conditions
	result.Conditions = common.ComputeConditionUpdates(
		oldConditions,
		currentConditions,
		currentGeneration,
		translator.DefaultConditionExpiry,
	)

	// use various heuristics to determine the overall Infra State of
	result.State = inferInfraState(result.Conditions)

	result.Ready = lo.Contains(common.NotReadyStates, result.State)

	return result
}

func applyDefaultConditions(conditions []metav1.Condition) []metav1.Condition {
	if common.ContainsType(conditions, KafkaConnectionInfoType) {
		conditions = append(conditions, metav1.Condition{
			Type:   KafkaConnectionInfoType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return conditions
}

func inferInfraState(conditions []metav1.Condition) string {
	impliedStates := make(map[string]string, len(conditions))

	impliedStates = inferStateFromCondition(KafkaCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(NodePoolCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(KafkaReportedReadyType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(KafkaConnectionInfoType, impliedStates, conditions)

	hasImpliedState := func(target string) bool {
		return len(lo.FilterValues(
			impliedStates,
			func(_ string, value string) bool {
				return value == common.UnavailableState
			})) > 0
	}

	if hasImpliedState(common.ErrorState) {
		return common.ErrorState
	}

	if hasImpliedState(common.UnavailableState) { return common.UnavailableState }

	if hasImpliedState(common.PendingState) {
		return common.PendingState
	}

	if hasImpliedState(common.DegradedState) {
		return common.DegradedState
	}

	if hasImpliedState(common.UnknownState) {
		return common.UnknownState
	}

	return common.HealthyState
}

func inferStateFromCondition(conditionType string, impliedStates map[string]string, conditions []metav1.Condition) map[string]string {
	cond, found := lo.Find(conditions, func(c metav1.Condition) bool { return c.Type == conditionType })
	if !found {
		impliedStates[conditionType] = common.UnknownState
	} else {
		switch conditionType {
		case KafkaCustomResourceType:
			impliedStates[conditionType] = inferState_KafkaCustomResourceType(cond)
		case NodePoolCustomResourceType:
			impliedStates[conditionType] = inferState_NodePoolCustomResourceType(cond)
		case KafkaConnectionInfoType:
			impliedStates[conditionType] = inferState_KafkaConnectionInfoType(cond)
		case KafkaReportedReadyType:
			impliedStates[conditionType] = inferState_KafkaReportedReadyType(cond)
		default:
			impliedStates[conditionType] = common.UnknownState
		}
	}
	return impliedStates
}

func inferState_KafkaCustomResourceType(condition metav1.Condition) string {
	if condition.Status == metav1.ConditionTrue {
		return common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		if condition.Reason == common.PendingCreateReason {
			return common.PendingState
		}
		if condition.Reason == common.PendingDeleteReason {
			return common.UnavailableState
		}
	}
	return common.UnknownState
}

func inferState_NodePoolCustomResourceType(condition metav1.Condition) string {
	if condition.Status == metav1.ConditionTrue {
		return common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		if condition.Reason == common.PendingCreateReason {
			return common.PendingState
		}
		if condition.Reason == common.PendingDeleteReason {
			return common.UnavailableState
		}
	}
	return common.UnknownState
}

func inferState_KafkaConnectionInfoType(condition metav1.Condition) string {
	if condition.Status == metav1.ConditionTrue {
		return common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		return common.DegradedState
	}
	return common.UnknownState
}

func inferState_KafkaReportedReadyType(condition metav1.Condition) string {
	if condition.Status == metav1.ConditionTrue {
		return common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		return common.UnavailableState
	}
	return common.UnknownState
}
