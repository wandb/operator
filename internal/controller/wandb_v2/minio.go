package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/minio"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
)

func (r *WeightsAndBiasesV2Reconciler) reconcileMinio(
	ctx context.Context,
	infraDetails translatorv2.InfraConfig,
	wandb *apiv2.WeightsAndBiases,
) *common.Results {
	var err error
	var results = &common.Results{}
	var nextResults *common.Results
	var minioConfig common.MinioConfig
	var actual minio.ActualMinio

	if minioConfig, err = infraDetails.GetMinioConfig(); err != nil {
		results.AddErrors(err)
		return results
	}

	if actual, err = tenant.Initialize(ctx, r.Client, minioConfig, wandb, r.Scheme); err != nil {
		results.AddErrors(err)
		return results
	}

	if minioConfig.Enabled {
		nextResults = actual.Upsert(ctx, minioConfig)
	} else {
		nextResults = actual.Delete(ctx)
	}
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	wandb.Status.MinioStatus = translatorv2.ExtractMinioStatus(ctx, results)
	if err = r.Status().Update(ctx, wandb); err != nil {
		results.AddErrors(err)
	}

	return results
}
