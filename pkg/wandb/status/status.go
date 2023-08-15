package status

import (
	"context"

	apiv1 "github.com/wandb/operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Status string

const (
	Loading       Status = "Loading"
	Applying      Status = "Applying"
	InvalidConfig Status = "Invalid Config"
	Completed     Status = "Completed"
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
