package tenant

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/model"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (a *minioTenant) createSecret(
	ctx context.Context, desiredSecret *corev1.Secret,
) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	if a.configSecret != nil {
		return results
	}

	if err := a.client.Create(ctx, desiredSecret); err != nil {
		log.Error(err, "Failed to create config secret")
		results.AddErrors(model.NewMinioError(
			model.MinioErrFailedToCreateCode,
			fmt.Sprintf("failed to create config secret: %v", err),
		))
		return results
	}

	log.Info("Created Minio config secret", "secret", desiredSecret.Name)
	return results
}

func (a *minioTenant) createTenant(
	ctx context.Context, desiredTenant *miniov2.Tenant,
) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	if a.tenant != nil {
		msg := "cannot create Tenant CR when it already exists"
		err := model.NewMinioError(model.MinioErrFailedToCreateCode, msg)
		log.Error(err, msg)
		results.AddErrors(err)
		return results
	}

	if err := a.client.Create(ctx, desiredTenant); err != nil {
		log.Error(err, "Failed to create Tenant CR")
		results.AddErrors(model.NewMinioError(
			model.MinioErrFailedToCreateCode,
			fmt.Sprintf("failed to create Tenant CR: %v", err),
		))
		return results
	}

	results.AddStatuses(
		model.NewMinioStatusDetail(model.MinioCreatedCode, fmt.Sprintf("Created Tenant CR: %s", TenantName)),
	)

	return results
}
