package opstree

import (
	"time"

	"github.com/samber/lo"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	RedisStandaloneCustomResourceType  = "RedisStandaloneCustomResource"
	RedisSentinelCustomResourceType    = "RedisSentinelCustomResource"
	RedisReplicationCustomResourceType = "RedisReplicationCustomResource"
	RedisConnectionInfoType            = "RedisConnectionInfo"
	RedisReportedReadyType             = "RedisReportedReady"
)

func ComputeStatus(
	oldConditions, currentConditions []metav1.Condition,
	connection *translator.InfraConnection,
	currentGeneration int64,
) (translator.InfraStatus, ctrl.Result) {
	result := translator.InfraStatus{}

	if connection != nil {
		result.Connection = *connection
	}

	currentConditions = applyDefaultConditions(currentConditions)

	result.Conditions = common.ComputeConditionUpdates(
		oldConditions,
		currentConditions,
		currentGeneration,
		translator.DefaultConditionExpiry,
	)

	result.State = inferInfraState(result.Conditions)

	result.Ready = !lo.Contains(common.NotReadyStates, result.State)

	requeueAfter := 3 * time.Minute
	switch result.State {
	case common.ErrorState:
		requeueAfter = 15 * time.Second
	case common.DegradedState:
		requeueAfter = 5 * time.Minute
	case common.PendingState:
		requeueAfter = 2 * time.Minute
	case common.HealthyState:
		requeueAfter = 10 * time.Minute
	}

	return result, ctrl.Result{RequeueAfter: requeueAfter}
}

func applyDefaultConditions(conditions []metav1.Condition) []metav1.Condition {
	if !common.ContainsType(conditions, RedisConnectionInfoType) {
		conditions = append(conditions, metav1.Condition{
			Type:   RedisConnectionInfoType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return conditions
}

func inferInfraState(conditions []metav1.Condition) string {
	impliedStates := make(map[string]string, len(conditions))

	impliedStates = inferStateFromCondition(RedisStandaloneCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(RedisSentinelCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(RedisReplicationCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(RedisConnectionInfoType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(RedisReportedReadyType, impliedStates, conditions)

	hasImpliedState := func(target string) bool {
		return len(lo.FilterValues(
			impliedStates,
			func(_ string, value string) bool {
				return value == target
			})) > 0
	}

	if hasImpliedState(common.ErrorState) {
		return common.ErrorState
	}

	if hasImpliedState(common.UnavailableState) {
		return common.UnavailableState
	}

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
		case RedisStandaloneCustomResourceType:
			impliedStates[conditionType] = inferState_RedisStandaloneCustomResourceType(cond)
		case RedisSentinelCustomResourceType:
			impliedStates[conditionType] = inferState_RedisSentinelCustomResourceType(cond)
		case RedisReplicationCustomResourceType:
			impliedStates[conditionType] = inferState_RedisReplicationCustomResourceType(cond)
		case RedisConnectionInfoType:
			impliedStates[conditionType] = inferState_RedisConnectionInfoType(cond)
		case RedisReportedReadyType:
			impliedStates[conditionType] = inferState_RedisReportedReadyType(cond)
		default:
			impliedStates[conditionType] = common.UnknownState
		}
	}
	return impliedStates
}

func inferState_RedisStandaloneCustomResourceType(condition metav1.Condition) string {
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

func inferState_RedisSentinelCustomResourceType(condition metav1.Condition) string {
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

func inferState_RedisReplicationCustomResourceType(condition metav1.Condition) string {
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

func inferState_RedisConnectionInfoType(condition metav1.Condition) string {
	if condition.Status == metav1.ConditionTrue {
		return common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		return common.DegradedState
	}
	return common.UnknownState
}

func inferState_RedisReportedReadyType(condition metav1.Condition) string {
	if condition.Status == metav1.ConditionTrue {
		return common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		if condition.Reason == "degraded" {
			return common.DegradedState
		}
		return common.UnavailableState
	}
	return common.UnknownState
}
