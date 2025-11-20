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
)

func CrudResource(
	ctx context.Context,
	client client.Client,
	namespacedName types.NamespacedName,
	desired *chiv1.ClickHouseInstallation,
) error {
	var err error
	var actual = &chiv1.ClickHouseInstallation{}

	if err = common.GetResource(
		ctx, client, namespacedName, ResourceTypeName, actual,
	); err != nil {
		return err
	}
	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}
