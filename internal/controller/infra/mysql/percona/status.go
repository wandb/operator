package percona

import (
	"time"

	"github.com/samber/lo"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	MySQLCustomResourceType = "MySQLCustomResource"
	MySQLConnectionInfoType = "MySQLConnectionInfo"
	MySQLReportedReadyType  = "MySQLReportedReady"
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
	if !common.ContainsType(conditions, MySQLConnectionInfoType) {
		conditions = append(conditions, metav1.Condition{
			Type:   MySQLConnectionInfoType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return conditions
}

func inferInfraState(conditions []metav1.Condition) string {
	impliedStates := make(map[string]string, len(conditions))

	impliedStates = inferStateFromCondition(MySQLCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(MySQLConnectionInfoType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(MySQLReportedReadyType, impliedStates, conditions)

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

	if hasImpliedState(common.HealthyState) {
		return common.HealthyState
	}

	return common.UnknownState
}

func inferStateFromCondition(conditionType string, impliedStates map[string]string, conditions []metav1.Condition) map[string]string {
	cond, found := lo.Find(conditions, func(c metav1.Condition) bool { return c.Type == conditionType })
	if !found {
		impliedStates[conditionType] = common.UnknownState
	} else {
		switch conditionType {
		case MySQLCustomResourceType:
			impliedStates[conditionType] = inferState_MySQLCustomResourceType(cond)
		case MySQLConnectionInfoType:
			impliedStates[conditionType] = inferState_MySQLConnectionInfoType(cond)
		case MySQLReportedReadyType:
			impliedStates[conditionType] = inferState_MySQLReportedReadyType(cond)
		default:
			impliedStates[conditionType] = common.UnknownState
		}
	}
	return impliedStates
}

func inferState_MySQLCustomResourceType(condition metav1.Condition) string {
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

func inferState_MySQLConnectionInfoType(condition metav1.Condition) string {
	if condition.Status == metav1.ConditionTrue {
		return common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		return common.DegradedState
	}
	return common.UnknownState
}

func inferState_MySQLReportedReadyType(condition metav1.Condition) string {
	if condition.Status == metav1.ConditionTrue {
		return common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		switch condition.Reason {
		case "initializing":
			return common.PendingState
		case "paused", "stopping":
			return common.UnavailableState
		case "error":
			return common.ErrorState
		default:
			return common.UnavailableState
		}
	}
	return common.UnknownState
}
