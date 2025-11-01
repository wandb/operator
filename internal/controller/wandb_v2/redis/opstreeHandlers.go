package redis

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/wandb_v2"
	"github.com/wandb/operator/internal/controller/wandb_v2/common"
)

type opstreeSnapshot struct {
	ctx        context.Context
	desired    opstree
	actual     opstree
	status     *apiv2.WBDatabaseStatus
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler
}

var allHandlers = []opstreeHandler{}

type opstreeHandler interface {
	reconcile(snapshot opstreeSnapshot) common.CtrlState
}
