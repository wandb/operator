package seaweedfs

import (
	"context"
	"time"

	"github.com/samber/lo"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	SeaweedCustomResourceType = "SeaweedCustomResource"
	SeaweedConnectionInfoType = "SeaweedConnectionInfo"
	SeaweedReportedReadyType  = "SeaweedReportedReady"
	SeaweedWritableType       = "SeaweedWritable"
	SeaweedS3ReachableType    = "SeaweedS3Reachable"
)

func ComputeStatus(
	ctx context.Context,
	enabled bool,
	oldConditions, currentConditions []metav1.Condition,
	connection *apiv2.ObjectStoreConnection,
	currentGeneration int64,
) (apiv2.ObjectStoreInfraStatus, []corev1.Event, ctrl.Result) {
	ctx, _ = logx.WithSlog(ctx, logx.ObjectStore)
	result := apiv2.ObjectStoreInfraStatus{}

	if connection != nil {
		result.Connection = *connection
	}

	currentConditions = applyDefaultConditions(currentConditions)

	result.Conditions = common.ComputeConditionUpdates(
		oldConditions,
		currentConditions,
		currentGeneration,
		common.DefaultConditionExpiry,
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
	if !common.ContainsType(conditions, SeaweedConnectionInfoType) {
		conditions = append(conditions, metav1.Condition{
			Type:   SeaweedConnectionInfoType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}
	if !common.ContainsType(conditions, SeaweedWritableType) {
		conditions = append(conditions, metav1.Condition{
			Type:   SeaweedWritableType,
			Status: metav1.ConditionUnknown,
			Reason: common.NoResourceReason,
		})
	}
	if !common.ContainsType(conditions, SeaweedS3ReachableType) {
		conditions = append(conditions, metav1.Condition{
			Type:   SeaweedS3ReachableType,
			Status: metav1.ConditionUnknown,
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

	impliedStates = inferStateFromCondition(ctx, SeaweedCustomResourceType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, SeaweedConnectionInfoType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, SeaweedReportedReadyType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, SeaweedWritableType, impliedStates, conditions)
	impliedStates = inferStateFromCondition(ctx, SeaweedS3ReachableType, impliedStates, conditions)

	hasImpliedState := func(target string) bool {
		return len(lo.FilterValues(
			impliedStates,
			func(_ string, value string) bool {
				return value == target
			})) > 0
	}

	summaryState := common.UnknownState

	if impliedStates[SeaweedConnectionInfoType] == common.UnavailableState &&
		impliedStates[SeaweedReportedReadyType] == common.HealthyState {
		events = append(events, corev1.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  "SeaweedConnectionInfoUnavailable",
			Message: "SeaweedFS connection info is unavailable, but SeaweedFS is reported as ready.",
		})
		summaryState = common.ErrorState
	}

	if summaryState != common.UnknownState {
		return summaryState, events
	}

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
		case SeaweedCustomResourceType:
			impliedStates[conditionType] = inferState_SeaweedCustomResourceType(ctx, cond)
		case SeaweedConnectionInfoType:
			impliedStates[conditionType] = inferState_SeaweedConnectionInfoType(ctx, cond)
		case SeaweedReportedReadyType:
			impliedStates[conditionType] = inferState_SeaweedReportedReadyType(ctx, cond)
		case SeaweedWritableType, SeaweedS3ReachableType:
			impliedStates[conditionType] = inferState_SeaweedWritableType(ctx, cond)
		default:
			impliedStates[conditionType] = common.UnknownState
		}
	}
	return impliedStates
}

func inferState_SeaweedWritableType(ctx context.Context, condition metav1.Condition) string {
	log := logx.GetSlog(ctx)
	result := common.PendingState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		result = common.ErrorState
	}
	log.Debug(
		"implied state", "state", result, "condition", condition.Type,
		"reason", condition.Reason, "status", condition.Status,
	)
	return result
}

func inferState_SeaweedCustomResourceType(ctx context.Context, condition metav1.Condition) string {
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

func inferState_SeaweedConnectionInfoType(ctx context.Context, condition metav1.Condition) string {
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

func inferState_SeaweedReportedReadyType(ctx context.Context, condition metav1.Condition) string {
	log := logx.GetSlog(ctx)
	result := common.UnknownState
	if condition.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if condition.Status == metav1.ConditionFalse {
		switch condition.Reason {
		case "yellow":
			result = common.DegradedState
		case "red":
			result = common.ErrorState
		default:
			result = common.DegradedState
		}
	}
	log.Debug(
		"implied state", "state", result, "condition", condition.Type,
		"reason", condition.Reason, "status", condition.Status,
	)
	return result
}
