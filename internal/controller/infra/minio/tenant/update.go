package tenant

import (
	"context"
	"fmt"
	"strconv"

	"github.com/wandb/operator/internal/controller/translator/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
)

func (a *minioTenant) updateTenant(
	ctx context.Context, desiredTenant *miniov2.Tenant, minioConfig common.MinioConfig,
) *common.Results {
	results := common.InitResults()

	// Extract connection info from Tenant CR
	// Connection format: wandb-minio-hl.{namespace}.svc.cluster.local:443
	namespace := a.tenant.Namespace
	minioHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, namespace)
	minioPort := strconv.Itoa(MinioPort)

	connInfo := common.MinioConnInfo{
		Host:      minioHost,
		Port:      minioPort,
		AccessKey: MinioAccessKey,
	}
	results.AddStatuses(common.NewMinioConnDetail(connInfo))

	return results
}
