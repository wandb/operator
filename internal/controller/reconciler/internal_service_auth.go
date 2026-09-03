package reconciler

import (
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
)

// serviceAccountIssuerUnknownReason marks a CR that can't be reconciled because
// the cluster's service-account issuer is neither configured nor discoverable.
const serviceAccountIssuerUnknownReason = "ServiceAccountIssuerUnknown"

const serviceAccountIssuerUnknownMessage = "could not determine the cluster service-account issuer: " +
	"set spec.wandb.internalServiceAuth.oidcIssuer to the value of " +
	"`kubectl get --raw /.well-known/openid-configuration`, or grant the operator get on that URL " +
	"and restart it"

// internalServiceAuthEnabled reports whether W&B services validate each other's
// projected ServiceAccount tokens.
func internalServiceAuthEnabled(wandb *v2.WeightsAndBiases) bool {
	return wandb.Spec.Wandb.InternalServiceAuth.Enabled != nil &&
		*wandb.Spec.Wandb.InternalServiceAuth.Enabled
}

// resolveInternalServiceAuthIssuer returns the issuer W&B services must validate
// projected ServiceAccount tokens against: the explicit CR value when set, else
// the issuer discovered from the cluster at start-up.
//
// Empty means unknown, and callers must not substitute a guess. The API server
// stamps its own --service-account-issuer as the token's `iss`, so any other
// value fails validation — which surfaces as a 401 and panics the API rather
// than degrading.
func resolveInternalServiceAuthIssuer(wandb *v2.WeightsAndBiases) string {
	if wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer != "" {
		return wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer
	}
	return utils.ServiceAccountIssuer()
}
