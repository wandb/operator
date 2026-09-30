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
	deployment common.DeploymentResource,
	config manifest.InfraConfig,
) (map[string][]metav1.Condition, map[string]*apiv2.ObjectStoreConnection) {
	outConds := map[string][]metav1.Condition{}
	outConns := map[string]*apiv2.ObjectStoreConnection{}
	for key, spec := range deployment.GetBaseDeploymentSpec().ObjectStore {
		switch {
		case spec.ManagedObjectStore != nil:
			outConds[key], outConns[key] = managedObjectStoreWriteState(ctx, client, deployment, spec.ManagedObjectStore, config)
		case spec.ExternalObjectStore != nil:
			outConds[key], outConns[key] = externalobjectstore.WriteState(ctx, client, deployment, key, spec.ExternalObjectStore)
		}
	}
	return outConds, outConns
}

func objectStoreReadState(
	ctx context.Context,
	client client.Client,
	deployment common.DeploymentResource,
	conditions map[string][]metav1.Condition,
) map[string][]metav1.Condition {
	out := map[string][]metav1.Condition{}
	for key, spec := range deployment.GetBaseDeploymentSpec().ObjectStore {
		switch {
		case spec.ManagedObjectStore != nil:
			out[key] = managedObjectStoreReadState(ctx, client, deployment, spec.ManagedObjectStore, conditions[key])
		case spec.ExternalObjectStore != nil:
			out[key] = externalobjectstore.ReadState(ctx, client, deployment, key, conditions[key])
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
	deployment common.DeploymentResource,
	conditions map[string][]metav1.Condition,
	infraConns map[string]*apiv2.ObjectStoreConnection,
) (ctrl.Result, error) {
	if deployment.GetBaseDeploymentStatus().ObjectStoreStatus == nil {
		deployment.GetBaseDeploymentStatus().ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{}
	}
	var results []ctrl.Result
	var firstErr error
	for key, spec := range deployment.GetBaseDeploymentSpec().ObjectStore {
		var res ctrl.Result
		var err error
		switch {
		case spec.ManagedObjectStore != nil:
			res, err = managedObjectStoreInferStatus(ctx, client, recorder, deployment, key, conditions[key], infraConns[key])
		case spec.ExternalObjectStore != nil:
			res, err = externalObjectStoreInferStatus(ctx, client, deployment, key, conditions[key], infraConns[key])
		}
		results = append(results, res)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return consolidateResults(results), firstErr
}

func runObjectStoreRetentionFinalizer(ctx context.Context, c client.Client, deployment common.DeploymentResource, key string, spec apiv2.ObjectStoreSpec) error {
	switch deployment.GetBaseDeploymentSpec().GetRetentionPolicy(objectStoreInstanceInfraSpec(spec)).OnDelete {
	case apiv2.PurgeOnDelete:
		return objectStorePurgeFinalizer(ctx, c, deployment, key, spec)
	case apiv2.DetachOnDelete:
		return objectStoreDetachFinalizer(ctx, c, deployment, key, spec)
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
	deployment common.DeploymentResource,
	key string,
	spec apiv2.ObjectStoreSpec,
) error {
	if managed := spec.ManagedObjectStore; managed != nil {
		onDeleteRule := seaweedfs.ToObjectStoreOnDeleteRule(deployment, deployment.GetBaseDeploymentSpec().GetRetentionPolicy(managed.ManagedInfraSpec))
		specNamespacedName := managedObjectStoreSpecNamespacedName(managed)
		return seaweedfs.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if spec.ExternalObjectStore != nil {
		return externalobjectstore.DeleteConnectionSecret(ctx, client, deployment, key)
	}
	return nil
}

func objectStoreDetachFinalizer(
	ctx context.Context,
	client client.Client,
	deployment common.DeploymentResource,
	_ string,
	spec apiv2.ObjectStoreSpec,
) error {
	managed := spec.ManagedObjectStore
	if managed == nil {
		return nil
	}
	specNamespacedName := managedObjectStoreSpecNamespacedName(managed)
	return seaweedfs.DetachFinalizer(ctx, client, specNamespacedName, deployment)
}

// managed

func managedObjectStoreWriteState(
	ctx context.Context,
	client client.Client,
	deployment common.DeploymentResource,
	spec *apiv2.ManagedObjectStoreSpec,
	config manifest.InfraConfig,
) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
	log := ctrl.LoggerFrom(ctx)
	var specNamespacedName = managedObjectStoreSpecNamespacedName(spec)

	if conditions := seaweedfs.CheckDetached(ctx, client, specNamespacedName, deployment.GetUID(), spec.Replicas); conditions != nil {
		return conditions, nil
	}

	desiredCr, err := seaweedfs.ToObjectStoreVendorSpec(ctx, deployment, deployment.GetBaseDeploymentSpec(), spec, client.Scheme(), config)
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

	conditions, connection := seaweedfs.WriteState(ctx, client, specNamespacedName, desiredCr, desiredConfig, deployment)
	return conditions, connection
}

func managedObjectStoreReadState(
	ctx context.Context,
	client client.Client,
	deployment common.DeploymentResource,
	spec *apiv2.ManagedObjectStoreSpec,
	newConditions []metav1.Condition,
) []metav1.Condition {
	specNamespacedName := managedObjectStoreSpecNamespacedName(spec)
	retentionPolicy := deployment.GetBaseDeploymentSpec().GetRetentionPolicy(spec.ManagedInfraSpec)
	readConditions := seaweedfs.ReadState(
		ctx,
		client,
		specNamespacedName,
		seaweedfs.ToObjectStoreOnDeleteRule(deployment, retentionPolicy),
	)
	newConditions = append(newConditions, readConditions...)
	return newConditions
}

func managedObjectStoreInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	deployment common.DeploymentResource,
	key string,
	newConditions []metav1.Condition,
	newInfraConn *apiv2.ObjectStoreConnection,
) (ctrl.Result, error) {
	statusBefore := deployment.GetBaseDeploymentStatus().DeepCopy()
	enabled := true
	oldStatus := deployment.GetBaseDeploymentStatus().ObjectStoreStatus[key]
	oldConditions := oldStatus.Conditions
	oldInfraConn := oldStatus.Connection

	updatedStatus, events, ctrlResult := seaweedfs.ComputeStatus(
		ctx,
		enabled,
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		deployment.GetGeneration(),
	)
	for _, e := range events {
		recorder.Event(deployment, e.Type, e.Reason, e.Message)
	}
	deployment.GetBaseDeploymentStatus().ObjectStoreStatus[key] = updatedStatus
	err := updateDeploymentStatusIfChanged(ctx, client, deployment, statusBefore)

	return ctrlResult, err
}

// external

func externalObjectStoreInferStatus(ctx context.Context, c client.Client, deployment common.DeploymentResource, key string, newConditions []metav1.Condition, newInfraConn *apiv2.ObjectStoreConnection) (ctrl.Result, error) {
	statusBefore := deployment.GetBaseDeploymentStatus().DeepCopy()
	oldStatus := deployment.GetBaseDeploymentStatus().ObjectStoreStatus[key]
	oldInfraConn := oldStatus.Connection
	state, ready, updatedConditions := external.InferExternalStatus(oldStatus.Conditions, newConditions, deployment.GetGeneration(), newInfraConn != nil)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	deployment.GetBaseDeploymentStatus().ObjectStoreStatus[key] = apiv2.ObjectStoreInfraStatus{
		WBInfraStatus: apiv2.WBInfraStatus{Ready: ready, State: state, Conditions: updatedConditions},
		Connection:    *conn,
	}
	return ctrl.Result{}, updateDeploymentStatusIfChanged(ctx, c, deployment, statusBefore)
}

// helpers

func managedObjectStoreSpecNamespacedName(spec *apiv2.ManagedObjectStoreSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
