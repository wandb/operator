package controllers

import (
	"context"
	"fmt"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesReconciler) getWantedRelease(
	ctx context.Context,
	wandb *apiv1.WeightsAndBiases,
) (release.Release, error) {
	log := ctrllog.FromContext(ctx)

	log.Info("Getting wanted release version...")
	releaseManager := release.NewManager(ctx, wandb)
	wantedRelease, err := releaseManager.GetLatestSupportedRelease()
	if err != nil {
		wantedRelease, _ = releaseManager.GetLatestDownloadedRelease()
	}

	if wandb.Spec.Cdk8sVersion != "" {
		wantedRelease = releaseManager.GetSpecRelease()
		if wantedRelease == nil {
			log.Error(err, "Failed to find any release with version %s.", wandb.Spec.Cdk8sVersion)
			return nil, err
		}
	}

	if wantedRelease == nil {
		err = fmt.Errorf("no release found")
		log.Error(err, "No release found.")
		return nil, err
	}

	if wantedRelease == nil {
		err = fmt.Errorf("no release found")
		log.Error(err, "No release found.")
		return nil, err
	}

	return wantedRelease, nil
}
