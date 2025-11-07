package redis

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/wandb_v2"
	"github.com/wandb/operator/internal/controller/wandb_v2/common"
)

type opstreeSnapshot struct {
	desired    opstree
	actual     opstree
	wandb      *apiv2.WeightsAndBiases
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler
}

type OpstreeSnapshotHandler func(context.Context, opstreeSnapshot) common.CtrlState
