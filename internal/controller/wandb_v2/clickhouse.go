package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/clickhouse"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
)

func (r *WeightsAndBiasesV2Reconciler) reconcileClickHouse(
	ctx context.Context,
	infraDetails translatorv2.InfraConfig,
	wandb *apiv2.WeightsAndBiases,
) *common.Results {
	var err error
	var results = &common.Results{}
	var nextResults *common.Results
	var clickhouseConfig common.ClickHouseConfig
	var actual clickhouse.ActualClickHouse

	if clickhouseConfig, err = infraDetails.GetClickHouseConfig(); err != nil {
		results.AddErrors(err)
		return results
	}

	if actual, err = altinity.Initialize(ctx, r.Client, clickhouseConfig, wandb, r.Scheme); err != nil {
		results.AddErrors(err)
		return results
	}

	if clickhouseConfig.Enabled {
		nextResults = actual.Upsert(ctx, clickhouseConfig)
	} else {
		nextResults = actual.Delete(ctx)
	}
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	wandb.Status.ClickHouseStatus = translatorv2.ExtractClickHouseStatus(ctx, results)
	if err = r.Status().Update(ctx, wandb); err != nil {
		results.AddErrors(err)
	}

	return results
}
