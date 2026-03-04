package mysql

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	v2 "github.com/wandb/operator/pkg/vendored/mysql-operator/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Mysql)
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	return common.DetachOwnerReference(ctx, cl, nsnBuilder.ClusterNsName(), ResourceTypeName, &v2.InnoDBCluster{}, wandbOwner.GetUID())
}
