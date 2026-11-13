# Test Operator changes

Choose checks for the layer being changed. API and reconciliation tests run
without provisioning a full W&B deployment; cluster scenarios cover dependency
controllers, networking, migrations, and application behavior.

## Unit and envtest checks

When Helm is installed, the Go telemetry tests also render the charts. Complete
the dependency setup in [chart checks](#chart-checks) before the first `make test`
in a fresh checkout or worktree.

```bash
make lint
make test
go tool cover -html=cover.out
```

`make test` runs generation, embedded CRD synchronization, vet, envtest setup,
and the Go tests outside the e2e and vendored packages. It writes `cover.out`.
Use the Go version in [go.mod](../../go.mod); CI tool versions are recorded in
[run-tests.yaml](../../.github/workflows/run-tests.yaml). Downloads may be needed
on the first run. To run one envtest package directly, run `make setup-envtest`
first; the suites find its binaries under `bin/k8s`, for example
`go test ./internal/webhook/v2/...`. After changing a mocked interface,
regenerate its fakes:

```bash
go generate ./...
```

## Chart checks

Install Helm and chart-testing, add the
[chart dependency repositories](setup.md#chart-dependency-repositories), and
resolve dependencies before checking templates:

```bash
helm dependency build deploy/operator
ct lint --all --config deploy/ct.yaml
helm lint --strict deploy/operator
helm template wandb-operator deploy/operator \
  --namespace wandb-operators --include-crds > /tmp/operator-rendered.yaml
```

The [chart validation workflow](../../.github/workflows/chart-validation.yaml)
lists the repository setup and checks every profile. The forwarding profile
requires `telemetry.forwarding.otlp.endpoint`. Include OpenShift, telemetry off,
full, and forward configurations when changing shared templates. Inspect CRD and
webhook output in addition to checking whether rendering succeeds.

## Local integration testing

Use [Tilt setup](setup.md) for interactive testing. The `useExternalMysql`,
`useExternalRedis`, `useExternalObjectStore`, and `useCustomCA` settings compose
test infrastructure and generated connection material. The
[colocated custom CA runbook](../../hack/testing-manifests/wandb/custom-ca-e2e/README.md)
documents its verifier.

To exercise reuse of detached infrastructure in an isolated development cluster:

1. Start a W&B CR with `retentionPolicy="detach"`.
2. Delete that CR and wait for finalization while the controllers are running.
3. Run `./hack/scripts/managed-connections-to-external.sh <wandbName> <wandbNamespace>`
   (`wandb wandb` for the Tilt defaults). It renames each `<wandbName>-*-connection`
   Secret to `external-*`; without arguments it searches the `default` namespace.
4. Set `crFile` to a CR using those external connections and bring it up in Tilt.
   [wandb-external-v2.yaml](../../hack/testing-manifests/test-infra/examples/wandb-external-v2.yaml)
   and the `external-*` overlays under `hack/testing-manifests/wandb/kustomize/overlays/`
   reference those Secret names.
5. Verify readiness and actual application data access.

`make test-e2e` and `make test-e2e-retention` use a real cluster and change
resources. Inspect their prerequisites and selected context before using them.

Application data testing should use independent write/read checks for SDK runs,
artifacts, and the enabled W&B/Weave features. The previously discussed broader
seed-and-verify dataset is still a documentation gap; do not treat a proposed
coverage inventory as an implemented tool.

## WESTest in CI

The [PR workflow](pr-westest-testing.md) tests eight local Kind scenarios against
non-draft PR merge commits targeting `main`. It builds the checked-out Operator
and chart inside the test environment. Fork and Dependabot runs skip these tests;
airgap scenarios are temporarily excluded.

The [nightly workflow](nightly-westest.md) tests published Operator charts. Manual
dispatch can build a development/nightly chart, or select an existing chart with
`operator_version`. The matrix includes nine local scenarios and seven GCP
scenarios; cloud scenarios run only on `main`. AWS remains disabled, and the daily
schedule remains commented out pending cloud cleanup validation.

Both workflows use the public WESTest container and resolve one stable W&B
application version and manifest digest for each run. The operator-upgrade
scenario holds the application version fixed across its two phases. Review the
workflow-specific guides for credentials, parameters, timeouts, and cleanup.

The offline runner and matrix checks are defined in
[run-tests.yaml](../../.github/workflows/run-tests.yaml), but that job is
currently disabled. Run them locally:

```bash
python3 -B -m unittest discover -s hack/scripts -p test_nightly_westest.py
```

The [original nightly plan](../design/nightly-westest-plan.md) remains historical;
use the current runbooks above for operation and extension.
