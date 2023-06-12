package controllers

import (
	"context"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesReconciler) applyConfig(
	ctx context.Context,
	wandb *apiv1.WeightsAndBiases,
	cfg *config.Config,
) error {
	log := ctrllog.FromContext(ctx)

	log.Info("Downloading release", "version", cfg.Release.Version())
	if err := cfg.Release.Download(); err != nil {
		log.Error(err, "Failed to download release")
		return err
	}

	log.Info("Install upgrade release", "version", cfg.Release.Version())
	if err := cfg.Release.Install(); err != nil {
		log.Error(err, "Failed to download release")
		return nil
	}

	log.Info("Generating upgrade", "version", cfg.Release.Version())
	if err := cfg.Release.Generate(cfg.Config); err != nil {
		log.Error(err, "Failed to download release")
		return nil
	}

	if err := cfg.Release.Apply(ctx, r.Client, wandb, r.Scheme); err != nil {
		log.Error(err, "Failed to apply config")
		return err
	}

	return nil
}
