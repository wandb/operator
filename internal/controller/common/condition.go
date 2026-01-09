package common

import (
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PendingDeleteReason   = "PendingDelete"
	PendingCreateReason   = "PendingCreate"
	ResourceExistsReason  = "ResourceExists"
	NoResourceReason      = "NoResource"
	ApiErrorReason        = "ApiError"
	ControllerErrorReason = "ControllerError"
	ResourceErrorReason   = "ResourceError"
	UnknownReason         = "Unknown"
)

const (
	ReconciledType = "Reconciled"
)

func ContainsType(conditions []metav1.Condition, typeName string) bool {
	return lo.ContainsBy(conditions, func(c metav1.Condition) bool {
		return c.Type == typeName
	})
}

// ComputeConditionUpdates is a complex flow that:
//  1. filters out duplicate(older) conditions of the same Type
//  2. prepares the currentConditions for reconciliation with currentGeneration and LastTransitionTime
//  3. supply conditions whereby:
//     a. keep the old condition if there's no new condition of that type
//     b. use the new condition if there's no old condition of that type
//     c. update the condition of the new condition has changed data
//  4. filter out expired conditions
func ComputeConditionUpdates(
	oldConditions, currentConditions []metav1.Condition,
	currentGeneration int64,
	expiry time.Duration,
) []metav1.Condition {
	// filter out any conditions with duplicated types by taking the last(latest) of each Type for old & current
	oldConditions = takeLatestByType(oldConditions)
	currentConditions = takeLatestByType(currentConditions)

	// give each current a lastReconciled of Now() and the current reconcile Generation
	currentConditions = setToCurrentReconciliation(currentConditions, currentGeneration)

	// Unique set of Condition types: from old or current
	typeSet := lo.Uniq(lo.Map(
		append(oldConditions, currentConditions...),
		func(c metav1.Condition, _ int) string { return c.Type }),
	)

	// Find old & current Condition for each Type
	type matchedTypes struct {
		old     *metav1.Condition
		current *metav1.Condition
	}
	oldPointers := lo.Map(oldConditions, func(c metav1.Condition, _ int) *metav1.Condition { return &c })
	currentPointers := lo.Map(currentConditions, func(c metav1.Condition, _ int) *metav1.Condition { return &c })
	possibleUpdates := make([]matchedTypes, 0, len(typeSet))
	for _, typeName := range typeSet {
		old := lo.FindOrElse(oldPointers, nil, func(c *metav1.Condition) bool { return c.Type == typeName })
		current := lo.FindOrElse(currentPointers, nil, func(c *metav1.Condition) bool { return c.Type == typeName })
		possibleUpdates = append(possibleUpdates, matchedTypes{old, current})
	}

	// for each old/current of a given Type:
	// - use old, if there is no current
	// - use current, if there is no old
	// - prefer the current to the old *only* if it has updates
	//   (so that the lastTransitionTime and ObservedGeneration are only updated when there are changes)
	result := make([]metav1.Condition, 0)
	for _, upd := range possibleUpdates {
		if upd.old == nil && upd.current == nil {
			break // this should never happen, but for completeness...
		}
		if upd.old == nil {
			result = append(result, *upd.current)
		} else if upd.current == nil {
			result = append(result, *upd.old)
		} else {
			if hasConditionChanged(*upd.old, *upd.current) {
				result = append(result, *upd.current)
			} else {
				result = append(result, *upd.old)
			}
		}
	}

	// for conditions that haven't been updated in a long time, perhaps due to some infra update,
	// clean them out so that they don't stay around forever
	result = removeExpiredConditions(result, expiry)

	return result
}

// removeExpiredConditions will exclude Conditions that haven't been updated since expiry time ago
func removeExpiredConditions(result []metav1.Condition, expiry time.Duration) []metav1.Condition {
	return lo.Filter(result, func(c metav1.Condition, _ int) bool {
		return c.LastTransitionTime.After(time.Now().Add(-expiry))
	})
}

// setToCurrentReconciliation should set each condition to the supplied currentGeneration and
// the LastTransitionTime to Now()
// This indicates that the condition has been updated during this reconcile loop.
func setToCurrentReconciliation(conditions []metav1.Condition, currentGeneration int64) []metav1.Condition {
	return lo.Map(conditions, func(c metav1.Condition, _ int) metav1.Condition {
		c.LastTransitionTime = metav1.Now()
		c.ObservedGeneration = currentGeneration
		return c
	})
}

// hasConditionChanged looks for relevant field changes, excluding LastTransitionTime.
// It assumes old and current are of the same Type.
func hasConditionChanged(old metav1.Condition, current metav1.Condition) bool {
	return old.Status != current.Status || old.Message != current.Message || old.Reason != current.Reason
}

// takeLatestByType will filter out conditions so that only the last for each Type
// combination will be returned. Sometimes, more information about a Type may become
// available during a single pass of a reconciliation loop.
func takeLatestByType(conditions []metav1.Condition) []metav1.Condition {
	registry := make(map[string]metav1.Condition)
	for _, c := range conditions {
		registry[c.Type] = c
	}
	return lo.Values(registry)
}
