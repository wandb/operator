# PR WESTest testing

[The PR workflow](../.github/workflows/pr-westest.yaml) runs seven local Kind
scenarios against the PR merge commit's operator checkout:

- `local-kind-ingress`
- `local-kind-gateway`
- `local-kind-oidc-pkce`
- `local-kind-oidc-implicit`
- `local-kind-proxy`
- `local-kind-proxy-auth`
- `local-kind-proxy-tls`

The external-datastore scenario is temporarily excluded.

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
into the public WESTest `0.5.2` container.

The shared [runner action](../.github/actions/westest-run/action.yml) passes
`--operator-repo /src/operator` to both `westest config` and `westest run` and
explicitly enables local builds. WESTest builds the checkout's Dockerfile,
loads that image directly into Kind, and installs a writable copy of the local
chart, including its sibling chart dependencies and local CRDs. Scenario-pinned
published operator versions are not used in this mode.

This workflow does not publish an operator image or chart, request cloud
credentials, or run cloud scenarios. The separate
[nightly workflow](nightly-westest-testing.md) continues to test published charts.

Each scenario has a 60-minute execution budget plus 11 minutes for cancellation
cleanup, within a 90-minute job limit. Failures do not cancel sibling scenarios.
The action retains the usual automatic teardown and uploads only an allowlisted
`summary.json`, identifying the operator checkout by commit SHA. Full logs remain
in Actions; credentials, Terraform state, and kubeconfigs are not uploaded.
