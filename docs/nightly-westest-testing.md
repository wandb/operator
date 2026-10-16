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
| `aws-eks-s3-irsa` | `refs/heads/main` only | `westest-aws` |
| `gcp-gke-gcs-hmac` | `refs/heads/main` only | `westest-gcp` |

An empty `scenario` input selects all five local scenarios and, on main, both
cloud scenarios. A named scenario selects only that scenario. Unsupported names
and attempts to select a cloud scenario on another ref fail during setup. There
is no pull-request trigger for these provisioning tests.

Local tests use `ubuntu-latest-8-cores`, with up to three scenarios in parallel.
Their Kind clusters need the additional CPU. Cloud tests use ordinary
`ubuntu-latest` runners, with up to two scenarios in parallel; Kubernetes runs in
the cloud account. A failed scenario does not cancel its siblings. Runs on the
same ref are serialized, and a new dispatch does not cancel an active run.

## Container and chart versions

The workflow pins the public multi-architecture WESTest v0.4.0 image by digest:

```text
us-docker.pkg.dev/wandb-production/public/westest/westest@sha256:a23a657156cd5e7f9bde57d771f40877dcbd4ca919663a85a2e083ebb05dda97
```

The image contains WESTest, its scenarios and Terraform fixtures, Terraform,
Helm, kubectl, Kind, Docker CLI, the AWS and Google Cloud CLIs, the GKE auth plugin,
and the W&B Python CLI. Tests no longer download a release binary or install
these tools on the host. The image is publicly pullable without a GitHub App
installation token.

The shared [Docker action](../.github/actions/westest-run/action.yml) mounts a
fresh runner-local work directory at `/work`. Local tests also mount the Docker
socket and use host networking so WESTest can reach Kind's API and application
ports. The image entrypoint configures scenario hostnames in its own `/etc/hosts`.
Cloud tests mount their credentials but do not mount the Docker socket or use
host networking.

Both `westest config` and `westest run` receive
`--param operatorVersion=<chart version>`. This overrides scenario phase defaults;
setting only `WESTEST_OPERATOR_CHART_VERSION` would leave scenario-pinned chart
versions in effect.

The reusable [nightly build](../.github/workflows/nightly-build.yaml) is unchanged:

- Scheduled runs use `<base>-nightly-<UTC timestamp>`.
- Manual builds default to `<base>-dev-[<dev_tag>-]<UTC timestamp>`. The optional
  label is sanitized, and the `channel` input can select `nightly` instead.
- The image tag and chart version are identical. The image is published under
  `us-docker.pkg.dev/wandb-production/public/wandb/operator`, and the chart under
  `oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator`.
- Setting `operator_version` skips the build and tests that published chart.

## Cloud authentication

Only the cloud test job requests `id-token: write`. Each matrix entry selects its
GitHub environment before requesting credentials. Environments provide a place
for deployment branch restrictions, reviewers and other protections; cloud trust
also independently checks the main branch.

### AWS

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
access token. Tests target `playground-111` in `us-central1` with
`WESTEST_CLOUD_OPT_IN=1`.

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
   `WESTEST_APP_ID`, and `WESTEST_APP_PRIVATE_KEY`. The App credentials are still
   used by the build to fetch watchtower; test jobs no longer need them.
4. Merge the workflow, then dispatch one AWS and one GCP scenario on main against
   a known published chart. Confirm OIDC login, provisioning, validators and
   teardown. These main-branch runs are the end-to-end authentication check;
   local tests cannot mint the same GitHub identity.
5. Check the cloud accounts for leftover resources, including the known etcd PVC
   cleanup issue, before enabling the cron or widening the cloud matrix.

For example, substitute a published chart for `$CHART_VERSION`:

```bash
gh workflow run nightly.yaml --repo wandb/operator --ref main \
  -f scenario=aws-eks-s3-irsa -f operator_version="$CHART_VERSION"
gh workflow run nightly.yaml --repo wandb/operator --ref main \
  -f scenario=gcp-gke-gcs-hmac -f operator_version="$CHART_VERSION"
```

To test a branch locally in Kind through Actions, dispatch that branch with a
local scenario. Omit `operator_version` to build the selected branch:

```bash
gh workflow run nightly.yaml --repo wandb/operator --ref my-branch \
  -f scenario=local-kind-ingress -f dev_tag=my-change
```

## Timeouts, results and cleanup follow-ups

Local runs have a 60-minute execution budget, followed by up to 11 minutes for
WESTest's 10-minute cancellation cleanup. Cloud runs have a 75-minute execution
budget and 46 minutes for WESTest's 45-minute cancellation cleanup. The wrapper
sends SIGINT first, and preserves the timeout's nonzero exit status. Job limits
are 90 and 180 minutes respectively, leaving time for setup and finalization.
Normal success and failure use WESTest's automatic teardown. The always-run
summary step also stops a container whose Docker client was interrupted, allowing
its normal teardown window. Force cancellation or runner loss can still interrupt
cleanup.

The Actions log contains the live WESTest output. Each scenario uploads only an
allowlisted `summary.json`: scenario, provider, image, chart version, container
name, exit code, duration when available, and the number of cleanup checkpoints
remaining. The job summary flags remaining checkpoints. The report job includes
both local and cloud outcomes; `WESTEST_SLACK_WEBHOOK` remains optional for failure
notifications.

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
