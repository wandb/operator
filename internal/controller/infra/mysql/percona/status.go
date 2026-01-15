package percona

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	MySQLCustomResourceType = "MySQLCustomResource"
	MySQLConnectionInfoType = "MySQLConnectionInfo"
	MySQLReportedReadyType  = "MySQLReportedReady"
)

func ComputeStatus(
	ctx context.Context,
	enabled bool,
	oldConditions, currentConditions []metav1.Condition,
	connection *translator.InfraConnection,
	currentGeneration int64,
) (translator.InfraStatus, []corev1.Event, ctrl.Result) {
	ctx, _ = logx.WithSlog(ctx, logx.Mysql)
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

	state, events := inferInfraState(ctx, enabled, result.Conditions)
	result.State = state

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

	return result, events, ctrl.Result{RequeueAfter: requeueAfter}
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

func inferInfraState(
	ctx context.Context,
	enabled bool,
	conditions []metav1.Condition,
) (string, []corev1.Event) {
	if !enabled {
		return common.UnavailableState, nil
	}
	var events []corev1.Event
	impliedStates := make(map[string]string, len(conditions))

	impliedStates = inferStateFromCondition(ctx, MySQLCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, MySQLConnectionInfoType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, MySQLReportedReadyType, impliedStates, conditions)

	hasImpliedState := func(target string) bool {
		return len(lo.FilterValues(
			impliedStates,
			func(_ string, value string) bool {
				return value == target
			})) > 0
	}

	summaryState := common.UnknownState

	// if the service is reporting as healthy but the connection info is unavailable
	// log the missing connection as an event and mark the infraStatus as 'error'
	if impliedStates[MySQLConnectionInfoType] == common.UnavailableState &&
		impliedStates[MySQLReportedReadyType] == common.HealthyState {
		events = append(events, corev1.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  "MySQLConnectionInfoUnavailable",
			Message: fmt.Sprintf("MySQL connection info is unavailable, but MySQL is reported as ready."),
		})
		summaryState = common.ErrorState
	}

	// if there is a specific state identified, use that
	if summaryState != common.UnknownState {
		return summaryState, events
	}

	// otherwise, return the most significant state mapped for any condition
	stateSignificanceOrder := []string{
		common.ErrorState,
		common.UnavailableState,
		common.PendingState,
		common.DegradedState,
		common.HealthyState,
	}
	for _, s := range stateSignificanceOrder {
		if hasImpliedState(s) {
			return s, events
		}
	}

	return common.UnknownState, events
}

func inferStateFromCondition(ctx context.Context, conditionType string, impliedStates map[string]string, conditions []metav1.Condition) map[string]string {
	cond, found := lo.Find(conditions, func(c metav1.Condition) bool { return c.Type == conditionType })
	if !found {
		impliedStates[conditionType] = common.UnknownState
	} else {
		switch conditionType {
		case MySQLCustomResourceType:
			impliedStates[conditionType] = inferState_MySQLCustomResourceType(ctx, cond)
		case MySQLConnectionInfoType:
			impliedStates[conditionType] = inferState_MySQLConnectionInfoType(ctx, cond)
		case MySQLReportedReadyType:
			impliedStates[conditionType] = inferState_MySQLReportedReadyType(ctx, cond)
		default:
			impliedStates[conditionType] = common.UnknownState
		}
	}
	return impliedStates
}

func inferState_MySQLCustomResourceType(ctx context.Context, condition metav1.Condition) string {
	log := logx.GetSlog(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		if condition.Reason == common.PendingCreateReason {
			result = common.PendingState
		}
		if condition.Reason == common.PendingDeleteReason {
			result = common.UnavailableState
		}
	}
	log.Debug(
		"implied state", "state", result, "condition", condition.Type,
		"reason", condition.Reason, "status", condition.Status,
	)
	return result
}

func inferState_MySQLConnectionInfoType(ctx context.Context, condition metav1.Condition) string {
	log := logx.GetSlog(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		result = common.UnavailableState
	}
	log.Debug(
		"implied state", "state", result, "condition", condition.Type,
		"reason", condition.Reason, "status", condition.Status,
	)
	return result
}

func inferState_MySQLReportedReadyType(ctx context.Context, condition metav1.Condition) string {
	log := logx.GetSlog(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		switch condition.Reason {
		case "initializing":
			result = common.PendingState
		case "paused", "stopping":
			result = common.UnavailableState
		case "error":
			result = common.ErrorState
		default:
			result = common.UnavailableState
		}
	}
	log.Debug(
		"implied state", "state", result, "condition", condition.Type,
		"reason", condition.Reason, "status", condition.Status,
	)
	return result
}
