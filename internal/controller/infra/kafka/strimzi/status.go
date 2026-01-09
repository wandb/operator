package strimzi

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
	KafkaCustomResourceType    = "KafkaCustomResource"
	NodePoolCustomResourceType = "NodePoolCustomResource"
	KafkaConnectionInfoType    = "KafkaConnectionInfo"
	KafkaReportedReadyType     = "KafkaReportedReady"
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
	state, events := inferInfraState(ctx, result.Conditions)
	result.State = state

	result.Ready = lo.Contains(common.NotReadyStates, result.State)

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
	if common.ContainsType(conditions, KafkaConnectionInfoType) {
		conditions = append(conditions, metav1.Condition{
			Type:   KafkaConnectionInfoType,
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

	impliedStates = inferStateFromCondition(ctx, KafkaCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, NodePoolCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, KafkaReportedReadyType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, KafkaConnectionInfoType, impliedStates, conditions)

	hasImpliedState := func(target string) bool {
		return len(lo.FilterValues(
			impliedStates,
			func(_ string, value string) bool {
				return value == target
			})) > 0
	}

	if impliedStates[KafkaConnectionInfoType] == common.DegradedState &&
		impliedStates[KafkaReportedReadyType] == common.HealthyState {
		events = append(events, corev1.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  "KafkaConnectionInfoUnavailable",
			Message: fmt.Sprintf("Kafka connection info is unavailable, but Kafka is reported as ready."),
		})
	}

	if hasImpliedState(common.ErrorState) {
		return common.ErrorState, events
	}

	if hasImpliedState(common.UnavailableState) {
		return common.UnavailableState, events
	}

	if hasImpliedState(common.PendingState) {
		return common.PendingState, events
	}

	if hasImpliedState(common.DegradedState) {
		return common.DegradedState, events
	}

	if hasImpliedState(common.HealthyState) {
		return common.HealthyState, events
	}

	return common.UnknownState, events
}

func inferStateFromCondition(ctx context.Context, conditionType string, impliedStates map[string]string, conditions []metav1.Condition) map[string]string {
	cond, found := lo.Find(conditions, func(c metav1.Condition) bool { return c.Type == conditionType })
	if !found {
		impliedStates[conditionType] = common.UnknownState
	} else {
		switch conditionType {
		case KafkaCustomResourceType:
			impliedStates[conditionType] = inferState_KafkaCustomResourceType(ctx, cond)
		case NodePoolCustomResourceType:
			impliedStates[conditionType] = inferState_NodePoolCustomResourceType(ctx, cond)
		case KafkaConnectionInfoType:
			impliedStates[conditionType] = inferState_KafkaConnectionInfoType(ctx, cond)
		case KafkaReportedReadyType:
			impliedStates[conditionType] = inferState_KafkaReportedReadyType(ctx, cond)
		default:
			impliedStates[conditionType] = common.UnknownState
		}
	}
	return impliedStates
}

func inferState_KafkaCustomResourceType(ctx context.Context, condition metav1.Condition) string {
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
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "KafkaCustomResource", result))
	return result
}

func inferState_NodePoolCustomResourceType(ctx context.Context, condition metav1.Condition) string {
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
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "NodePoolCustomResource", result))
	return result
}

func inferState_KafkaConnectionInfoType(ctx context.Context, condition metav1.Condition) string {
	log := ctrl.LoggerFrom(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		result = common.DegradedState
	}
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "KafkaConnectionInfo", result))
	return result
}

func inferState_KafkaReportedReadyType(ctx context.Context, condition metav1.Condition) string {
	log := ctrl.LoggerFrom(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		result = common.UnavailableState
	}
	log.Info(fmt.Sprintf("For condition '%s', infer state '%s'", "KafkaReportedReady", result))
	return result
}
