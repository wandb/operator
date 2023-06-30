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
	rel release.Release,
	cfg map[string]interface{},
) error {
	log := ctrllog.FromContext(ctx)

	log.Info("Downloading release", "version", rel.Version())
	if err := rel.Download(); err != nil {
		log.Error(err, "Failed to download release")
		return err
	}

	log.Info("Install upgrade release", "version", rel.Version())
	if err := rel.Install(); err != nil {
		log.Error(err, "Failed to install release")
		return err
	}

	log.Info("Generating upgrade", "version", rel.Version())
	if err := rel.Generate(cfg); err != nil {
		log.Error(err, "Failed to generate release")
		return err
	}

	if err := rel.Apply(ctx, r.Client, wandb, r.Scheme); err != nil {
		log.Error(err, "Failed to apply config")
		return err
	}

	return nil
}
