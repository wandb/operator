package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	"github.com/wandb/operator/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func minioWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) ([]metav1.Condition, *translator.InfraConnection) {
	var specNamespacedName = minioSpecNamespacedName(wandb.Spec.Minio)

	desiredCr, err := translatorv2.ToMinioVendorSpec(ctx, wandb.Spec.Minio, wandb, client.Scheme())
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ControllerErrorReason,
			},
		}, nil
	}

	desiredConfig, err := translatorv2.ToMinioEnvConfig(ctx, wandb.Spec.Minio)
	if err != nil {
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
	specNamespacedName := minioSpecNamespacedName(wandb.Spec.Minio)
	readConditions := tenant.ReadState(ctx, client, specNamespacedName)
	newConditions = append(newConditions, readConditions...)
	return newConditions
}

func minioInferStatus(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	newConditions []metav1.Condition,
	newInfraConn *translator.InfraConnection,
) error {
	oldConditions := wandb.Status.MinioStatus.Conditions
	oldInfraConn := translatorv2.ToTranslatorInfraConnection(wandb.Status.MinioStatus.Connection)

	updatedStatus := tenant.ComputeStatus(
		oldConditions,
		newConditions,
		utils.Coalesce(newInfraConn, &oldInfraConn),
		wandb.Generation,
	)
	wandb.Status.MinioStatus = translatorv2.ToWbInfraStatus(updatedStatus)
	err := client.Status().Update(ctx, wandb)

	return err
}

func minioSpecNamespacedName(minio apiv2.WBMinioSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: minio.Namespace,
		Name:      minio.Name,
	}
}
