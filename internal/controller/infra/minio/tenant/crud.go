package tenant

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "Tenant"
)

func CrudResource(
	ctx context.Context,
	client client.Client,
	namespacedName types.NamespacedName,
	desired *miniov2.Tenant,
) error {
	var err error
	var actual = &miniov2.Tenant{}

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
