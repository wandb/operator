# Test Operator changes

Choose checks for the layer being changed. API and reconciliation tests run
without provisioning a full W&B deployment; cluster scenarios cover dependency
controllers, networking, migrations, and application behavior.

## Unit and envtest checks

```bash
make lint
make test
go tool cover -html=cover.out
```

`make test` runs generation, embedded CRD synchronization, vet, envtest setup,
and the Go tests outside the e2e and vendored packages. It writes `cover.out`.
Use the Go version in [go.mod](../../go.mod); CI tool versions are recorded in
[run-tests.yaml](../../.github/workflows/run-tests.yaml). Downloads may be needed
on the first run. After changing a mocked interface, regenerate its fakes:

```bash
go generate ./...
```

## Chart checks

Install Helm and chart-testing, configure the chart dependency repositories, and
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
3. Run `./hack/scripts/managed-connections-to-external.sh` to prepare external
   connection Secrets.
4. Set `crFile` to a CR using those external connections and bring it up in Tilt.
5. Verify readiness and actual application data access.

`make test-e2e` and `make test-e2e-retention` use a real cluster and change
resources. Inspect their prerequisites and selected context before using them.

Application data testing should use independent write/read checks for SDK runs,
artifacts, and the enabled W&B/Weave features. The previously discussed broader
seed-and-verify dataset is still a documentation gap; do not treat a proposed
coverage inventory as an implemented tool.

## WESTest in CI

The current [Nightly WESTest workflow](../../.github/workflows/nightly.yaml)
supports manual dispatch. Its schedule is commented out. Dispatch can build a
development/nightly chart and image, or test a supplied `operator_version`
without building. An optional `scenario` selects one scenario.

The default smoke set covers local Ingress, Gateway, OIDC PKCE, OIDC implicit, and
proxy scenarios. It uses the pinned WESTest release and large runners declared
in the workflow. The repository's local `westest-run` action downloads WESTest
with a GitHub App token; required secrets and permissions are defined in that
workflow. Inspect uploaded scenario artifacts and the report job on failure.

The [original nightly plan](../design/nightly-westest-plan.md) contains rollout
history, proposed scenarios, and prerequisites. Confirm them against the active
workflow before operating or extending it. A scenario must test the image paired
with the intended chart version, not an unrelated published image.
