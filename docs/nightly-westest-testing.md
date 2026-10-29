# Nightly WESTest testing

[The nightly workflow](../.github/workflows/nightly.yaml) builds an operator image
and chart, then tests that chart with the published WESTest container. Manual runs
can instead select an existing chart version and skip the build. The daily 07:00
UTC schedule remains commented out until the first cloud runs and cleanup have
been validated.

## Scenarios and branch selection

| Scenario | Where it runs | GitHub environment |
| --- | --- | --- |
| `local-kind-ingress` | Any dispatched branch | None |
| `local-kind-gateway` | Any dispatched branch | None |
| `local-kind-oidc-pkce` | Any dispatched branch | None |
| `local-kind-oidc-implicit` | Any dispatched branch | None |
| `local-kind-proxy` | Any dispatched branch | None |
| `local-kind-external` | Any dispatched branch | None |
| `local-kind-proxy-auth` | Any dispatched branch | None |
| `local-kind-proxy-tls` | Any dispatched branch | None |
| `local-kind-operator-upgrade` | Any dispatched branch | None |
| `gcp-gke-gcs-hmac` | `refs/heads/main` only | `westest-gcp` |
| `gcp-gke-gcs-byob` | `refs/heads/main` only | `westest-gcp` |
| `gcp-gke-provider-services` | `refs/heads/main` only | `westest-gcp` |
| `gcp-gke-gcs-letsencrypt` | `refs/heads/main` only | `westest-gcp` |
| `gcp-gke-ingress-managed-cert` | `refs/heads/main` only | `westest-gcp` |

An empty `scenario` input selects all nine local scenarios and, on main, all five GCP
cloud scenarios. A named scenario selects only that scenario. Unsupported names
and attempts to select a cloud scenario on another ref fail during setup. There
is no pull-request trigger for these provisioning tests.

The local matrix covers external datastores, authenticated proxy egress, and
TLS-intercepting proxy CA trust alongside the baseline routing and OIDC checks.
`local-kind-app-test-data` is not yet enabled for nightly use.

`gcp-gke-gateway-managed-cert-regional` and `gcp-gke-gateway-managed-cert-global`
are temporarily disabled, including explicit dispatches, until Core's WESTest CI
permissions allow the ExternalDNS project-level `roles/dns.reader` grant. Re-enable
both after that permission change is merged and applied. The managed-certificate
Ingress scenario remains enabled.

AWS scenarios remain disabled. The legacy `gcp-gke-ingress-in-cluster-services`
and `gcp-gke-gateway-in-cluster-services` scaffolds are excluded because they do
not run end-to-end validators; the Gateway scaffold also does not deploy W&B.

Local tests use `ubuntu-latest-8-cores`, with up to five scenarios in parallel.
Their Kind clusters need the additional CPU. Cloud tests use ordinary
`ubuntu-latest` runners, with up to two scenarios in parallel; Kubernetes runs in
the cloud account. A failed scenario does not cancel its siblings. Runs on the
same ref are serialized, and a new dispatch does not cancel an active run.

## Container and chart versions

The action hardcodes the public WESTest image repository. The workflow selects
only the tag through `WESTEST_IMAGE_TAG`, currently `0.5.1`:

```text
us-docker.pkg.dev/wandb-production/public/westest/westest:0.5.1
```

To test a different WESTest release, change `WESTEST_IMAGE_TAG` in the workflow.
The action's `image-tag` input accepts Docker tags only; full image references,
repository paths and digest overrides are rejected before credentials are prepared.

The image contains WESTest, its scenarios and Terraform fixtures, Terraform,
Helm, kubectl, Kind, Docker CLI, the AWS and Google Cloud CLIs, the GKE auth plugin,
and the W&B Python CLI. Tests no longer download a release binary or install
these tools on the host. The image is publicly pullable without a GitHub App
installation token.

The host still needs Docker CLI and GNU coreutils `timeout`. The wrapper checks
both before preparing credentials or starting containers and reports a clear
dependency error if either is missing.

The shared [Docker action](../.github/actions/westest-run/action.yml) mounts a
fresh runner-local work directory at `/work`. Local tests also mount the Docker
socket and use host networking so WESTest can reach Kind's API and application
ports. The image entrypoint configures scenario hostnames in its own `/etc/hosts`.
Cloud tests mount their credentials but do not mount the Docker socket or use
host networking.

For ordinary scenarios, both `westest config` and `westest run` receive
`--param operatorVersion=<chart version>`. The operator-upgrade scenario instead
receives `--param fromOperatorVersion=<baseline>` and
`--param toOperatorVersion=<chart version>`, so only its second phase installs
the chart under test. These parameters override scenario phase defaults;
setting only `WESTEST_OPERATOR_CHART_VERSION` would leave scenario-pinned chart
versions in effect.

