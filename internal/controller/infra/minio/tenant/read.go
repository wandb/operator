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
	connection *translator.InfraConnection,
) (*translator.MinioStatus, error) {
	var err error
	var found bool
	var status = &translator.MinioStatus{}
	var actualResource = &miniov2.Tenant{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if found, err = ctrlcommon.GetResource(
		ctx, client, nsnBuilder.SpecNsName(), ResourceTypeName, actualResource,
	); err != nil {
		return nil, err
	}
	if !found {
		actualResource = nil
	}

	///////////////////////////////////
	// set connection details

	if connection != nil {
		status.Connection = *connection
	}

	///////////////////////////////////
	// add conditions

	///////////////////////////////////
	// set top-level summary
	computeStatusSummary(ctx, actualResource, status)

	return status, nil
}

func computeStatusSummary(_ context.Context, tenantCR *miniov2.Tenant, status *translator.MinioStatus) {
	if tenantCR != nil {
		switch tenantCR.Status.HealthStatus {
		case miniov2.HealthStatusGreen:
			status.State = "Ready"
			status.Ready = true
		case miniov2.HealthStatusRed:
			status.State = "Error"
			status.Ready = false
		case miniov2.HealthStatusYellow:
			status.State = "Degraded"
			status.Ready = true
		default:
			status.State = "NotReady"
			status.Ready = false
		}
	} else {
		status.State = "Not Installed"
		status.Ready = false
	}
}
