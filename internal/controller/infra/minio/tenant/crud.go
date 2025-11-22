package tenant

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "Tenant"
	ConfigTypeName   = "MinioConfig"
)

func CrudResource(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredCr *miniov2.Tenant,
) error {
	var err error
	var actual = &miniov2.Tenant{}

	if err = common.GetResource(
		ctx, client, TenantNamespacedName(specNamespacedName), ResourceTypeName, actual,
	); err != nil {
		return err
	}
	if err = common.CrudResource(ctx, client, desiredCr, actual); err != nil {
		return err
	}

	return nil
}

func CrudConfig(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredConfig *corev1.Secret,
) error {
	var err error
	var actual = &corev1.Secret{}

	if err = common.GetResource(
		ctx, client, ConfigNamespacedName(specNamespacedName), ConfigTypeName, actual,
	); err != nil {
		return err
	}
	if err = common.CrudResource(ctx, client, desiredConfig, actual); err != nil {
		return err
	}

	return nil
}
