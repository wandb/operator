package bufstream

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

func ComputeStatus(
	ctx context.Context,
	enabled bool,
	oldConditions, currentConditions []metav1.Condition,
	connection *apiv2.KafkaConnection,
	currentGeneration int64,
) (apiv2.KafkaInfraStatus, []corev1.Event, ctrl.Result) {
	ctx, _ = logx.WithSlog(ctx, logx.Kafka)
	result := apiv2.KafkaInfraStatus{}

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
	if !common.ContainsType(conditions, KafkaConnectionInfoType) {
		conditions = append(conditions, metav1.Condition{
			Type:   KafkaConnectionInfoType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}
	if !common.ContainsType(conditions, KafkaReportedReadyType) {
		conditions = append(conditions, metav1.Condition{
			Type:   KafkaReportedReadyType,
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

	for _, t := range []string{
		ObjectStoreReadyType,
		EtcdApplicationType,
		BufstreamApplicationType,
		KafkaReportedReadyType,
		KafkaConnectionInfoType,
	} {
		impliedStates[t] = inferStateFromCondition(ctx, t, conditions)
	}

	hasImpliedState := func(target string) bool {
		return len(lo.FilterValues(impliedStates, func(_ string, value string) bool {
			return value == target
		})) > 0
	}

	if impliedStates[KafkaConnectionInfoType] == common.UnavailableState &&
		impliedStates[KafkaReportedReadyType] == common.HealthyState {
		events = append(events, corev1.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  "KafkaConnectionInfoUnavailable",
			Message: "Kafka connection info is unavailable, but Kafka is reported as ready.",
		})
		return common.ErrorState, events
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

func inferStateFromCondition(ctx context.Context, conditionType string, conditions []metav1.Condition) string {
	log := logx.GetSlog(ctx)
	cond, found := lo.Find(conditions, func(c metav1.Condition) bool { return c.Type == conditionType })
	if !found {
		return common.UnknownState
	}

	result := common.UnknownState
	if cond.Status == metav1.ConditionTrue {
		result = common.HealthyState
	}
	if cond.Status == metav1.ConditionFalse {
		switch cond.Reason {
		case common.PendingCreateReason:
			result = common.PendingState
		case common.PendingDeleteReason:
			result = common.UnavailableState
		default:
			result = common.UnavailableState
		}
	}
	log.Debug(
		"implied state", "state", result, "condition", cond.Type,
		"reason", cond.Reason, "status", cond.Status,
	)
	return result
}
