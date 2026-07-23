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
	"github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// clickHouseObjectStoreInstance is the object-store instance name managed
// ClickHouse prefers for its S3 disk; ResolveInstance falls back to the
// default instance when it is not provisioned.
const clickHouseObjectStoreInstance = "clickhouse"

func clickHouseWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	mfst manifest.Manifest,
) map[string][]metav1.Condition {
	out := map[string][]metav1.Condition{}
	for key, spec := range wandb.Spec.ClickHouse {
		switch {
		case spec.ManagedClickHouse != nil:
			out[key] = managedClickHouseWriteState(ctx, client, wandb, spec.ManagedClickHouse, mfst)
		case spec.ExternalClickHouse != nil:
			out[key] = externalch.WriteState(ctx, client, wandb, key, spec.ExternalClickHouse)
		}
	}
	return out
}

func clickHouseReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	conditions map[string][]metav1.Condition,
) (map[string][]metav1.Condition, map[string]*apiv2.ClickHouseConnection) {
	outConds := map[string][]metav1.Condition{}
	outConns := map[string]*apiv2.ClickHouseConnection{}
	for key, spec := range wandb.Spec.ClickHouse {
		switch {
		case spec.ManagedClickHouse != nil:
			outConds[key], outConns[key] = managedClickHouseReadState(ctx, client, wandb, spec.ManagedClickHouse, conditions[key])
		case spec.ExternalClickHouse != nil:
			outConds[key], outConns[key] = externalch.ReadState(ctx, client, wandb, key, conditions[key])
		default:
			outConds[key] = conditions[key]
		}
	}
	return outConds, outConns
}

func clickHouseInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	conditions map[string][]metav1.Condition,
	infraConns map[string]*apiv2.ClickHouseConnection,
) (ctrl.Result, error) {
	if wandb.Status.ClickHouseStatus == nil {
		wandb.Status.ClickHouseStatus = map[string]apiv2.ClickHouseInfraStatus{}
	}
	var results []ctrl.Result
	var firstErr error
	for key, spec := range wandb.Spec.ClickHouse {
		var res ctrl.Result
		var err error
		switch {
		case spec.ManagedClickHouse != nil:
			res, err = managedClickHouseInferStatus(ctx, client, recorder, wandb, key, conditions[key], infraConns[key])
		case spec.ExternalClickHouse != nil:
			res, err = externalClickHouseInferStatus(ctx, client, wandb, key, conditions[key], infraConns[key])
		}
		results = append(results, res)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return consolidateResults(results), firstErr
}

func runClickHouseRetentionFinalizer(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, spec apiv2.ClickHouseSpec) error {
	switch wandb.GetRetentionPolicy(clickHouseInstanceInfraSpec(spec)).OnDelete {
	case apiv2.PurgeOnDelete:
		return clickHousePurgeFinalizer(ctx, c, wandb, key, spec)
	case apiv2.DetachOnDelete:
		return clickHouseDetachFinalizer(ctx, c, wandb, key, spec)
	}
	return nil
}

func clickHouseInstanceInfraSpec(spec apiv2.ClickHouseSpec) apiv2.ManagedInfraSpec {
	if spec.ManagedClickHouse != nil {
		return spec.ManagedClickHouse.ManagedInfraSpec
	}
	return apiv2.ManagedInfraSpec{}
}

func clickHousePurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec apiv2.ClickHouseSpec,
) error {
	if managed := spec.ManagedClickHouse; managed != nil {
		specNamespacedName := managedClickHouseSpecNamespacedName(managed)
		onDeleteRule := altinity.ToClickHouseOnDeleteRule(wandb, wandb.GetRetentionPolicy(managed.ManagedInfraSpec))
		return altinity.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if spec.ExternalClickHouse != nil {
		return externalch.DeleteConnectionSecret(ctx, client, wandb, key)
	}
	return nil
}

func clickHouseDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	_ string,
	spec apiv2.ClickHouseSpec,
) error {
	managed := spec.ManagedClickHouse
	if managed == nil {
		return nil
	}
	specNamespacedName := managedClickHouseSpecNamespacedName(managed)
	return altinity.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedClickHouseWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	spec *apiv2.ManagedClickHouseSpec,
	mfst manifest.Manifest,
) []metav1.Condition {
	log := ctrl.LoggerFrom(ctx)

	// Altinity swallows the apiserver's rejection of over-long derived names, so
	// fail the status loudly here; also covers CRs that predate admission checks.
	if err := altinity.ValidateDerivedNames(spec); err != nil {
		log.Error(err, "managed ClickHouse name cannot be deployed")
		return []metav1.Condition{
			{
				Type:    common.ReconciledType,
				Status:  metav1.ConditionFalse,
				Reason:  common.InvalidNameReason,
				Message: err.Error(),
			},
			{
				Type:    altinity.ClickHouseCustomResourceType,
				Status:  metav1.ConditionFalse,
				Reason:  common.InvalidNameReason,
				Message: err.Error(),
			},
		}
	}

	// ClickHouse table data lives in the object store: use the "clickhouse"
	// instance when provisioned, otherwise the default instance.
	objStoreStatus, _ := apiv2.ResolveInstance(wandb.Status.ObjectStoreStatus, clickHouseObjectStoreInstance)
	objStoreSpec, _ := apiv2.ResolveInstance(wandb.Spec.ObjectStore, clickHouseObjectStoreInstance)
	waitForObjectStore := objStoreSpec.ManagedObjectStore != nil

	// Resolve the bucket connection; wait and requeue if it isn't ready yet.
	objStorage, err := altinity.ResolveObjectStorage(ctx, client, spec, &objStoreStatus.Connection)
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

	// Translate the Keeper and ClickHouse CRs; WriteState writes Keeper first.
	desiredKeeper, err := keeper.ToKeeperVendorSpec(ctx, wandb, spec, client.Scheme(), altinity.KeeperNsName(spec))
	if err != nil {
		log.Error(err, "failed to translate Keeper spec to vendor spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	desiredServiceAccount, err := altinity.ToServiceAccount(wandb, spec, objStorage, client.Scheme())
	if err != nil {
		log.Error(err, "failed to translate ClickHouse ServiceAccount")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}
	}

	desired, err := altinity.ToClickHouseVendorSpec(ctx, wandb, spec, client.Scheme(), objStorage, waitForObjectStore, mfst)
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

	specNamespacedName := managedClickHouseSpecNamespacedName(spec)

	if conditions := altinity.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID()); conditions != nil {
		return conditions
	}

	results := make([]metav1.Condition, 0)
	results = append(results, altinity.WriteState(ctx, client, specNamespacedName, desiredServiceAccount, desiredKeeper, desired)...)

	return results
}

func managedClickHouseReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	spec *apiv2.ManagedClickHouseSpec,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *apiv2.ClickHouseConnection) {
	specNamespacedName := managedClickHouseSpecNamespacedName(spec)
	onDeleteRule := altinity.ToClickHouseOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	readConditions, newInfraConn := altinity.ReadState(ctx, client, specNamespacedName, wandb, onDeleteRule)
	newConditions = append(newConditions, readConditions...)

	// Keeper readiness gates ClickHouse readiness (see inferInfraState).
	newConditions = append(newConditions, keeper.ReadState(ctx, client, altinity.KeeperNsName(spec))...)

	return newConditions, newInfraConn
}

func managedClickHouseInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	key string,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.ClickHouseConnection,
) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	enabled := true
	oldStatus := wandb.Status.ClickHouseStatus[key]
	oldConditions := oldStatus.Conditions
	oldInfraConn := oldStatus.Connection

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
	wandb.Status.ClickHouseStatus[key] = updatedStatus
	err := updateWandbStatusIfChanged(ctx, client, wandb, statusBefore)

	return ctrlResult, err
}

// external

func externalClickHouseInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, newConditions []metav1.Condition, newInfraConn *apiv2.ClickHouseConnection) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	oldStatus := wandb.Status.ClickHouseStatus[key]
	oldInfraConn := oldStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(oldStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.ClickHouseStatus[key] = apiv2.ClickHouseInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, updateWandbStatusIfChanged(ctx, c, wandb, statusBefore)
}

// helpers

func managedClickHouseSpecNamespacedName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
