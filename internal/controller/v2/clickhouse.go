package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
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
) []metav1.Condition {
	if wandb.Spec.ClickHouse.ManagedClickHouse != nil {
		return managedClickHouseWriteState(ctx, client, wandb)
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
) ([]metav1.Condition, *translator.ClickHouseConnection) {
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
	newInfraConn *translator.ClickHouseConnection,
) (ctrl.Result, error) {
	if wandb.Spec.ClickHouse.ManagedClickHouse != nil {
		return managedClickHouseInferStatus(ctx, client, recorder, wandb, newConditions, newInfraConn)
	}
	// TODO: external clickhouse infer status
	return ctrl.Result{}, nil
}

func clickHousePurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	if spec == nil {
		return nil
	}
	specNamespacedName := managedClickHouseSpecNamespacedName(spec)
	onDeleteRule := translatorv2.ToClickHouseOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	return altinity.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
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
) []metav1.Condition {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse

	var specNamespacedName = managedClickHouseSpecNamespacedName(spec)
	log := ctrl.LoggerFrom(ctx)
	desired, err := translatorv2.ToClickHouseVendorSpec(ctx, wandb, client.Scheme())
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

func managedClickHouseReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.ClickHouseConnection) {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse

	specNamespacedName := managedClickHouseSpecNamespacedName(spec)
	onDeleteRule := translatorv2.ToClickHouseOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
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
	newInfraConn *translator.ClickHouseConnection,
) (ctrl.Result, error) {
	enabled := true
	oldConditions := wandb.Status.ClickHouseStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorClickHouseConnection(wandb.Status.ClickHouseStatus.Connection)

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
	wandb.Status.ClickHouseStatus = translatorv2.ToWbClickHouseInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

// external

func externalClickHouseWriteState(
	_ context.Context,
	_ client.Client,
	_ *apiv2.WeightsAndBiases,
) []metav1.Condition {
	// TODO: implement external clickhouse write state
	return nil
}

func externalClickHouseReadState(
	_ context.Context,
	_ client.Client,
	_ *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.ClickHouseConnection) {
	// TODO: implement external clickhouse read state
	return newConditions, nil
}

// helpers

func managedClickHouseSpecNamespacedName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
