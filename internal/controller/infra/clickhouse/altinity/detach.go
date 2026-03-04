package altinity

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.ClickHouse)
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	return common.DetachOwnerReference(ctx, cl, nsnBuilder.InstallationNsName(), ResourceTypeName, &chiv1.ClickHouseInstallation{}, wandbOwner.GetUID())
}
