package reconciler

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	externalch "github.com/wandb/operator/internal/controller/infra/external/clickhouse"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity/keeper"
	"github.com/wandb/operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func clickHouseWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	objStoreConn *apiv2.ObjectStoreConnection,
) []metav1.Condition {
	if wandb.Spec.ClickHouse.ManagedClickHouse != nil {
		return managedClickHouseWriteState(ctx, client, wandb, objStoreConn)
	}
	if wandb.Spec.ClickHouse.ExternalClickHouse != nil {
		return externalClickHouseWriteState(ctx, client, wandb)
	}
	return nil
}

func clickHouseReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.ClickHouseConnection) {
	if wandb.Spec.ClickHouse.ManagedClickHouse != nil {
		return managedClickHouseReadState(ctx, client, wandb, newConditions)
	}
	if wandb.Spec.ClickHouse.ExternalClickHouse != nil {
		return externalClickHouseReadState(ctx, client, wandb, newConditions)
	}
	return newConditions, nil
}

func clickHouseInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.ClickHouseConnection,
) (ctrl.Result, error) {
	if wandb.Spec.ClickHouse.ManagedClickHouse != nil {
		return managedClickHouseInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
	}
	if wandb.Spec.ClickHouse.ExternalClickHouse != nil {
		return externalClickHouseInferStatus(ctx, client, wandb, newConditions, newInfraConn)
	}
	return ctrl.Result{}, nil
}

func clickHousePurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	if spec := wandb.Spec.ClickHouse.ManagedClickHouse; spec != nil {
		specNamespacedName := managedClickHouseSpecNamespacedName(spec)
		onDeleteRule := altinity.ToClickHouseOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return altinity.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if wandb.Spec.ClickHouse.ExternalClickHouse != nil {
		return externalch.DeleteConnectionSecret(ctx, client, wandb)
	}
	return nil
}

func clickHouseDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	if spec == nil {
		return nil
	}
	specNamespacedName := managedClickHouseSpecNamespacedName(spec)
	return altinity.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedClickHouseWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	objStoreConn *apiv2.ObjectStoreConnection,
) []metav1.Condition {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse

	// ClickHouse uses ReplicatedMergeTree, which needs a ClickHouse Keeper
	// ensemble for replication coordination. Provision Keeper before the
	// ClickHouse installation; surface only hard API/translation errors so a
	// Keeper that is still coming up does not block creating the CHI.
	if conds := managedKeeperWriteState(ctx, client, wandb); conds != nil {
		return conds
	}

	var specNamespacedName = managedClickHouseSpecNamespacedName(spec)
	log := ctrl.LoggerFrom(ctx)

	// Managed ClickHouse stores table data in the configured object store. Resolve
	// the bucket connection (managed or external); if it is not ready yet, wait and
	// requeue rather than creating a CHI without working storage.
	objStorage, err := altinity.ResolveObjectStorage(ctx, client, wandb, objStoreConn)
	if err != nil {
		log.Error(err, "object storage not ready for ClickHouse")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.PendingCreateReason,
			},
			{
				Type:   altinity.ClickHouseCustomResourceType,
				Status: metav1.ConditionFalse,
				Reason: common.PendingCreateReason,
			},
		}
	}

	desired, err := altinity.ToClickHouseVendorSpec(ctx, wandb, client.Scheme(), objStorage)
	if err != nil {
		log.Error(err, "failed to translate ClickHouse spec to vendor spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	if conditions := altinity.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID()); conditions != nil {
		return conditions
	}

	results := altinity.WriteState(ctx, client, specNamespacedName, desired)
	return results
}

// managedKeeperWriteState provisions the ClickHouse Keeper ensemble that backs
// ReplicatedMergeTree replication. It returns non-nil conditions only when
// ClickHouse reconciliation should be short-circuited (a hard API or
// translation error); otherwise it returns nil and the CHI write proceeds.
func managedKeeperWriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	desired, err := keeper.ToKeeperVendorSpec(ctx, wandb, c.Scheme())
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to translate Keeper spec to vendor spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	conds := keeper.WriteState(ctx, c, keeper.SpecNamespacedName(spec), desired)
	for _, cond := range conds {
		if cond.Type == common.ReconciledType && cond.Status == metav1.ConditionFalse {
			return conds
		}
	}
	return nil
}

func managedClickHouseReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.ClickHouseConnection) {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse

	specNamespacedName := managedClickHouseSpecNamespacedName(spec)
	onDeleteRule := altinity.ToClickHouseOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	readConditions, newInfraConn := altinity.ReadState(ctx, client, specNamespacedName, wandb, onDeleteRule)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func managedClickHouseInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.ClickHouseConnection,
) (ctrl.Result, error) {
	enabled := true
	oldConditions := wandb.Status.ClickHouseStatus.Conditions
	oldInfraConn := wandb.Status.ClickHouseStatus.Connection

	updatedStatus, events, ctrlResult := altinity.ComputeStatus(
		ctx,
		enabled,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	for _, e := range events {
		recorder.Event(wandb, e.Type, e.Reason, e.Message)
	}
	wandb.Status.ClickHouseStatus = updatedStatus
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalClickHouseWriteState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) []metav1.Condition {
	return externalch.WriteState(ctx, c, wandb)
}

func externalClickHouseReadState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition) ([]metav1.Condition, *apiv2.ClickHouseConnection) {
	return externalch.ReadState(ctx, c, wandb, newConditions)
}

func externalClickHouseInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition, newInfraConn *apiv2.ClickHouseConnection) (ctrl.Result, error) {
	oldInfraConn := wandb.Status.ClickHouseStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(wandb.Status.ClickHouseStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.ClickHouseStatus = apiv2.ClickHouseInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, c.Status().Update(ctx, wandb)
}

// helpers

func managedClickHouseSpecNamespacedName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
