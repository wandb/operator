package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
)

func (r *WeightsAndBiasesV2Reconciler) minioResourceReconcile(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desired *miniov2.Tenant

	if desired, err = translatorv2.ToMinioVendorSpec(ctx, wandb.Spec.Minio, wandb, r.Scheme); err != nil {
		return err
	}
	if err = tenant.CrudResource(ctx, r.Client, translatorv2.MinioNamespacedName(wandb.Spec.Minio), desired); err != nil {
		return err
	}

	//wandb.Status.MinioStatus = translatorv2.ExtractMinioStatus(ctx, results)
	//if err = r.Status().Update(ctx, wandb); err != nil {
	//	results.AddErrors(err)
	//}

	return nil
}

func (r *WeightsAndBiasesV2Reconciler) minioStatusUpdate(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var conditions []common.MinioCondition

	if conditions, err = tenant.GetConditions(
		ctx,
		r.Client,
		translatorv2.MinioNamespacedName(wandb.Spec.Minio),
	); err != nil {
		return err
	}
	wandb.Status.MinioStatus = translatorv2.ExtractMinioStatus(ctx, conditions)
	if err = r.Status().Update(ctx, wandb); err != nil {
		return err
	}

	return nil
}
