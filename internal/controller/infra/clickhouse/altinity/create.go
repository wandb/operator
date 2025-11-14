package altinity

import (
	"context"
	"fmt"

	chiv1 "github.com/wandb/operator/api/altinity-clickhouse-vendored/clickhouse.altinity.com/v1"
	"github.com/wandb/operator/internal/model"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (a *altinityClickHouse) createCHI(
	ctx context.Context, desiredCHI *chiv1.ClickHouseInstallation,
) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	if a.chi != nil {
		msg := "cannot create CHI CR when it already exists"
		err := model.NewClickHouseError(model.ClickHouseErrFailedToCreate, msg)
		log.Error(err, msg)
		results.AddErrors(err)
		return results
	}

	if err := a.client.Create(ctx, desiredCHI); err != nil {
		log.Error(err, "Failed to create CHI CR")
		results.AddErrors(model.NewClickHouseError(
			model.ClickHouseErrFailedToCreate,
			fmt.Sprintf("failed to create CHI CR: %v", err),
		))
		return results
	}

	results.AddStatuses(
		model.NewClickHouseStatus(model.ClickHouseCreated, fmt.Sprintf("Created CHI CR: %s", CHIName)),
	)

	return results
}
