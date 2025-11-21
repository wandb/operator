package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *WeightsAndBiasesV2Reconciler) minioResourceReconcile(
	ctx context.Context,
	wandb *apiv2.WeightsAndBiases,
) error {
	var err error
	var desired *miniov2.Tenant
	var specNamespacedName = minioSpecNamespacedName(wandb.Spec.Minio)

	if desired, err = translatorv2.ToMinioVendorSpec(ctx, wandb.Spec.Minio, wandb, r.Scheme); err != nil {
		return err
	}
	if err = tenant.CrudResource(ctx, r.Client, specNamespacedName, desired); err != nil {
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
	log := ctrl.LoggerFrom(ctx)

	var err error
	var conditions []common.MinioCondition
	var specNamespacedName = minioSpecNamespacedName(wandb.Spec.Minio)

	if conditions, err = tenant.GetConditions(ctx, r.Client, specNamespacedName); err != nil {
		return err
	}
	wandb.Status.MinioStatus = translatorv2.ExtractMinioStatus(ctx, conditions)
	if err = r.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "failed to update status")
		return err
	}

	return nil
}

func minioSpecNamespacedName(minio apiv2.WBMinioSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: minio.Namespace,
		Name:      minio.Name,
	}
}
