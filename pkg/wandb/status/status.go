package status

import (
	"context"

	apiv1 "github.com/wandb/operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Status string

const (
	Loading       Status = "Loading"
	Applying      Status = "Applying"
	InvalidConfig Status = "Invalid Config"
	Completed     Status = "Completed"
	Deploying     Status = "Deploying"
	Success       Status = "Success"
	Failure       Status = "Failure"
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

// SetPhase updates the overall phase and optionally the message.
func (s *Manager) SetPhase(status Status, message string) error {
	s.wandb.Status.Phase = string(status)
	s.wandb.Status.Message = message
	return s.client.Status().Update(s.ctx, s.wandb)
}

// AddCondition adds or updates a condition for a specific resource.
func (s *Manager) AddCondition(condition metav1.Condition) error {
	conditions := s.wandb.Status.Conditions
	for i, c := range conditions {
		if c.Type == condition.Type {
			// Update existing condition
			conditions[i] = condition
			s.wandb.Status.Conditions = conditions
			return s.client.Status().Update(s.ctx, s.wandb)
		}
	}
	// Add new condition
	s.wandb.Status.Conditions = append(conditions, condition)
	return s.client.Status().Update(s.ctx, s.wandb)
}

// RemoveCondition removes a condition by type.
func (s *Manager) RemoveCondition(conditionType string) error {
	newConditions := []metav1.Condition{}
	for _, c := range s.wandb.Status.Conditions {
		if c.Type != conditionType {
			newConditions = append(newConditions, c)
		}
	}
	s.wandb.Status.Conditions = newConditions
	return s.client.Status().Update(s.ctx, s.wandb)
}
