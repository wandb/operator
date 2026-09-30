package common

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentResource is a Kubernetes resource with shared deployment settings
// and status. Implementations return pointers to their embedded fields so status
// updates are persisted on the same object used for ownership and events.
type DeploymentResource interface {
	client.Object
	GetBaseDeploymentSpec() *apiv2.BaseDeploymentSpec
	GetBaseDeploymentStatus() *apiv2.BaseDeploymentStatus
}

var _ DeploymentResource = (*apiv2.WeightsAndBiases)(nil)
