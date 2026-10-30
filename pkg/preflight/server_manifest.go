package preflight

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
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
	shown := redactRepository(repo)

	ctx, cancel := context.WithTimeout(ctx, serverManifestTimeout)
	defer cancel()
	mfst, err := manifest.GetServerManifest(ctx, repo, version, env.RegistryAuth)
	if err != nil {
		return serverManifestFailure(repo, shown, version, err)
	}

	found := fmt.Sprintf("manifest %s found in %s", version, shown)
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
	if !constraint.Check(comparableOperatorVersion(operator, mfst.RequiredOperatorVersion)) {
		return fail(fmt.Sprintf("server %s requires operator %s, but the operator is %s",
			version, mfst.RequiredOperatorVersion, env.OperatorVersion))
	}
	return Result{Check: ServerManifestCheck, Outcome: OutcomePass,
		Message: fmt.Sprintf("%s; operator %s satisfies %s", found, env.OperatorVersion, mfst.RequiredOperatorVersion)}
}

// constraintPrerelease spots a prerelease bound such as ">=2.0.0-rc.5": a digit immediately followed by "-".
// Hyphen ranges ("1.2 - 1.4") have spaces around the dash and don't match.
var constraintPrerelease = regexp.MustCompile(`\d-[0-9A-Za-z]`)

// comparableOperatorVersion drops the operator's prerelease only for a constraint without prerelease bounds. Under
// semver a prerelease never satisfies such a constraint, yet manifests pin "^2.0.0" for the 2.0.0 betas. A constraint
// that names a prerelease means to order prereleases, so the original version is compared.
func comparableOperatorVersion(operator *semver.Version, constraint string) *semver.Version {
	if operator.Prerelease() == "" || constraintPrerelease.MatchString(constraint) {
		return operator
	}
	core, _ := operator.SetPrerelease("")
	return &core
}

// redactRepository drops userinfo, which a manifestRepository can embed, so results never carry credentials.
func redactRepository(repo string) string {
	u, err := url.Parse(repo)
	if err != nil {
		return "the configured manifest repository"
	}
	u.User = nil
	return u.String()
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
func serverManifestFailure(repo, shown, version string, err error) Result {
	var regErr *errcode.ErrorResponse
	switch {
	case errors.Is(err, errdef.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return Result{Check: ServerManifestCheck, Outcome: OutcomeFail,
			Message: fmt.Sprintf("server version %s is not published in %s", version, shown)}
	case errors.As(err, &regErr) && (regErr.StatusCode == http.StatusUnauthorized || regErr.StatusCode == http.StatusForbidden):
		return Result{Check: ServerManifestCheck, Outcome: OutcomeUnknown,
			Message: fmt.Sprintf("%s requires registry credentials this check doesn't have", shown)}
	default:
		return Result{Check: ServerManifestCheck, Outcome: OutcomeUnknown,
			Message: fmt.Sprintf("load manifest %s from %s: %s", version, shown, redactRepositoryError(err, repo, shown))}
	}
}

// redactRepositoryError keeps credentials from a repository URL out of an error that may quote it verbatim.
func redactRepositoryError(err error, repo, shown string) string {
	msg := strings.ReplaceAll(err.Error(), repo, shown)
	msg = strings.ReplaceAll(msg, strings.TrimPrefix(repo, "oci://"), strings.TrimPrefix(shown, "oci://"))
	if u, perr := url.Parse(repo); perr == nil && u.User != nil {
		if pw, ok := u.User.Password(); ok {
			msg = redact(msg, pw)
		}
		msg = redact(msg, u.User.Username())
	}
	return msg
}
