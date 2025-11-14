package wandb_v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/model"
)

func (r *WeightsAndBiasesV2Reconciler) reconcileMySQL(
	ctx context.Context,
	infraDetails model.InfraConfig,
	wandb *apiv2.WeightsAndBiases,
) *model.Results {
	var err error
	var results = &model.Results{}
	var nextResults *model.Results
	var mysqlConfig model.MySQLConfig
	var actual mysql.ActualMySQL

	if mysqlConfig, err = infraDetails.GetMySQLConfig(); err != nil {
		results.AddErrors(err)
		return results
	}

	if actual, err = percona.Initialize(ctx, r.Client, mysqlConfig, wandb, r.Scheme); err != nil {
		results.AddErrors(err)
		return results
	}

	if mysqlConfig.Enabled {
		nextResults = actual.Upsert(ctx, mysqlConfig)
	} else {
		nextResults = actual.Delete(ctx)
	}
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	wandb.Status.MySQLStatus = results.ExtractMySQLStatus(ctx)
	if err = r.Status().Update(ctx, wandb); err != nil {
		results.AddErrors(err)
	}

	return results
}
