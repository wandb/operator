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
	status     *apiv2.WBRedisStatus
	reconciler *wandb_v2.WeightsAndBiasesV2Reconciler
}

func (o opstreeSnapshot) expectsStandalone() bool {
	return o.desired.redis != nil
}

func (o opstreeSnapshot) expectsReplication() bool {
	return o.desired.replication != nil
}

func (o opstreeSnapshot) expectsSentinel() bool {
	return o.desired.sentinel != nil
}

func (o opstreeSnapshot) hasStandalone() bool {
	return o.actual.redis != nil
}

func (o opstreeSnapshot) hasReplication() bool {
	return o.actual.replication != nil
}

func (o opstreeSnapshot) hasSentinel() bool {
	return o.actual.sentinel != nil
}

var allHandlers = []opstreeHandler{}

type opstreeHandler interface {
	reconcile() common.CtrlState
}

// CreateOpstreeRedisStandalone will reconcile only for non-Sentinel deployments
func CreateOpstreeRedisStandalone(snapshot opstreeSnapshot) *createOpstreeRedisImpl {
	return &createOpstreeRedisImpl{
		snapshot: snapshot,
	}
}

type createOpstreeRedisImpl struct {
	snapshot opstreeSnapshot
}

func (impl *createOpstreeRedisImpl) reconcile() common.CtrlState {
	expectsStandalone := impl.snapshot.desired.redis != nil
	hasStandalone := impl.snapshot.actual.redis != nil
	hasSentinel := impl.snapshot.actual.sentinel != nil
	hasReplication := impl.snapshot.actual.replication != nil

	// TODO: Implement reconciliation logic
	_ = expectsStandalone
	_ = hasStandalone
	_ = hasSentinel
	_ = hasReplication

	return common.CtrlContinue()
}
