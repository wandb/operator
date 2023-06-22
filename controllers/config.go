package controllers

import (
	"context"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesReconciler) applyConfig(
	ctx context.Context,
	wandb *apiv1.WeightsAndBiases,
	release release.Release,
	cfg map[string]interface{},
) error {
	log := ctrllog.FromContext(ctx)

	log.Info("Downloading release", "version", release.Version())
	if err := release.Download(); err != nil {
		log.Error(err, "Failed to download release")
		return err
	}

	log.Info("Install upgrade release", "version", release.Version())
	if err := release.Install(); err != nil {
		log.Error(err, "Failed to download release")
		return nil
	}

	log.Info("Generating upgrade", "version", release.Version())
	if err := release.Generate(cfg); err != nil {
		log.Error(err, "Failed to download release")
		return nil
	}

	if err := release.Apply(ctx, r.Client, wandb, r.Scheme); err != nil {
		log.Error(err, "Failed to apply config")
		return err
	}

	return nil
}
