package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func minioWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) ([]metav1.Condition, *translator.InfraConnection) {
	spec := wandb.Spec.Minio.ManagedMinio
	if spec == nil {
		return nil, nil
	}

	log := ctrl.LoggerFrom(ctx)
	var specNamespacedName = managedMinioSpecNamespacedName(spec)

	if conditions := tenant.CheckDetached(ctx, client, specNamespacedName, wandb.GetUID(), spec.Replicas); conditions != nil {
		return conditions, nil
	}

	desiredCr, err := translatorv2.ToMinioVendorSpec(ctx, wandb, client.Scheme())
	if err != nil {
		log.Error(err, "failed to translate MinIO spec to vendor spec")
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}, nil
	}

	desiredConfig, err := translatorv2.ToMinioEnvConfig(ctx, *spec)
	if err != nil {
		log.Error(err, "failed to translate MinIO envConfig to vendor spec")
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

func minioReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
) []metav1.Condition {
	spec := wandb.Spec.Minio.ManagedMinio
	if spec == nil {
		return newConditions
	}

	specNamespacedName := managedMinioSpecNamespacedName(spec)
	retentionPolicy := wandb.GetRetentionPolicy(spec.ManagedInfraSpec)
	readConditions := tenant.ReadState(
		ctx,
		client,
		specNamespacedName,
		translatorv2.ToMinioOnDeleteRule(wandb, retentionPolicy),
	)
	newConditions = append(newConditions, readConditions...)
	return newConditions
}

func minioInferStatus(
	ctx context.Context,
	client client.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.InfraConnection,
) (ctrl.Result, error) {
	spec := wandb.Spec.Minio.ManagedMinio
	if spec == nil {
		return ctrl.Result{}, nil
	}

	enabled := true
	oldConditions := wandb.Status.MinioStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorInfraConnection(wandb.Status.MinioStatus.Connection)

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
	wandb.Status.MinioStatus = translatorv2.ToWbInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return ctrlResult, err
}

func minioPurgeFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.Minio.ManagedMinio
	if spec == nil {
		return nil
	}
	var specNamespacedName = managedMinioSpecNamespacedName(spec)

	onDeleteRule := translatorv2.ToMinioOnDeleteRule(wandb, wandb.GetRetentionPolicy(spec.ManagedInfraSpec))
	if err := tenant.PurgeFinalizer(ctx, client, specNamespacedName, onDeleteRule); err != nil {
		return err
	}
	return nil
}

func minioDetachFinalizer(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	spec := wandb.Spec.Minio.ManagedMinio
	if spec == nil {
		return nil
	}
	specNamespacedName := managedMinioSpecNamespacedName(spec)
	return tenant.DetachFinalizer(ctx, client, specNamespacedName, wandb)
}

func managedMinioSpecNamespacedName(spec *apiv2.ManagedMinioSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: spec.Namespace,
		Name:      spec.Name,
	}
}
