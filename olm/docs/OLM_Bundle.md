# OLM Bundle Overview

This repository generates an Operator Lifecycle Manager (OLM) bundle that can
be submitted to
[community-operators-prod](https://github.com/redhat-openshift-ecosystem/community-operators-prod)
under `operators/wandb-operator/<version>/`.

The published 1.x packages (`1.0.0`, `1.21.2`, `1.22.0`) are the Helm-era
operator and live in [`olm/olm-catalog/`](../olm-catalog/) as a snapshot. Do not
generate into that directory. New versions are produced from current sources
with `make bundle` / `make bundle-community`.

## Generate the bundle

Requires `operator-sdk`, `kustomize` (`make kustomize` installs it), and
`python3`.

```bash
# Uses deploy/operator/Chart.yaml appVersion and the public v2 image.
make bundle

# Optional overrides
make bundle VERSION=2.0.0-beta.10 IMG=us-docker.pkg.dev/wandb-production/public/wandb/operator:2.0.0-beta.10
```

This writes `./bundle` (manifests, metadata, scorecard tests) and
`bundle.Dockerfile` at the repo root, then runs `operator-sdk bundle validate`.

## Stage a community-operators-prod directory

```bash
make bundle-community
# or
make bundle-community VERSION=2.0.0-beta.10
```

This stamps community-only metadata onto `./bundle` and copies the result to
`dist/community-operators/operators/wandb-operator/<VERSION>/`:

| File | Purpose |
| --- | --- |
| `manifests/` | CRDs (`WeightsAndBiases` v1+v2, `Application`, `ActionRun`), CSV, RBAC, Services |
| `metadata/annotations.yaml` | Package `wandb-operator`, channel, `com.redhat.openshift.versions: v4.21` |
| `metadata/dependencies.yaml` | OLM package deps (see below) |
| `tests/scorecard/` | Scorecard config |
| `bundle.Dockerfile` | Scratch image for the version directory |

### Knobs

| Variable | Default | Notes |
| --- | --- | --- |
| `VERSION` | chart `appVersion` | Bundle/CSV version (no leading `v`). |
| `CHANNELS` | `beta` | Channel(s) this bundle is added to. Prereleases go to `beta`. |
| `DEFAULT_CHANNEL` | `stable` | Package default channel. Stays `stable` so 1.x installs are not auto-upgraded onto a beta. |
| `BUNDLE_MATURITY` | `beta` | CSV `spec.maturity`. Use `stable` for GA. |
| `BUNDLE_REPLACES` | empty | CSV `spec.replaces`. Empty for the first bundle in a channel; set `wandb-operator.v1.22.0` (or the previous 2.x) when a channel should upgrade from an earlier bundle. |
| `OPENSHIFT_VERSIONS` | `v4.21` | `com.redhat.openshift.versions`; matches CRC 2.60.1 / OpenShift 4.21.8 and CSV `minKubeVersion: 1.34.0`. |

`operator-sdk bundle validate` only requires the default-channel annotation to
be non-empty, so `channels=beta` with `default=stable` is a valid bundle even
though `stable` does not contain this version.

### Publishing a GA 2.x

```bash
make bundle-community VERSION=2.0.0 CHANNELS=stable,beta \
  BUNDLE_MATURITY=stable BUNDLE_REPLACES=wandb-operator.v1.22.0
```

Package-level [`ci.yaml`](https://github.com/redhat-openshift-ecosystem/community-operators-prod/blob/main/operators/wandb-operator/ci.yaml)
already exists upstream (`replaces-mode` and reviewers). Do not copy a second
`ci.yaml` into the version directory.

## Validate like the pipeline

The hosted pipeline runs `check_osdk_bundle_validate_operator_framework` and
`check_osdk_bundle_validate_operatorhub`, which map to:

```bash
D=dist/community-operators/operators/wandb-operator/<VERSION>
operator-sdk bundle validate $D --select-optional suite=operatorframework
operator-sdk bundle validate $D --select-optional name=operatorhub
operator-sdk bundle validate $D --select-optional name=community
```

Expect a warning about "APIs which were deprecated and removed in v1.25" for
`cronjobs`, `horizontalpodautoscalers`, `poddisruptionbudgets`,
`endpointslices`, `events`. The validator cannot see API versions in RBAC
rules and flags any rule naming those resources; the 1.22.0 bundle carried the
same warning and merged.

Optionally build the staged image to prove the Dockerfile is self-contained:

```bash
(cd $D && docker build -f bundle.Dockerfile -t wandb-operator-bundle:<VERSION> .)
```

## Submit a community-operators-prod PR

1. Fork [community-operators-prod](https://github.com/redhat-openshift-ecosystem/community-operators-prod).
2. Copy the staged directory:

   ```bash
   cp -R dist/community-operators/operators/wandb-operator/<VERSION> \
     /path/to/community-operators-prod/operators/wandb-operator/
   ```

3. One signed-off commit (`git commit -s`; author must match the sign-off), PR
   title `operator wandb-operator (<VERSION>)`. Only the new version directory
   may change: one bundle per PR, no edits to other versions or files.
4. Fill in the PR template checklist. Authors listed in `ci.yaml` `reviewers`
   get auto-merge once the pipeline is green; otherwise a listed reviewer
   comments `/approve`.
5. If `acquire-lease` or another task flakes, comment
   `/pipeline restart operator-hosted-pipeline`.

Typical failures: cert-manager CRs left in manifests, missing
`com.redhat.openshift.versions`, unresolvable `olm.package` versions,
`alm-examples` missing an owned kind, or a `replaces` target that is not in the
same channel.

## Catalog dependencies

`metadata/dependencies.yaml` declares operators that exist in
community-operators-prod. OLM installs these when the wandb-operator
Subscription is created.

```yaml
dependencies:
  - type: olm.package
    value: { packageName: clickhouse, version: ">=0.26.3" }
  - type: olm.package
    value: { packageName: grafana-operator, version: ">=5.21.0" }
  - type: olm.package
    value: { packageName: victoriametrics-operator, version: ">=0.66.3" }
  - type: olm.package
    value: { packageName: redis-operator, version: ">=0.22.2" }
```

Grafana and VictoriaMetrics are optional Helm chart deps (telemetry); they are
required OLM deps so a catalog install always pulls them.

Redis: the catalog now carries redis-operator 0.25.0+ (Helm pins 0.22.2), so
the package dependency and the `redisreplications` / `redissentinels`
`required` CRDs are declared. Catalog releases <= 0.15.1 cannot reconcile the
HA Redis CRs this operator writes, hence the floor.

### Managed MySQL and object store

Moco and SeaweedFS are not in the catalog. On an OLM-only cluster, use
`spec.mysql.default.externalMysql` and
`spec.objectStore.default.externalObjectStore` (and bring your own ingress on
OpenShift). The bundle sample follows that shape. The generated ClusterRole
still includes Moco/SeaweedFS verbs so out-of-band installs keep working.

## Sample CRs (`alm-examples`)

[`config/olm/samples/`](../../config/olm/samples/) feeds `alm-examples`; there
must be one example per owned CRD version or the pipeline flags it. The
`WeightsAndBiases` v2 sample must satisfy the validating webhook:
`spec.wandb.hostname` and `spec.wandb.version` (a published W&B Server version)
are required, and secret references use `valueFrom.secretKeyRef`.

## Upgrade from 1.22.0

A 2.x bundle that `replaces` `wandb-operator.v1.22.0` is a v1 -> v2 CRD
conversion: the storage version is v2, conversion/defaulting/validation
webhooks are required, and OLM must install those webhooks before the new CRD
is applied. The beta channel does not set `replaces`, so existing 1.22.0
installs are untouched until a GA bundle is published to `stable`.

## Directory layout

```
$ tree dist/community-operators/operators/wandb-operator/<VERSION>

<VERSION>
├── bundle.Dockerfile
├── manifests
│   ├── apps.wandb.com_actionruns.yaml
│   ├── apps.wandb.com_applications.yaml
│   ├── apps.wandb.com_weightsandbiases.yaml
│   ├── ...
│   └── wandb-operator.clusterserviceversion.yaml
├── metadata
│   ├── annotations.yaml
│   └── dependencies.yaml
└── tests
    └── scorecard
        └── config.yaml
```
