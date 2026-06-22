# Container Images

This document maps **all container images** required to deploy the W&B
application and its backing infrastructure: where each image's version is
defined, and where it is consumed.

## How images get into a deployment

Images come from **three distinct sources**, which is the key to understanding
"where versions are defined":

1. **Server manifest (runtime-pulled)** — the W&B application images. Not in
   this repo at all; resolved per-version from an OCI artifact.
2. **Hardcoded Go constants** — the managed backing-service images (MySQL,
   Redis, Kafka, ClickHouse, SeaweedFS, plus their exporters).
3. **Helm chart** — the operator's own image and the third-party *operators*
   (which in turn pull their own images, not pinned here).

---

## 1. W&B server applications (manifest-driven)

These are **not** defined anywhere in this repo. They come from the
[server manifest](../CLAUDE.md) selected by the CR's `spec.wandb.manifestRepository`
+ `spec.wandb.version`.

| Aspect | Location |
|--------|----------|
| Version defined | The server manifest itself (OCI artifact, generated upstream in `wandb/core`). Local copies under `hack/testing-manifests/server-manifest/<version>/` |
| Manifest type | `ImageRef{Repository, Tag, Digest}` per app — `pkg/wandb/manifest/manifest.go` |
| Resolved at runtime | `GetServerManifest()` in `pkg/wandb/manifest/manifest.go` |
| Consumed (apps) | `internal/controller/reconciler/reconcile_v2.go` (`~514-619`) builds Application CRs → `internal/controller/reconciler/pods.go` `resolveContainers` (`~64-150`) builds pod specs from `app.Image.Repository/Tag/Digest` |
| Consumed (migrations) | `pkg/wandb/manifest/manifest.go` (`MigrationJob.Image`, `~234`) → `reconcile_v2.go` `runMigrationJobs` (`~1079-1150`) |

This covers all W&B microservices (api, executor, filestream, parquet, weave,
weave-trace, nginx-proxy, glue, etc.). **To change these versions, bump the
manifest — not this repo.**

---

## 2. Managed backing infrastructure (hardcoded Go constants)

All defined as `const` in the per-service `spec.go`, consumed when building the
vendored CR for each service.

| Image | Tag | Version defined (`const`) | Consumed |
|-------|-----|---------------------------|----------|
| `ghcr.io/cybozu-go/moco/mysql` | `8.4.8` | `internal/controller/infra/managed/mysql/moco/spec.go:22` (`MocoMySQLImage`) | `MySQLCluster` spec; MySQL init job in `internal/controller/reconciler/mysql.go:327` |
| `prom/mysqld-exporter` | `v0.15.1` | `internal/controller/infra/managed/mysql/moco/spec.go:23` (`DefaultMySQLExporterImage`) | MySQLCluster exporter sidecar (telemetry only) |
| `quay.io/opstree/redis` | `v7.0.15` | `internal/controller/infra/managed/redis/opstree/spec.go:26` (`RedisStandaloneImage`) | Redis CR |
| `quay.io/opstree/redis` | `v7.0.15` | `internal/controller/infra/managed/redis/opstree/spec.go:27` (`RedisReplicationImage`) | RedisReplication CR |
| `quay.io/opstree/redis-sentinel` | `v7.0.12` | `internal/controller/infra/managed/redis/opstree/spec.go:28` (`RedisSentinelImage`) | RedisSentinel CR |
| `quay.io/opstree/redis-exporter` | `v1.44.0` | `internal/controller/infra/managed/redis/opstree/spec.go:33` (`DefaultRedisExporterImage`) | Redis exporter sidecar (telemetry only) |
| `quay.io/strimzi/kafka` | `0.49.1-kafka-4.1.0` | `internal/controller/infra/managed/kafka/strimzi/spec.go:26` (`KafkaImage`) | Kafka CR (`ToKafkaVendorSpec`) |
| `altinity/clickhouse-server` | `25.8.16.10002.altinitystable` | `internal/controller/infra/managed/clickhouse/altinity/spec.go:24` (`ClickHouseImage`) | ClickHouseInstallation CR |
| `chrislusf/seaweedfs` | `latest` ⚠️ | `internal/controller/infra/managed/objectstore/seaweedfs/spec.go:21` (`SeaweedImage`) | Seaweed CR |

