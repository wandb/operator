package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
)

func computeOverallState(conditions []apiv2.WBStatusCondition, ready bool) apiv2.WBStateType {
	if len(conditions) == 0 {
		if ready {
			return apiv2.WBStateReady
		}
		return apiv2.WBStateUnknown
	}

	worst := conditions[0].State
	for _, condition := range conditions[1:] {
		if condition.State.IsWorseThan(worst) {
			worst = condition.State
		}
	}
	return worst
}
