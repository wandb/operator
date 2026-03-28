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
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	if spec == nil {
		return nil
	}

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

func clickHouseReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.InfraConnection) {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	if spec == nil {
		return newConditions, nil
	}

	specNamespacedName := managedClickHouseSpecNamespacedName(spec)
	onDeleteRule := translatorv2.ToClickHouseOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	readConditions, newInfraConn := altinity.ReadState(ctx, client, specNamespacedName, wandb, onDeleteRule)
	newConditions = append(newConditions, readConditions...)
	return newConditions, newInfraConn
}

func clickHouseInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.InfraConnection,
) (ctrl.Result, error) {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	if spec == nil {
		return ctrl.Result{}, nil
	}

	enabled := true
	oldConditions := wandb.Status.ClickHouseStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorInfraConnection(wandb.Status.ClickHouseStatus.Connection)

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
	wandb.Status.ClickHouseStatus = translatorv2.ToWbInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
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

func managedClickHouseSpecNamespacedName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