> ⚠️ SeaweedFS is pinned to `latest`, which is a reproducibility / supply-chain
> concern worth tracking.

**To change these:** edit the `const` in the relevant `spec.go`.

---

## 3. Operator + Helm chart images

Defined in `deploy/operator/` (chart `operator`, version `2.0.0-alpha.2`).

| Image | Tag | Version defined | Consumed |
|-------|-----|-----------------|----------|
| `us-docker.pkg.dev/wandb-production/public/wandb/operator` | `2.0.0-alpha.2` | `deploy/operator/values.yaml:13-14` | Operator Deployment (via `wandb-base` chart). Built from `Dockerfile` — packs both `/manager` and `/crd-installer` into one image |
| `alpine/k8s` | `1.35.4` | `deploy/operator/values.yaml:177-178` | Altinity ClickHouse operator CRD hook |

---

## 4. Third-party operators (pull their own images)

These Helm dependencies (`deploy/operator/Chart.yaml`) install operators that
manage their *own* image versions — **not pinned in this repo**; only the
operator chart version is.

| Dependency | Chart version | Manages |
|------------|---------------|---------|
| `wandb-base` | 0.11.8 | W&B operator deployment / RBAC / webhooks |
| `moco` | 0.24.0 | MySQL |
| `redis-operator` | 0.22.2 | Redis / Sentinel / Replication |
| `strimzi-kafka-operator` | 0.50.0 | Kafka |
| `seaweedfs-operator` | 0.1.24 | SeaweedFS |
| `prometheus-operator-crds` | 29.0.0 | ServiceMonitor CRDs |
| `altinity-clickhouse-operator` | 0.26.3 | ClickHouse |
| `victoria-metrics-operator` | 0.58.1 | VMSingle / VMAgent / VLSingle / VTSingle |
| `grafana-operator` | 5.21.4 | Grafana UI |
| `telemetry` (local subchart) | 0.1.0 | OTLP gateway, dashboards, rules |

---

## 5. Telemetry stack

| Image | Tag | Version defined | Consumed |
|-------|-----|-----------------|----------|
| `otel/opentelemetry-collector-contrib` | `0.102.1` | `deploy/telemetry/templates/telemetry-otlp-gateway.yaml:113` | `victoria-otlp-gateway` Deployment |
| VictoriaMetrics components (VMSingle / VMAgent / VLSingle / VTSingle) | — | Managed by `victoria-metrics-operator` | `deploy/telemetry/templates/telemetry-victoria-core.yaml` |
| Grafana | — | Managed by `grafana-operator` | `deploy/telemetry/templates/telemetry-ui.yaml` |

Gateway / Ingress (`internal/controller/reconciler/gateway.go`,
`internal/controller/reconciler/ingress.go`) create `Gateway` / `HTTPRoute` /
`Ingress` resources only — the actual controller (e.g. NGINX Gateway Fabric) is
**external** and ships no operator-managed image.

---

## Summary: what you need to pull

- **App images** → whatever the chosen server manifest references (dynamic).
- **9 hardcoded infra images** → the constants in
  `internal/controller/infra/managed/*/*/spec.go`.
- **Operator image + `alpine/k8s`** → `deploy/operator/values.yaml`.
- **OTel collector** → `deploy/telemetry/templates/telemetry-otlp-gateway.yaml`.
- **Everything else** (Grafana, VictoriaMetrics, and each backing-service
  operator's own pods) is pulled by the third-party operator charts at their
  chart versions in `deploy/operator/Chart.yaml`.
