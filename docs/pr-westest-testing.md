# PR WESTest testing

[The PR workflow](../.github/workflows/pr-westest.yaml) runs only
`local-kind-external` on this diagnostic branch, against the PR merge commit's
operator checkout. The branch starts from [PR #438](https://github.com/wandb/operator/pull/438)
and disables its other scenarios to isolate the external datastore provisioning
failure. The airgap scenarios also remain disabled.

Non-draft PRs targeting `main` run when opened, updated, reopened, or
marked ready for review. New updates or conversion back to draft cancel older
runs. Fork PRs and Dependabot-triggered runs skip these tests because downloading
the private Watchtower binary requires `WESTEST_APP_ID` and
`WESTEST_APP_PRIVATE_KEY`.

Setup resolves the latest published stable application version once and passes
that version and its server-manifest digest to every scenario. This keeps the
application consistent across the matrix while testing the local operator.

Each scenario gets an `ubuntu-latest-8-cores` runner, with up to five scenarios
in parallel. The workflow prepares the amd64 Watchtower binary and Grafana
plugins required by the operator Dockerfile, then mounts the checkout read-only
into the public WESTest `0.8.0` container.

The shared [runner action](../.github/actions/westest-run/action.yml) passes
`--operator-repo /src/operator` to both `westest config` and `westest run` and
explicitly enables local builds. For the enabled scenarios, WESTest builds the
checkout's Dockerfile, loads that image directly into Kind, and installs a
writable copy of the local chart, including its sibling chart dependencies and
local CRDs.

The shared action retains support for airgap scenarios. WESTest mirrors the
published dependencies, then builds the checkout's operator image and packages
its chart into the temporary test registry. The deployment pulls the PR-built
image and chart from that registry.
The enforce variant also blocks public egress from the Kind nodes. The runner
uses its bundled WSM executable and mounts the work directory at the same
absolute path inside and outside the container so the sibling registry can
read its TLS files. It also sets `XDG_CACHE_HOME` to the shared work directory's
`cache` subdirectory, keeping the targets of Terraform's module symlinks visible
to the host Docker daemon.

Operator artifacts stay within the test environment; the workflow does not
publish them to a release registry. The separate
[nightly workflow](nightly-westest-testing.md) continues to test published charts.

Each scenario has a 60-minute execution budget plus 11 minutes for cancellation
cleanup, within a 90-minute job limit. Failures do not cancel sibling scenarios.
The action retains automatic teardown and uploads `summary.json`, identifying
the operator checkout by commit SHA. This branch enables the action's
`include-external-diagnostics` input, which also stages the following WESTest
0.8.0 captures under `diagnostics/<run-id>/` in the `westest-local-kind-external`
artifact, including after a provisioning failure:

- Cluster node status/descriptions, pod and PVC lists, and events.
- External datastore resource lists, pod/PVC descriptions, and events.
- Current and previous logs from the MySQL, Redis, ClickHouse, and SeaweedFS pods.

The exporter omits environment, command, argument, and annotation sections from
descriptions and redacts credential assignments, URL credentials, and private
key blocks. It uses explicit file patterns and rejects symlinks; raw Terraform
state, checkpoints, kubeconfigs, run environment files, and application logs are
not uploaded. Missing diagnostics produce a warning and retain the summary.
The summary reports the number of diagnostic files. Uploads are retained for
14 days. Other action callers retain summary-only uploads by default.