The upgrade baseline is pinned by `OPERATOR_UPGRADE_FROM_VERSION` in the workflow,
currently `2.0.0-beta.8`. Manual runs can override it with `upgrade_from_version`.
The destination always uses the resolved `chart_version`: either the new build
or the supplied `operator_version`. Identical starting and destination versions
are rejected before starting Docker. The application version remains fixed across
both phases. Advance the baseline through a reviewed change, eventually selecting
a supported stable v2 release; it is not resolved from a moving latest tag.

For example, to test a published beta-to-beta upgrade:

```bash
gh workflow run nightly.yaml --repo wandb/operator --ref main \
  -f scenario=local-kind-operator-upgrade \
  -f upgrade_from_version=2.0.0-beta.8 -f operator_version=2.0.0-beta.9
```

The reusable [nightly build](../.github/workflows/nightly-build.yaml) is unchanged:

- Scheduled runs use `<base>-nightly-<UTC timestamp>`.
- Manual builds default to `<base>-dev-[<dev_tag>-]<UTC timestamp>`. The optional
  label is sanitized, and the `channel` input can select `nightly` instead.
- The image tag and chart version are identical. The image is published under
  `us-docker.pkg.dev/wandb-production/public/wandb/operator`, and the chart under
  `oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator`.
- Setting `operator_version` skips the build and tests that published chart.

## Application versions

Operator tests use already released application versions. Setup anonymously lists
tags in `us-docker.pkg.dev/wandb-production/public/wandb/server-manifest` and chooses
the highest semantic version matching exactly `X.Y.Z`. RC, beta, daily, and other
prerelease application builds are not eligible; their validation belongs in Core.
The optional `app_version` dispatch input selects an exact `X.Y.Z` for reproduction,
but that version must also have a published stable server manifest.

Core publishes the stable manifest after its release job succeeds, so the public
manifest is the publication signal. A release becomes eligible when its stable
manifest tag appears. Setup reads the selected manifest and verifies its SHA-256
digest against the registry response before any scenario starts. Registry errors,
missing selected manifests, and digest mismatches fail setup without falling back
to another version.

The resulting version is resolved once and passed as `--param appVersion=<version>`
to both `westest config` and `westest run` for every local and cloud scenario.
The operator-upgrade scenario applies that same version to both phases, holding
the application constant while the operator changes. The setup summary and each
scenario's public summary record the application version and manifest
digest resolved at setup. A fresh workflow run resolves the latest release again;
use `app_version` to reproduce an earlier run's selected version:

```bash
gh workflow run nightly.yaml --repo wandb/operator --ref main \
  -f scenario=local-kind-operator-upgrade \
  -f upgrade_from_version=2.0.0-beta.8 -f operator_version=2.0.0-beta.9 \
  -f app_version=0.85.1
```

Application selection requires no GitHub token, registry credentials, or Core
repository access. It uses the public Registry V2 API directly. The manifest digest
provides the recorded artifact identity; setup does not look up the Core commit.

## Cloud authentication

Only the cloud test job requests `id-token: write`. Each matrix entry selects its
GitHub environment before requesting credentials. Environments provide a place
for deployment branch restrictions, reviewers and other protections; cloud trust
also independently checks the main branch.

### AWS

AWS is currently disabled in the scenario matrix. The authentication setup below
is retained for when AWS scenarios are re-enabled.

`aws-actions/configure-aws-credentials` assumes
`arn:aws:iam::770934259321:role/WestestGitHubActions` through GitHub OIDC, requesting
a four-hour session. The AWS trust policy requires the `westest-aws` environment
subject, the STS audience, and (after the prerequisite change is applied) the
`refs/heads/main` ref claim.

The Docker action writes the temporary access key, secret and session token to a
mode-0600 credentials file under the `Sandbox` profile and mounts it read-only at
`/root/.aws`. WESTest explicitly uses this profile. Credentials are not supplied
as Docker command-line arguments. The action sets:

```text
WESTEST_CLOUD_OPT_IN=1
WESTEST_AWS_PROFILE=Sandbox
WESTEST_AWS_REGION=us-west-2
WESTEST_AWS_IAM_PATH=/westest/
WESTEST_AWS_ROLE_PERMISSIONS_BOUNDARY_ARN=arn:aws:iam::770934259321:policy/WestestWorkloadBoundary
```

The initial IRSA scenario creates workload roles, not an IAM user. It does not
need a user permissions boundary or a second role-assumption hop.

### GCP

`google-github-actions/auth` uses the provider
`projects/27586067691/locations/global/workloadIdentityPools/westest-github/providers/github`
to impersonate `westest-ci@playground-111.iam.gserviceaccount.com`. The provider
checks the repository identity, main ref, `westest-gcp` environment, and allowed
event type.

The action's external-account credential file retains its refreshable GitHub
OIDC credential source and service-account impersonation configuration. The
Docker action copies it to a private, read-only mount and sets both
`GOOGLE_APPLICATION_CREDENTIALS` and `CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE` to
the container path. This supports Terraform, gcloud and GKE authentication
throughout provisioning and teardown without passing a short-lived standalone
access token. Tests target `playground-111` in `us-west1` with
`WESTEST_CLOUD_OPT_IN=1`.

