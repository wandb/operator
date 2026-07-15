package reconciler

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	externalobjectstore "github.com/wandb/operator/internal/controller/infra/external/objectstore"
	"github.com/wandb/operator/internal/controller/infra/managed/objectstore/seaweedfs"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func objectStoreWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	mfst manifest.Manifest,
) (map[string][]metav1.Condition, map[string]*apiv2.ObjectStoreConnection) {
	outConds := map[string][]metav1.Condition{}
	outConns := map[string]*apiv2.ObjectStoreConnection{}
	for key, spec := range wandb.Spec.ObjectStore {
		switch {
		case spec.ManagedObjectStore != nil:
			outConds[key], outConns[key] = managedObjectStoreWriteState(ctx, client, wandb, key, spec.ManagedObjectStore, mfst)
		case spec.ExternalObjectStore != nil:
			outConds[key], outConns[key] = externalobjectstore.WriteState(ctx, client, wandb, key, spec.ExternalObjectStore)
		}
	}
	return outConds, outConns
}

func objectStoreReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	conditions map[string][]metav1.Condition,
) map[string][]metav1.Condition {
	out := map[string][]metav1.Condition{}
	for key, spec := range wandb.Spec.ObjectStore {
		switch {
		case spec.ManagedObjectStore != nil:
			out[key] = managedObjectStoreReadState(ctx, client, wandb, spec.ManagedObjectStore, conditions[key])
		case spec.ExternalObjectStore != nil:
			out[key] = externalobjectstore.ReadState(ctx, client, wandb, key, conditions[key])
		default:
			out[key] = conditions[key]
		}
	}
	return out
}

func objectStoreInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	conditions map[string][]metav1.Condition,
	infraConns map[string]*apiv2.ObjectStoreConnection,
) (ctrl.Result, error) {
	if wandb.Status.ObjectStoreStatus == nil {
		wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{}
	}
	var results []ctrl.Result
	var firstErr error
	for key, spec := range wandb.Spec.ObjectStore {
		var res ctrl.Result
		var err error
		switch {
		case spec.ManagedObjectStore != nil:
			res, err = managedObjectStoreInferStatus(ctx, client, recorder, wandb, key, conditions[key], infraConns[key])
		case spec.ExternalObjectStore != nil:
			res, err = externalObjectStoreInferStatus(ctx, client, wandb, key, conditions[key], infraConns[key])
		}
		results = append(results, res)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return consolidateResults(results), firstErr
}

func runObjectStoreRetentionFinalizer(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, spec apiv2.ObjectStoreSpec) error {
	switch wandb.GetRetentionPolicy(objectStoreInstanceInfraSpec(spec)).OnDelete {
	case apiv2.PurgeOnDelete:
		return objectStorePurgeFinalizer(ctx, c, wandb, key, spec)
	case apiv2.DetachOnDelete:
		return objectStoreDetachFinalizer(ctx, c, wandb, key, spec)
	}
	return nil
}

func objectStoreInstanceInfraSpec(spec apiv2.ObjectStoreSpec) apiv2.ManagedInfraSpec {
	if spec.ManagedObjectStore != nil {
		return spec.ManagedObjectStore.ManagedInfraSpec
	}
	return apiv2.ManagedInfraSpec{}
}

func objectStorePurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec apiv2.ObjectStoreSpec,
) error {
	if managed := spec.ManagedObjectStore; managed != nil {
		onDeleteRule := seaweedfs.ToObjectStoreOnDeleteRule(wandb, wandb.GetRetentionPolicy(managed.ManagedInfraSpec))
		// Legacy MinIO predates multi-instance and is scoped to the CR, so only
		// the default instance triggers its cleanup.
		if key == apiv2.DefaultInstanceName {
			_ = seaweedfs.CleanupLegacyMinio(
				ctx, client, wandb.Name, wandb.Namespace, wandb.GetUID(),
				true, onDeleteRule.Selector,
			)
		}
		specNamespacedName := managedObjectStoreSpecNamespacedName(managed)
		return seaweedfs.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if spec.ExternalObjectStore != nil {
		return externalobjectstore.DeleteConnectionSecret(ctx, client, wandb, key)
	}
	return nil
}

func objectStoreDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec apiv2.ObjectStoreSpec,
) error {
	managed := spec.ManagedObjectStore
	if managed == nil {
		return nil
	}
	if key == apiv2.DefaultInstanceName {
		_ = seaweedfs.CleanupLegacyMinio(
			ctx, client, wandb.Name, wandb.Namespace, wandb.GetUID(),
			false, nil,
		)
	}
	specNamespacedName := managedObjectStoreSpecNamespacedName(managed)
	return seaweedfs.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedObjectStoreWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec *apiv2.ManagedObjectStoreSpec,
	mfst manifest.Manifest,
) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
	log := ctrl.LoggerFrom(ctx)
	var specNamespacedName = managedObjectStoreSpecNamespacedName(spec)

	retentionPolicy := wandb.GetRetentionPolicy(spec.ManagedInfraSpec)
	onDeleteRule := seaweedfs.ToObjectStoreOnDeleteRule(wandb, retentionPolicy)
	if key == apiv2.DefaultInstanceName {
		if err := seaweedfs.CleanupLegacyMinio(
			ctx, client,
			wandb.Name, wandb.Namespace, wandb.GetUID(),
			onDeleteRule.Policy == common.Purge,
			onDeleteRule.Selector,
		); err != nil {
			log.Error(err, "failed to clean up legacy MinIO resources")
		}
	}

	if conditions := seaweedfs.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
		return conditions, nil
	}

	desiredCr, err := seaweedfs.ToObjectStoreVendorSpec(ctx, wandb, spec, client.Scheme(), mfst)
	if err != nil {
		log.Error(err, "failed to translate object store spec to vendor spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}, nil
	}

	desiredConfig, err := seaweedfs.ToObjectStoreEnvConfig(ctx, *spec)
	if err != nil {
		log.Error(err, "failed to translate object store envConfig to vendor spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}, nil
	}

	conditions, connection := seaweedfs.WriteState(ctx, client, specNamespacedName, desiredCr, desiredConfig, wandb)
	return conditions, connection
}

func managedObjectStoreReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	spec *apiv2.ManagedObjectStoreSpec,
	newConditions []metav1.Condition,
) []metav1.Condition {
	specNamespacedName := managedObjectStoreSpecNamespacedName(spec)
	retentionPolicy := wandb.GetRetentionPolicy(spec.ManagedInfraSpec)
	readConditions := seaweedfs.ReadState(
		ctx,
		client,
		specNamespacedName,
		seaweedfs.ToObjectStoreOnDeleteRule(wandb, retentionPolicy),
	)
	newConditions = append(newConditions, readConditions...)
	return newConditions
}

func managedObjectStoreInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	key string,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.ObjectStoreConnection,
) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	enabled := true
	oldStatus := wandb.Status.ObjectStoreStatus[key]
	oldConditions := oldStatus.Conditions
	oldInfraConn := oldStatus.Connection

	updatedStatus, events, ctrlResult := seaweedfs.ComputeStatus(
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
	wandb.Status.ObjectStoreStatus[key] = updatedStatus
	err := updateWandbStatusIfChanged(ctx, client, wandb, statusBefore)

	return ctrlResult, err
}

// external

func externalObjectStoreInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string, newConditions []metav1.Condition, newInfraConn *apiv2.ObjectStoreConnection) (ctrl.Result, error) {
	statusBefore := wandb.DeepCopy().Status
	oldStatus := wandb.Status.ObjectStoreStatus[key]
	oldInfraConn := oldStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(oldStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.ObjectStoreStatus[key] = apiv2.ObjectStoreInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, updateWandbStatusIfChanged(ctx, c, wandb, statusBefore)
}

// helpers

func managedObjectStoreSpecNamespacedName(spec *apiv2.ManagedObjectStoreSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
