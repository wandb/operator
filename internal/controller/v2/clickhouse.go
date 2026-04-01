package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
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
		onDeleteRule := translatorv2.ToClickHouseOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
		return altinity.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule)
	}
	if wandb.Spec.ClickHouse.ExternalClickHouse != nil {
		return deleteWandbConnectionSecret(ctx, client, types.NamespacedName{
			Namespace: wandb.Namespace,
			Name:      clickHouseConnectionName,
		})
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
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
) []metav1.Condition {
	spec := wandb.Spec.ClickHouse.ExternalClickHouse
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"url":      spec.URL,
		"Host":     spec.Host,
		"Port":     spec.Port,
		"User":     spec.Username,
		"Password": spec.Password,
		"Database": spec.Database,
	}

	data := map[string]string{}
	for key, sel := range fields {
		val, err := resolveSecretKey(ctx, c, wandb.Namespace, sel)
		if err != nil {
			logger.Error(err, "failed to resolve external clickhouse field", "key", key)
			return []metav1.Condition{{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			}}
		}
		if val != "" {
			data[key] = val
		}
	}

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: clickHouseConnectionName}
	return writeExternalConnectionSecret(ctx, c, wandb, nsName, data)
}

func externalClickHouseReadState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) ([]metav1.Condition, *translator.ClickHouseConnection) {
	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: clickHouseConnectionName}
	secret := &corev1.Secret{}
	found, err := common.GetResource(ctx, c, nsName, "Secret", secret)
	if err != nil {
		return append(newConditions, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		}), nil
	}
	if !found {
		return newConditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return newConditions, &translator.ClickHouseConnection{
		URL:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Host:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		Username: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "User", Optional: ptr.To(false)},
		Password: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Password", Optional: ptr.To(false)},
		Database: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Database", Optional: ptr.To(false)},
	}
}

func externalClickHouseInferStatus(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.ClickHouseConnection,
) (ctrl.Result, error) {
	oldInfraConn := translatorv2.ToTranslatorClickHouseConnection(wandb.Status.ClickHouseStatus.Connection)
	conn := utils.Coalesce(newInfraConn, &oldInfraConn)

	state := common.HealthyState
	ready := true
	if newInfraConn == nil {
		state = common.ErrorState
		ready = false
	}

	updatedConditions := common.ComputeConditionUpdates(
		wandb.Status.ClickHouseStatus.Conditions,
		newConditions,
		wandb.Generation,
		translator.DefaultConditionExpiry,
	)

	wandb.Status.ClickHouseStatus = translatorv2.ToWbClickHouseInfraStatus(translator.ClickHouseStatus{
		InfraStatus: translator.InfraStatus{
			Ready:      ready,
			State:      state,
			Conditions: updatedConditions,
		},
		Connection: *conn,
	})
	return ctrl.Result{}, c.Status().Update(ctx, wandb)
}

// helpers

func managedClickHouseSpecNamespacedName(spec *apiv2.ManagedClickHouseSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
