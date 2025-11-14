package tenant

import (
	"context"
	"fmt"
	"strconv"

	miniov2 "github.com/wandb/operator/api/minio-operator-vendored/minio.min.io/v2"
	"github.com/wandb/operator/internal/model"
)

func (a *minioTenant) updateTenant(
	ctx context.Context, desiredTenant *miniov2.Tenant, minioConfig model.MinioConfig,
) *model.Results {
	results := model.InitResults()

	// Extract connection info from Tenant CR
	// Connection format: wandb-minio-hl.{namespace}.svc.cluster.local:443
	namespace := a.tenant.Namespace
	minioHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, namespace)
	minioPort := strconv.Itoa(MinioPort)

	connInfo := model.MinioConnInfo{
		Host:      minioHost,
		Port:      minioPort,
		AccessKey: MinioAccessKey,
	}
	results.AddStatuses(model.NewMinioConnDetail(connInfo))

	return results
}
