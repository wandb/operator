package altinity

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
	ClickHouseCustomResourceType = "ClickHouseCustomResource"
	ClickHouseConnectionInfoType = "ClickHouseConnectionInfo"
	ClickHouseReportedReadyType  = "ClickHouseReportedReady"
)

func ComputeStatus(
	ctx context.Context,
	oldConditions, currentConditions []metav1.Condition,
	connection *translator.InfraConnection,
	currentGeneration int64,
) (translator.InfraStatus, []corev1.Event, ctrl.Result) {
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

	state, events := inferInfraState(ctx, result.Conditions)
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
	if !common.ContainsType(conditions, ClickHouseConnectionInfoType) {
		conditions = append(conditions, metav1.Condition{
			Type:   ClickHouseConnectionInfoType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	return conditions
}

func inferInfraState(
	ctx context.Context,
	conditions []metav1.Condition,
) (string, []corev1.Event) {
	var events []corev1.Event
	impliedStates := make(map[string]string, len(conditions))

	impliedStates = inferStateFromCondition(ctx, ClickHouseCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, ClickHouseConnectionInfoType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, ClickHouseReportedReadyType, impliedStates, conditions)

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
	if impliedStates[ClickHouseConnectionInfoType] == common.UnavailableState &&
		impliedStates[ClickHouseReportedReadyType] == common.HealthyState {
		events = append(events, corev1.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  "ClickHouseConnectionInfoUnavailable",
			Message: fmt.Sprintf("ClickHouse connection info is unavailable, but ClickHouse is reported as ready."),
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
		case ClickHouseCustomResourceType:
			impliedStates[conditionType] = inferState_ClickHouseCustomResourceType(ctx, cond)
		case ClickHouseConnectionInfoType:
			impliedStates[conditionType] = inferState_ClickHouseConnectionInfoType(ctx, cond)
		case ClickHouseReportedReadyType:
			impliedStates[conditionType] = inferState_ClickHouseReportedReadyType(ctx, cond)
		default:
			impliedStates[conditionType] = common.UnknownState
		}
	}
	return impliedStates
}

func inferState_ClickHouseCustomResourceType(ctx context.Context, condition metav1.Condition) string {
	log := ctrl.LoggerFrom(ctx)
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
	log.WithName(logx.ClickHouse).Info(
		fmt.Sprintf("For condition '%s', infer state '%s'", "ClickHouseCustomResource", result),
	)
	return result
}

func inferState_ClickHouseConnectionInfoType(ctx context.Context, condition metav1.Condition) string {
	log := ctrl.LoggerFrom(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		result = common.UnavailableState
	}
	log.WithName(logx.ClickHouse).Info(
		fmt.Sprintf("For condition '%s', infer state '%s'", "ClickHouseConnectionInfo", result),
	)
	return result
}

func inferState_ClickHouseReportedReadyType(ctx context.Context, condition metav1.Condition) string {
	log := ctrl.LoggerFrom(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		if condition.Reason == common.ResourceErrorReason {
			result = common.ErrorState
		} else {
			result = common.DegradedState
		}
	}
	log.WithName(logx.ClickHouse).Info(
		fmt.Sprintf("For condition '%s', infer state '%s'", "ClickHouseReportedReady", result),
	)
	return result
}
