package reconciler

import (
	"context"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/preflight"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// runPreflightOnce reports whether the check is skipped, already passed for this generation, or passes now.
func runPreflightOnce(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	check preflight.Check,
	fieldPath string,
	params map[string]string,
) (bool, string, error) {
	if preflight.ParseSkipList(wandb.GetAnnotations()[preflight.SkipPreflightsAnnotation])[check.Name] {
		return true, "", nil
	}

	key := check.Name + "/" + fieldPath
	if prev, ok := wandb.Status.Preflights[key]; ok &&
		prev.ObservedGeneration == wandb.Generation &&
		prev.Outcome == string(preflight.OutcomePass) {
		return true, "", nil
	}

	statusBefore := wandb.DeepCopy().Status
	logger := ctrl.LoggerFrom(ctx).WithValues("check", check.Name, "field", fieldPath)
	logger.Info("Running preflight check")
	result := check.Run(ctx, params)
	logger.Info("Preflight check finished", "outcome", result.Outcome, "message", result.Message)
	if wandb.Status.Preflights == nil {
		wandb.Status.Preflights = map[string]apiv2.PreflightStatus{}
	}
	wandb.Status.Preflights[key] = apiv2.PreflightStatus{
		Outcome:            string(result.Outcome),
		Message:            result.Message,
		ObservedGeneration: wandb.Generation,
	}

	if err := updateWandbStatusIfChanged(ctx, c, wandb, statusBefore); err != nil {
		return false, "", err
	}
	return result.Outcome == preflight.OutcomePass, result.Message, nil
}
