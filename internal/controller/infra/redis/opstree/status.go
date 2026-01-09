package opstree

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	corev1 "k8s.io/api/core/v1"
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
	if !common.ContainsType(conditions, RedisConnectionInfoType) {
		conditions = append(conditions, metav1.Condition{
			Type:   RedisConnectionInfoType,
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

	impliedStates = inferStateFromCondition(ctx, RedisStandaloneCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, RedisSentinelCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, RedisReplicationCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, RedisConnectionInfoType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, RedisReportedReadyType, impliedStates, conditions)

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
	if impliedStates[RedisConnectionInfoType] == common.UnavailableState &&
		impliedStates[RedisReportedReadyType] == common.HealthyState {
		events = append(events, corev1.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  "RedisConnectionInfoUnavailable",
			Message: fmt.Sprintf("Redis connection info is unavailable, but Redis is reported as ready."),
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
		case RedisStandaloneCustomResourceType:
			impliedStates[conditionType] = inferState_RedisStandaloneCustomResourceType(ctx, cond)
		case RedisSentinelCustomResourceType:
			impliedStates[conditionType] = inferState_RedisSentinelCustomResourceType(ctx, cond)
		case RedisReplicationCustomResourceType:
			impliedStates[conditionType] = inferState_RedisReplicationCustomResourceType(ctx, cond)
		case RedisConnectionInfoType:
			impliedStates[conditionType] = inferState_RedisConnectionInfoType(ctx, cond)
		case RedisReportedReadyType:
			impliedStates[conditionType] = inferState_RedisReportedReadyType(ctx, cond)
		default:
			impliedStates[conditionType] = common.UnknownState
		}
	}
	return impliedStates
}

func inferState_RedisStandaloneCustomResourceType(ctx context.Context, condition metav1.Condition) string {
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
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "RedisStandaloneCustomResource", result))
	return result
}

func inferState_RedisSentinelCustomResourceType(ctx context.Context, condition metav1.Condition) string {
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
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "RedisSentinelCustomResource", result))
	return result
}

func inferState_RedisReplicationCustomResourceType(ctx context.Context, condition metav1.Condition) string {
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
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "RedisReplicationCustomResource", result))
	return result
}

func inferState_RedisConnectionInfoType(ctx context.Context, condition metav1.Condition) string {
	log := ctrl.LoggerFrom(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		result = common.UnavailableState
	}
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "RedisConnectionInfo", result))
	return result
}

func inferState_RedisReportedReadyType(ctx context.Context, condition metav1.Condition) string {
	log := ctrl.LoggerFrom(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		if condition.Reason == "degraded" {
			result = common.DegradedState
		} else {
			result = common.UnavailableState
		}
	}
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "RedisReportedReady", result))
	return result
}
