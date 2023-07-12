package controllers

import (
	"context"
	"fmt"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	"github.com/wandb/operator/pkg/wandb/status"
	corev1 "k8s.io/api/core/v1"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesReconciler) applyConfig(
	ctx context.Context,
	wandb *apiv1.WeightsAndBiases,
	statusManager *status.Manager,
	rel release.Release,
	cfg map[string]interface{},
) error {
	log := ctrllog.FromContext(ctx)

	statusManager.Set(status.Downloading)
	r.Recorder.Event(wandb, corev1.EventTypeNormal, "Downloading", "Downloading "+rel.Version())
	log.Info("Downloading release", "version", rel.Version())
	if err := rel.Download(); err != nil {
		log.Error(err, "Failed to download release")
		return err
	}

	statusManager.Set(status.Installing)
	r.Recorder.Event(wandb, corev1.EventTypeNormal, "Installing", "Installing "+rel.Version())
	log.Info("Install upgrade release", "version", rel.Version())
	if err := rel.Install(); err != nil {
		fmt.Println(err)
		log.Error(err, "Failed to install release")
		return err
	}

	statusManager.Set(status.Generating)
	r.Recorder.Event(wandb, corev1.EventTypeNormal, "Generating", "Generating manifests "+rel.Version())
	log.Info("Generating upgrade", "version", rel.Version())
	if err := rel.Generate(cfg); err != nil {
		fmt.Println(err)
		log.Error(err, "Failed to generate release")
		return err
	}

	statusManager.Set(status.Applying)
	r.Recorder.Event(wandb, corev1.EventTypeNormal, "Applying", "Applying manifests "+rel.Version())
	if err := rel.Apply(ctx, r.Client, wandb, r.Scheme); err != nil {
		fmt.Println(err)
		log.Error(err, "Failed to apply config")
		return err
	}

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "AppliedSuccessfully", "Successfully applied "+rel.Version())

	return nil
}
