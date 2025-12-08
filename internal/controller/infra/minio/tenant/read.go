package tenant

import (
	"context"

	ctrlcommon "github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
) ([]translator.MinioCondition, error) {
	var err error
	var results []translator.MinioCondition
	var actualResource = &miniov2.Tenant{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = ctrlcommon.GetResource(
		ctx, client, nsNameBldr.SpecNsName(), ResourceTypeName, actualResource,
	); err != nil {
		return results, err
	}

	if actualResource == nil {
		return results, nil
	}

	return results, nil
}
