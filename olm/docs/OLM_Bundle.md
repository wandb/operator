# OLM Bundle Overview

This repository generates an Operator Lifecycle Manager (OLM) bundle that can
be submitted to
[community-operators-prod](https://github.com/redhat-openshift-ecosystem/community-operators-prod)
under `operators/wandb-operator/<version>/`.

The published 1.x packages (`1.0.0`, `1.21.2`) are the Helm-era operator and
live in [`olm/olm-catalog/`](../olm-catalog/) as a snapshot. Do not generate
into that directory. New versions are produced from current sources with
`make bundle` / `make bundle-community`.

## Generate the bundle

Requires `operator-sdk`, `kustomize` (`make kustomize` installs it), and
`python3`.

```bash
# Uses deploy/operator/Chart.yaml appVersion and the public v2 image.
make bundle

# Optional overrides
make bundle VERSION=2.0.0 IMG=us-docker.pkg.dev/wandb-production/public/wandb/operator:2.0.0
```

This writes `./bundle` (manifests, metadata, scorecard tests) and
`bundle.Dockerfile` at the repo root, then runs `operator-sdk bundle validate`.

## Stage a community-operators-prod directory

```bash
make bundle-community
# or
make bundle-community VERSION=2.0.0 BUNDLE_REPLACES=wandb-operator.v1.22.0
```

This stamps community-only metadata onto `./bundle` and copies the result to
`dist/community-operators/operators/wandb-operator/<VERSION>/`:

| File | Purpose |
| --- | --- |
| `manifests/` | CRDs, CSV, RBAC, webhook configs |
| `metadata/annotations.yaml` | Package `wandb-operator`, channel `stable`, `com.redhat.openshift.versions: v4.21` |
| `metadata/dependencies.yaml` | OLM package deps (see below) |
| `tests/scorecard/` | Scorecard config |
| `bundle.Dockerfile` | Scratch image for the version directory |

`com.redhat.openshift.versions` is `v4.21` (OpenShift 4.21 and later), matching
CRC 2.60.1 / OpenShift 4.21.8. The CSV `minKubeVersion` is `1.34.0`. The 1.x
catalog entries stay on older OpenShift catalogs; only this version is 4.21+.

The CSV `replaces` field defaults to `wandb-operator.v1.22.0`. Override
`BUNDLE_REPLACES` when publishing a later 2.x.

Package-level [`ci.yaml`](https://github.com/redhat-openshift-ecosystem/community-operators-prod/blob/main/operators/wandb-operator/ci.yaml)
already exists upstream (`replaces-mode` and reviewers). Do not copy a second
`ci.yaml` into the version directory.

## Submit a community-operators-prod PR

1. Fork [community-operators-prod](https://github.com/redhat-openshift-ecosystem/community-operators-prod).
2. Copy the staged directory:

   ```bash
   cp -R dist/community-operators/operators/wandb-operator/<VERSION> \
     /path/to/community-operators-prod/operators/wandb-operator/
   ```

3. Open a PR. Pipeline checks include bundle validation and scorecard.

Typical failures: cert-manager CRs left in manifests, missing
`com.redhat.openshift.versions`, unresolvable `olm.package` versions, or
`replaces` not pointing at `wandb-operator.v1.22.0`. If the pipeline does not
yet publish a 4.21 catalog, the `v4.21` floor may need a brief hold.

## Upgrade from 1.22.0

This bundle replaces `wandb-operator.v1.22.0`. It is a v1→v2 CRD conversion:
the storage version is v2, conversion/defaulting/validation webhooks are
required, and OLM must install those webhooks before the new CRD is applied.

## Catalog dependencies

`metadata/dependencies.yaml` declares operators that exist in
community-operators-prod. Moco and SeaweedFS are omitted (not in the catalog).

```yaml
dependencies:
  - type: olm.package
    value: { packageName: clickhouse, version: ">=0.26.3" }
  - type: olm.package
    value: { packageName: grafana-operator, version: ">=5.21.0" }
  - type: olm.package
    value: { packageName: victoriametrics-operator, version: ">=0.66.3" }
```

OLM installs these when the wandb-operator Subscription is created. Grafana
and VictoriaMetrics are optional Helm chart deps (telemetry); they are
required OLM deps so a catalog install always pulls them.

Redis is **not** an OLM package dependency. Community-operators-prod maxes
out at redis-operator **0.15.1**; Helm pins **0.22.2**, and 0.15.x cannot
reconcile the HA Redis CRs this operator writes (missing
`redisreplications` / `redissentinels` RBAC, pruned Sentinel fields). The
CSV therefore does not list Redis GVKs under
`customresourcedefinitions.required` — OLM treats required CRDs as hard
install deps, so declaring them still pulled catalog 0.15.1 even after the
package was dropped.

On an OLM-only cluster, use `spec.redis.default.externalRedis`. For managed
Redis, install redis-operator **≥ 0.22.2** out of band. The generated
ClusterRole still includes Redis verbs so that optional install works.

### Managed MySQL and object store

Moco and SeaweedFS are not in the catalog. On an OLM-only cluster, use
`spec.mysql.default.externalMysql` and
`spec.objectStore.default.externalObjectStore` (and bring your own ingress on
OpenShift). The bundle sample follows that shape, including external Redis.
The generated ClusterRole still includes Moco/SeaweedFS verbs so out-of-band
installs keep working.

## Directory layout

```
$ tree dist/community-operators/operators/wandb-operator/<VERSION>

<VERSION>
├── bundle.Dockerfile
├── manifests
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
