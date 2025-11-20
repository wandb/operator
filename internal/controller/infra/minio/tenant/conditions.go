package tenant

import (
	"context"
	"fmt"
	"strconv"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConditions(
	ctx context.Context,
	client client.Client,
	namespacedName types.NamespacedName,
) ([]common.InfraStatusDetail, error) {
	var err error
	var results []common.InfraStatusDetail
	var actual = &miniov2.Tenant{}

	if err = ctrlcommon.GetResource(
		ctx, client, namespacedName, ResourceTypeName, actual,
	); err != nil {
		return results, err
	}

	if actual == nil {
		return results, nil
	}

	// Extract connection info from Tenant CR
	// Connection format: wandb-minio-hl.{namespace}.svc.cluster.local:443
	minioHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, namespacedName.Namespace)
	minioPort := strconv.Itoa(MinioPort)

	connInfo := common.MinioConnInfo{
		Host:      minioHost,
		Port:      minioPort,
		AccessKey: MinioAccessKey,
	}
	results = append(results, common.NewMinioConnDetail(connInfo))
	///////////

	return results, nil
}
