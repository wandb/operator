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
make bundle-community VERSION=2.0.0 BUNDLE_REPLACES=wandb-operator.v1.21.2
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

The CSV `replaces` field defaults to `wandb-operator.v1.21.2`. Override
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
`replaces` not pointing at `wandb-operator.v1.21.2`. If the pipeline does not
yet publish a 4.21 catalog, the `v4.21` floor may need a brief hold.

## Upgrade from 1.21.2

This bundle replaces `wandb-operator.v1.21.2`. It is a v1→v2 CRD conversion:
the storage version is v2, conversion/defaulting/validation webhooks are
required, and OLM must install those webhooks before the new CRD is applied.

## Catalog dependencies

`metadata/dependencies.yaml` declares operators that exist in
community-operators-prod. Moco and SeaweedFS are omitted (not in the catalog).

```yaml
dependencies:
  - type: olm.package
    value: { packageName: redis-operator, version: ">=0.15.0" }
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

### Redis catalog vs Helm

The Helm chart pins redis-operator **0.22.2**. OpenShift
community-operators-prod currently maxes out at **0.15.1** (OperatorHub.io
has 0.25.0; those are different catalogs). The OLM range is `>=0.15.0`, so a
catalog install resolves **0.15.1**.

Creating Redis CRs against 0.15.0 / 0.15.1 does **not** fail admission. The
0.15.x CRDs use structural schemas: fields they do not define are stripped
and stored without those keys. The wandb-operator apply still returns
success.

This operator does not set `readOnlyRootFilesystem` today. The 0.15.x
`spec.securityContext` schema **does** include that field, so if it is added
later it is applied, not ignored.

| Field this operator writes | 0.15.x Redis / RedisReplication | 0.15.x RedisSentinel |
| --- | --- | --- |
| `spec.securityContext` (runAsNonRoot, drop ALL, …) | kept | kept |
| `spec.securityContext.readOnlyRootFilesystem` (if set) | **applied** | **applied** |
| `spec.storage.volumeMount` (`/tmp` emptyDir) | kept | n/a (Sentinel has no `storage`) |
| `spec.volumeMount` (`/tmp` emptyDir) | n/a | **pruned** |
| `spec.redisExporter.port` | **pruned** | **pruned** |
| `spec.redisExporter.securityContext` | **pruned** | **pruned** |

**Without RORFS (current code):** standalone and replication stay usable.
Sentinel HA also runs because the container root FS stays writable, so the
missing `/tmp` mount does not matter. Telemetry exporters start without the
intended port (9121) and container security context — redis-operator uses
its own defaults.

**With RORFS on `spec.securityContext`:** standalone and replication can
still work because `storage.volumeMount` survives and supplies `/tmp`.
Sentinel cannot: RORFS is applied and `spec.volumeMount` is dropped, so
Sentinel pods lose a writable `/tmp` and typically CrashLoop. That is a
runtime failure, not a CRD validation error.

Pruned fields also make desired ≠ live on every reconcile
(`resourceContentEqual`), so Sentinel CRs (and any Redis CR with a
telemetry exporter) are updated on each loop. The update succeeds; the
unknown fields are stripped again.

0.15.x on OpenShift also has separate upstream RBAC gaps for
`redisreplications` / `redissentinels` (see
[OT-CONTAINER-KIT/redis-operator#665](https://github.com/OT-CONTAINER-KIT/redis-operator/issues/665)
and
[#1665](https://github.com/OT-CONTAINER-KIT/redis-operator/issues/1665)).
Those can block the redis-operator itself from reconciling HA CRs even
when wandb-operator writes them successfully.

For RORFS or Sentinel HA, install redis-operator **≥ 0.22.2** out of band,
or use `spec.redis.default.externalRedis`. Do not rely on the catalog
0.15.x operator.

### Managed MySQL and object store

Moco and SeaweedFS are not in the catalog. On an OLM-only cluster, use
`spec.mysql.default.externalMysql` and
`spec.objectStore.default.externalObjectStore` (and bring your own ingress on
OpenShift). The bundle sample follows that shape. The generated ClusterRole
still includes Moco/SeaweedFS verbs so out-of-band installs keep working.

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
