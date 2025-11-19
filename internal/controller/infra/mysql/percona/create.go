package percona

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/translator/common"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (a *perconaPXC) createPXC(
	ctx context.Context, desiredPXC *pxcv1.PerconaXtraDBCluster,
) *common.Results {
	log := ctrl.LoggerFrom(ctx)
	results := common.InitResults()

	if a.pxc != nil {
		msg := "cannot create PXC CR when it already exists"
		err := common.NewMySQLError(common.MySQLErrFailedToCreateCode, msg)
		log.Error(err, msg)
		results.AddErrors(err)
		return results
	}

	if err := a.client.Create(ctx, desiredPXC); err != nil {
		log.Error(err, "Failed to create PXC CR")
		results.AddErrors(common.NewMySQLError(
			common.MySQLErrFailedToCreateCode,
			fmt.Sprintf("failed to create PXC CR: %v", err),
		))
		return results
	}

	results.AddStatuses(
		common.NewMySQLStatusDetail(common.MySQLCreatedCode, fmt.Sprintf("Created PXC CR: %s", PXCName)),
	)

	return results
}
