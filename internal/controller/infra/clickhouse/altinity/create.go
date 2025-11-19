package altinity

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (a *altinityClickHouse) createCHI(
	ctx context.Context, desiredCHI *chiv1.ClickHouseInstallation,
) *common.Results {
	log := ctrl.LoggerFrom(ctx)
	results := common.InitResults()

	if a.chi != nil {
		msg := "cannot create CHI CR when it already exists"
		err := common.NewClickHouseError(common.ClickHouseErrFailedToCreateCode, msg)
		log.Error(err, msg)
		results.AddErrors(err)
		return results
	}

	if err := a.client.Create(ctx, desiredCHI); err != nil {
		log.Error(err, "Failed to create CHI CR")
		results.AddErrors(common.NewClickHouseError(
			common.ClickHouseErrFailedToCreateCode,
			fmt.Sprintf("failed to create CHI CR: %v", err),
		))
		return results
	}

	results.AddStatuses(
		common.NewClickHouseStatusDetail(common.ClickHouseCreatedCode, fmt.Sprintf("Created CHI CR: %s", CHIName)),
	)

	return results
}
