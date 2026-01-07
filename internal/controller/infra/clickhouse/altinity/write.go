package altinity

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "ClickHouseInstallation"
	AppConnTypeName  = "ClickHouseAppConn"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desired *chiv1.ClickHouseInstallation,
) error {
	var err error
	var found bool
	var actual = &chiv1.ClickHouseInstallation{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.InstallationNsName(), ResourceTypeName, actual,
	); err != nil {
		return err
	}
	if !found {
		actual = nil
	}

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}
