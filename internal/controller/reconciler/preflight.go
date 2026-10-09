package reconciler

import (
	"context"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external"
	"github.com/wandb/operator/pkg/preflight"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func preflightSkipped(wandb *apiv2.WeightsAndBiases, name string) bool {
	return preflight.ParseSkipList(wandb.GetAnnotations()[preflight.SkipPreflightsAnnotation])[name]
}

// runPreflightOnce reports whether the check is skipped, already passed for this generation and Secret inputs, or passes now.
func runPreflightOnce(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	check preflight.Check,
	fieldPath string,
	spec any,
	inputVersion string,
) (bool, string, error) {
	if preflightSkipped(wandb, check.Name) {
		return true, "", nil
	}

	key := check.Name + "/" + fieldPath
	if prev, ok := wandb.Status.Preflights[key]; ok &&
		prev.ObservedGeneration == wandb.Generation &&
		prev.InputVersion == inputVersion &&
		prev.Outcome == string(preflight.OutcomePass) {
		return true, "", nil
	}

	statusBefore := wandb.DeepCopy().Status
	logger := ctrl.LoggerFrom(ctx).WithValues("check", check.Name, "field", fieldPath)
	logger.Info("Running preflight check")
	resolve := func(ctx context.Context, v apiv2.ValueOrSecret) (string, error) {
		return external.ResolveValue(ctx, c, wandb.Namespace, v)
	}
	result := check.Run(ctx, spec, resolve)
	logger.Info("Preflight check finished", "outcome", result.Outcome, "message", result.Message)
	if wandb.Status.Preflights == nil {
		wandb.Status.Preflights = map[string]apiv2.PreflightStatus{}
	}
	wandb.Status.Preflights[key] = apiv2.PreflightStatus{
		Outcome:            string(result.Outcome),
		Message:            result.Message,
		ObservedGeneration: wandb.Generation,
		InputVersion:       inputVersion,
	}

	if err := updateWandbStatusIfChanged(ctx, c, wandb, statusBefore); err != nil {
		return false, "", err
	}
	return result.Outcome == preflight.OutcomePass, result.Message, nil
}

// secretInputVersion fingerprints referenced Secrets so a rotation invalidates a cached pass without
// storing credential values in status.
func secretInputVersion(ctx context.Context, c client.Client, namespace string, fields ...apiv2.ValueOrSecret) (string, error) {
	var parts []string
	seen := map[string]bool{}
	for _, f := range fields {
		ref := f.SecretKeyRef()
		if ref == nil || ref.Name == "" || seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		s := &corev1.Secret{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, s); err != nil {
			return "", err
		}
		parts = append(parts, ref.Name+"@"+s.ResourceVersion)
	}
	return strings.Join(parts, ","), nil
}
