package controller

import (
	"context"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/state"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesReconciler) loadUserSpec(ctx context.Context, manager *state.Manager) (*spec.Spec, error) {
	log := ctrllog.FromContext(ctx)
	if !r.UserSpecEnabled {
		log.Info("User spec is disabled")
		return nil, nil
	}

	userSpec, err := manager.GetUserInput()
	if err == nil {
		return userSpec, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	log.Info("No user input spec found, creating a new one")
	userSpec = &spec.Spec{Values: map[string]interface{}{}}
	if err := manager.SetUserInput(userSpec); err != nil {
		return nil, err
	}
	return userSpec, nil
}
