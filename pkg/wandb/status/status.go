package status

import (
	"context"

	apiv1 "github.com/wandb/operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Status string

const (
	Initializing   Status = "Initializing"
	Downloading    Status = "Downloading"
	Installing     Status = "Install"
	Loading        Status = "Loading"
	Generating     Status = "Generating"
	Applying       Status = "Applying"
	InvalidConfig  Status = "Invalid Config"
	InvalidVersion Status = "Invalid Version"
	Completed      Status = "Completed"
)

func NewManager(
	ctx context.Context,
	client client.Client,
	wandb *apiv1.WeightsAndBiases,
) *Manager {
	return &Manager{ctx, client, wandb}
}

type Manager struct {
	ctx    context.Context
	client client.Client
	wandb  *apiv1.WeightsAndBiases
}

func (s *Manager) Set(status Status) error {
	s.wandb.Status.Phase = string(status)
	return s.client.Status().Update(s.ctx, s.wandb)
}