The managed-certificate Ingress and the temporarily disabled Gateway scenarios
use WESTest's default Cloud DNS zone `westest-gcp` (`westest-gcp.wandb.ml`) in
`playground-111`. That zone must be publicly delegated. The CI identity needs
the existing DNS and Certificate Manager permissions, and the provider-services
scenario also needs Cloud SQL and Memorystore permissions. These are declared in
[core's WESTest provisioning permissions](https://github.com/wandb/core/blob/master/terraform/terraform-workspaces/modules/westest-ci/provisioning-permissions.md).
No additional workflow secrets or DNS inputs are required for the default zone.

`WESTEST_LICENSE` is an optional repository/environment secret if a deployment
requires a license; when present it is mounted as a private file. This workflow
adds no required repository secret for cleanup recovery.

## First-run prerequisites and validation

1. Merge and apply [core PR #59074](https://github.com/wandb/core/pull/59074), which
   adds the independent AWS main-branch condition while keeping the environments.
   The earlier WESTest OIDC and EBS/KMS permission changes must also be applied.
2. Have an operator repository administrator configure `westest-aws` and
   `westest-gcp`, restrict deployments to main, and add any desired reviewers.
   Merely naming an environment in YAML does not configure these protections.
3. Confirm `ubuntu-latest-8-cores` is available. For build mode, retain
   `CI_WORKLOAD_IDENTITY_PROVIDER`, `CI_WORKLOAD_IDENTITY_SERVICE_ACCOUNT`,
   `WESTEST_APP_ID`, and `WESTEST_APP_PRIVATE_KEY`. The App credentials are used
   by the build to fetch watchtower; setup and scenario jobs do not need them.
4. Merge the workflow, then dispatch the GCP scenarios on main against a known
   published chart. Confirm OIDC login, provisioning, validators and teardown,
   including Cloud SQL/Memorystore and the managed-certificate Ingress path.
   Validate both Gateway paths after their permissions are applied and they are re-enabled.
   These main-branch runs are the end-to-end authentication check;
   local tests cannot mint the same GitHub identity.
5. Check the cloud accounts for leftover resources, including the known etcd PVC
   cleanup issue, before enabling the cron.

For example, substitute a published chart for `$CHART_VERSION`:

```bash
gh workflow run nightly.yaml --repo wandb/operator --ref main \
  -f scenario=gcp-gke-gcs-hmac -f operator_version="$CHART_VERSION"
gh workflow run nightly.yaml --repo wandb/operator --ref main \
  -f scenario=gcp-gke-ingress-managed-cert -f operator_version="$CHART_VERSION"
```

To test a branch locally in Kind through Actions, dispatch that branch with a
local scenario. Omit `operator_version` to build the selected branch:

```bash
gh workflow run nightly.yaml --repo wandb/operator --ref my-branch \
  -f scenario=local-kind-ingress -f dev_tag=my-change
```

## Timeouts, results and cleanup follow-ups

Local runs have a 60-minute execution budget, followed by up to 11 minutes for
WESTest's 10-minute cancellation cleanup. The operator-upgrade scenario gets
90 minutes for its two deployment phases, with the same 11-minute cleanup grace.
Cloud runs have a 75-minute execution budget and 46 minutes for WESTest's
45-minute cancellation cleanup. The wrapper
sends SIGINT first, and preserves the timeout's nonzero exit status. Job limits
are 90 minutes for ordinary local scenarios, 120 minutes for the operator upgrade,
and 180 minutes for cloud scenarios, leaving time for setup and finalization.
Normal success and failure use WESTest's automatic teardown. The always-run
summary step also stops a container whose Docker client was interrupted, allowing
its normal teardown window. Force cancellation or runner loss can still interrupt
cleanup.

The Actions log contains the live WESTest output. Each scenario uploads only an
allowlisted `summary.json`: scenario, provider, image, chart version, application
version, resolved server manifest digest, container name, exit code,
duration when available, and the number of cleanup checkpoints
remaining. Upgrade summaries also record the starting chart version alongside
the destination chart version. The job summary flags remaining checkpoints. The
report job includes both local and cloud outcomes; `WESTEST_SLACK_WEBHOOK` remains
optional for failure notifications.

**Failed cloud cleanup recovery is deferred.** Raw Terraform state, WESTest
checkpoints, kubeconfigs, credential files and run logs are not uploaded from
this public repository. There is no encrypted recovery bundle, recovery secret,
or automated retry after WESTest's normal teardown. Runner-local state disappears
when GitHub deletes the runner. If cleanup fails, use the cloud account's resource
inventory and the run/cluster identifiers in the Actions log to find and clean up
remaining resources; do not assume the absence of a checkpoint proves cleanup
completed after a forced interruption.

Follow-up work includes a secure recovery mechanism for failed teardown and the
etcd PVC cleanup issue observed during local AWS/GCP validation. Keep the cron
disabled until cleanup behavior is acceptable for unattended runs.

Offline wrapper and matrix checks run in the regular PR test workflow:

```bash
python3 -B -m unittest discover -s hack/scripts -p test_nightly_westest.py
```
