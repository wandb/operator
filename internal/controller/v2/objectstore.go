package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	externalobjectstore "github.com/wandb/operator/internal/controller/infra/external/objectstore"
	"github.com/wandb/operator/internal/controller/infra/managed/minio/tenant"
	"github.com/wandb/operator/pkg/utils"
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
) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
	if wandb.Spec.ObjectStore.ManagedObjectStore != nil {
		return managedObjectStoreWriteState(ctx, client, wandb)
	}
	if wandb.Spec.ObjectStore.ExternalObjectStore != nil {
		return externalObjectStoreWriteState(ctx, client, wandb)
	}
	return nil, nil
}

func objectStoreReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) []metav1.Condition {
	if wandb.Spec.ObjectStore.ManagedObjectStore != nil {
		return managedObjectStoreReadState(ctx, client, wandb, newConditions)
	}
	if wandb.Spec.ObjectStore.ExternalObjectStore != nil {
		return externalObjectStoreReadState(ctx, client, wandb, newConditions)
	}
	return newConditions
}

func objectStoreInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.ObjectStoreConnection,
) (ctrl.Result, error) {
	if wandb.Spec.ObjectStore.ManagedObjectStore != nil {
		return managedObjectStoreInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
	}
	if wandb.Spec.ObjectStore.ExternalObjectStore != nil {
		return externalObjectStoreInferStatus(ctx, client, wandb, newConditions, newInfraConn)
	}
	return ctrl.Result{}, nil
}

func objectStorePurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	if spec := wandb.Spec.ObjectStore.ManagedObjectStore; spec != nil {
		specNamespacedName := managedObjectStoreSpecNamespacedName(spec)
		onDeleteRule := tenant.ToObjectStoreOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return tenant.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if wandb.Spec.ObjectStore.ExternalObjectStore != nil {
		return externalobjectstore.DeleteConnectionSecret(ctx, client, wandb)
	}
	return nil
}

func objectStoreDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.ObjectStore.ManagedObjectStore
	if spec == nil {
		return nil
	}
	specNamespacedName := managedObjectStoreSpecNamespacedName(spec)
	return tenant.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

// managed

func managedObjectStoreWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
	spec := wandb.Spec.ObjectStore.ManagedObjectStore

	log := ctrl.LoggerFrom(ctx)
	var specNamespacedName = managedObjectStoreSpecNamespacedName(spec)

	if conditions := tenant.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
		return conditions, nil
	}

	desiredCr, err := tenant.ToObjectStoreVendorSpec(ctx, wandb, client.Scheme())
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

	desiredConfig, err := tenant.ToObjectStoreEnvConfig(ctx, *spec)
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

	conditions, connection := tenant.WriteState(ctx, client, specNamespacedName, desiredCr, desiredConfig, wandb)
	return conditions, connection
}

func managedObjectStoreReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) []metav1.Condition {
	spec := wandb.Spec.ObjectStore.ManagedObjectStore

	specNamespacedName := managedObjectStoreSpecNamespacedName(spec)
	retentionPolicy := wandb.GetRetentionPolicy(spec.ManagedInfraSpec)
	readConditions := tenant.ReadState(
		ctx,
		client,
		specNamespacedName,
		tenant.ToObjectStoreOnDeleteRule(wandb, retentionPolicy),
	)
	newConditions = append(newConditions, readConditions...)
	return newConditions
}

func managedObjectStoreInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.ObjectStoreConnection,
) (ctrl.Result, error) {
	enabled := true
	oldConditions := wandb.Status.ObjectStoreStatus.Conditions
	oldInfraConn := wandb.Status.ObjectStoreStatus.Connection

	updatedStatus, events, ctrlResult := tenant.ComputeStatus(
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
	wandb.Status.ObjectStoreStatus = updatedStatus
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalObjectStoreWriteState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
	return externalobjectstore.WriteState(ctx, c, wandb)
}

func externalObjectStoreReadState(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition) []metav1.Condition {
	return externalobjectstore.ReadState(ctx, c, wandb, newConditions)
}

func externalObjectStoreInferStatus(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, newConditions []metav1.Condition, newInfraConn *apiv2.ObjectStoreConnection) (ctrl.Result, error) {
	oldInfraConn := wandb.Status.ObjectStoreStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(wandb.Status.ObjectStoreStatus.Conditions, newConditions, wandb.Generation, newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	wandb.Status.ObjectStoreStatus = apiv2.ObjectStoreInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, c.Status().Update(ctx, wandb)
}

// helpers

func managedObjectStoreSpecNamespacedName(spec *apiv2.ManagedObjectStoreSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
