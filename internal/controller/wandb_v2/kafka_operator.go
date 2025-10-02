package wandb_v2

import (
	"context"
	"errors"

	apiv2 "github.com/wandb/operator/api/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *WeightsAndBiasesV2Reconciler) handleKafkaOperator(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	if wandb.Spec.Streaming.Type != apiv2.WBKafkaStreaming {
		log.Info("Kafka Streaming handler skipping since not kafka", "type", wandb.Spec.Streaming.Type)
		return ctrl.Result{}, nil
	}
	if !wandb.Spec.Streaming.Enabled {
		log.Info("Kafka Streaming handler for enabled", "enabled", wandb.Spec.Streaming.Enabled)
		return ctrl.Result{}, nil
	}
	streamingStatus := wandb.Status.StreamingStatus
	log.Info("Streaming Reconcile", "reconcileStatus", streamingStatus.ReconciliationStatus)
	switch streamingStatus.ReconciliationStatus {
	case "":
		log.Info("handle empty reconcile status")
		wandb.Status.StreamingStatus.ReconciliationStatus = apiv2.WBInfraStatusMissing
		if err := r.Client.Update(ctx, wandb); err != nil {
			log.Error(err, "failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	case apiv2.WBInfraStatusMissing:
		log.Info("handle 'Missing' reconcile status")
		return ctrl.Result{}, nil
	}
	err := errors.New("not implemented")
	log.Error(err, "reconcile status handling not implemented for", "reconcileStatus", streamingStatus.ReconciliationStatus)
	return ctrl.Result{}, nil
}

func (r *WeightsAndBiasesV2Reconciler) installKafkaOperator(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) (ctrl.Result, error) {

	log := ctrllog.FromContext(ctx)
	log.Info("Installing Kafka Operator...")

	return ctrl.Result{}, nil
}
