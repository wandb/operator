package v2

import (
	"context"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator/common"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func minioWriteState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
) (*common.MinioConnection, error) {
	var err error
	var desiredCr *miniov2.Tenant
	var desiredConfig tenant.MinioEnvConfig
	var specNamespacedName = minioSpecNamespacedName(wandb.Spec.Minio)
	var connection *common.MinioConnection

	if desiredCr, err = translatorv2.ToMinioVendorSpec(ctx, wandb.Spec.Minio, wandb, client.Scheme()); err != nil {
		return nil, err
	}
	if desiredConfig, err = translatorv2.ToMinioEnvConfig(ctx, wandb.Spec.Minio); err != nil {
		return nil, err
	}
	if connection, err = tenant.WriteState(ctx, client, specNamespacedName, desiredCr, desiredConfig, wandb); err != nil {
		return nil, err
	}

	return connection, nil
}

func minioReadState(
	ctx context.Context,
	client client.Client,
	wandb *apiv2.WeightsAndBiases,
	connection *common.MinioConnection,
) error {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var conditions []common.MinioCondition
	var specNamespacedName = minioSpecNamespacedName(wandb.Spec.Minio)

	if conditions, err = tenant.ReadState(ctx, client, specNamespacedName); err != nil {
		return err
	}
	if connection != nil {
		conditions = append(conditions, common.NewMinioConnCondition(*connection))
	}
	wandb.Status.MinioStatus = translatorv2.ExtractMinioStatus(ctx, conditions)
	if err = client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "failed to update status")
		return err
	}

	return nil
}

func minioSpecNamespacedName(minio apiv2.WBMinioSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: minio.Namespace,
		Name:      minio.Name,
	}
}
