package mariadb

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/vendored/mariadb-operator/k8s.mariadb.com/v1alpha1"
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
	return common.DetachOwnerReference(ctx, cl, nsnBuilder.ClusterNsName(), ResourceTypeName, &v1alpha1.MariaDB{}, wandbOwner.GetUID())
}
