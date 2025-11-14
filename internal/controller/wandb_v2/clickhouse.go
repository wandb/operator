package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/clickhouse"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/model"
)

func (r *WeightsAndBiasesV2Reconciler) reconcileClickHouse(
	ctx context.Context,
	infraDetails model.InfraConfig,
	wandb *apiv2.WeightsAndBiases,
) *model.Results {
	var err error
	var results = &model.Results{}
	var nextResults *model.Results
	var clickhouseConfig model.ClickHouseConfig
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

	wandb.Status.ClickHouseStatus = results.ExtractClickHouseStatus(ctx)
	if err = r.Status().Update(ctx, wandb); err != nil {
		results.AddErrors(err)
	}

	return results
}
