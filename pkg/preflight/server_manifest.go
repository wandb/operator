package preflight

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

const serverManifestTimeout = 60 * time.Second

// RunServerManifestCheck loads the manifest for spec.wandb.version from the repository the webhook would default
// to. Reconcile loads it before anything else, so an unpublished version otherwise only fails after apply.
func RunServerManifestCheck(ctx context.Context, spec *apiv2.WeightsAndBiasesSpec, env Env) Result {
	fail := func(msg string) Result {
		return Result{Check: ServerManifestCheck, Outcome: OutcomeFail, Message: msg}
	}
	version := strings.TrimSpace(spec.Wandb.Version)
	if version == "" || version == "latest" {
		return fail("spec.wandb.version must be pinned to a published server version")
	}
	repo := effectiveManifestRepository(spec)

	ctx, cancel := context.WithTimeout(ctx, serverManifestTimeout)
	defer cancel()
	mfst, err := manifest.GetServerManifest(ctx, repo, version, env.RegistryAuth)
	if err != nil {
		return serverManifestFailure(repo, version, err)
	}

	found := fmt.Sprintf("manifest %s found in %s", version, repo)
	if env.OperatorVersion == "" || mfst.RequiredOperatorVersion == "" {
		return Result{Check: ServerManifestCheck, Outcome: OutcomePass, Message: found}
	}
	constraint, err := semver.NewConstraint(mfst.RequiredOperatorVersion)
	if err != nil {
		return Result{Check: ServerManifestCheck, Outcome: OutcomeUnknown,
			Message: fmt.Sprintf("manifest's requiredOperatorVersion %q is not a semver constraint", mfst.RequiredOperatorVersion)}
	}
	operator, err := semver.NewVersion(env.OperatorVersion)
	if err != nil {
		return Result{Check: ServerManifestCheck, Outcome: OutcomeUnknown,
			Message: fmt.Sprintf("operator version %q is not semver", env.OperatorVersion)}
	}
	// Prereleases never satisfy a constraint without one in semver, yet manifests pin "^2.0.0" for the 2.0.0 betas.
	core, _ := operator.SetPrerelease("")
	if !constraint.Check(&core) {
		return fail(fmt.Sprintf("server %s requires operator %s, but the operator is %s",
			version, mfst.RequiredOperatorVersion, env.OperatorVersion))
	}
	return Result{Check: ServerManifestCheck, Outcome: OutcomePass,
		Message: fmt.Sprintf("%s; operator %s satisfies %s", found, env.OperatorVersion, mfst.RequiredOperatorVersion)}
}

// effectiveManifestRepository mirrors the defaulting webhook.
func effectiveManifestRepository(spec *apiv2.WeightsAndBiasesSpec) string {
	repo := spec.Wandb.ManifestRepository
	if repo == "" {
		repo = apiv2.ManifestRepositoryFor(spec.Global.ImageRegistry)
	}
	if !strings.Contains(repo, "://") {
		repo = "oci://" + repo
	}
	return repo
}

// serverManifestFailure separates a version that doesn't exist from a registry that wouldn't let the caller look.
func serverManifestFailure(repo, version string, err error) Result {
	var regErr *errcode.ErrorResponse
	switch {
	case errors.Is(err, errdef.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return Result{Check: ServerManifestCheck, Outcome: OutcomeFail,
			Message: fmt.Sprintf("server version %s is not published in %s", version, repo)}
	case errors.As(err, &regErr) && (regErr.StatusCode == http.StatusUnauthorized || regErr.StatusCode == http.StatusForbidden):
		return Result{Check: ServerManifestCheck, Outcome: OutcomeUnknown,
			Message: fmt.Sprintf("%s requires registry credentials this check doesn't have", repo)}
	default:
		return Result{Check: ServerManifestCheck, Outcome: OutcomeUnknown,
			Message: fmt.Sprintf("load manifest %s from %s: %v", version, repo, err)}
	}
}
