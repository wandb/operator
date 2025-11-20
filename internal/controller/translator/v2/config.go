package v2

import (
	apiv2 "github.com/wandb/operator/api/v2"
)

func computeOverallState(details []apiv2.WBStatusCondition, ready bool) apiv2.WBStateType {
	if len(details) == 0 {
		if ready {
			return apiv2.WBStateReady
		}
		return apiv2.WBStateUnknown
	}

	worst := details[0].State
	for _, detail := range details[1:] {
		if detail.State.IsWorseThan(worst) {
			worst = detail.State
		}
	}
	return worst
}
