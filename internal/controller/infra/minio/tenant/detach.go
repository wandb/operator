package tenant

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	miniov2 "github.com/wandb/operator/pkg/vendored/minio-operator/minio.min.io/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Minio)
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	return common.DetachOwnerReference(ctx, cl, nsnBuilder.SpecNsName(), ResourceTypeName, &miniov2.Tenant{}, wandbOwner.GetUID())
}
