# Preflight checks catalog

This page lists every preflight check, both implemented and planned, grouped by the service it validates. For each check it says what a **pass** confirms. To run the checks, see [Running preflight checks with wsm](preflight-checks.md). To add one, see [Contributing a preflight check](contributing-preflight-checks.md).

A check runs when its CR field is present in the CR. `*` matches every instance name in the `mysql`, `redis`, `objectStore` and `clickhouse` maps.

**Implemented** column:

- ✅ Registered in `pkg/preflight/registry.go`.
- 🚧 Written on a feature branch, not yet merged.
- (blank) Planned.

Planned checks don't have a registry name yet; their **Check** cell is `—`.

**Type** column:

- **static**: reads the CR only, and runs with `--offline`.
- **cluster**: makes read-only Kubernetes API calls, unless noted.
- **connection**: dials an external service from the machine running the check.

## MySQL

| Check | CR field | Type | A pass confirms | Implemented |
|---|---|---|---|---|
| `externalMysqlCheck` | `spec.mysql.*.externalMysql` | connection | `host` and `port` are set. A connection succeeds using the same TLS settings the operator puts in the app's DSN. The credentials authenticate, and the database exists (the operator never creates it). | ✅ |
| — (extends `externalMysqlCheck`) | `spec.mysql.*.externalMysql` | connection | The server version meets W&B's minimum. | |
| — (extends `externalMysqlCheck`) | `spec.mysql.*.externalMysql` | connection | The user has the grants that W&B migrations need. | |
| — (extends `externalMysqlCheck`) | `spec.mysql.*.externalMysql` | connection | The charset and collation are the ones W&B expects (advisory). | |
| `externalMysqlInClusterCheck` | `spec.mysql.*.externalMysql` | cluster (creates a probe pod) | A probe pod in the CR's namespace opens a TCP connection to `host:port`. This means the app's network path works: cluster DNS, VPC-only hosts, and egress policies. No credentials are sent. | 🚧 `feat/Read-only-K8s-access-preflight` |
| `managedMysqlApisCheck` | `spec.mysql.*.managedMysql` | cluster | The MOCO `MySQLCluster` API (`moco.cybozu.com/v1beta2`) is served, so the MOCO operator is installed. | ✅ |

## Redis

| Check | CR field | Type | A pass confirms | Implemented |
|---|---|---|---|---|
| `externalRedisCheck` | `spec.redis.*.externalRedis` | connection | `host` and `port` are valid. Redis is reachable over TCP, or over TLS when a TLS setting or CA is given. `AUTH` + `PING` succeed. The result reports the server version when `INFO` is allowed. | ✅ |
| — (extends `externalRedisCheck`) | `spec.redis.*.externalRedis` | connection | The server version meets W&B's minimum. | |
| `managedRedisApisCheck` | `spec.redis.*.managedRedis` | cluster | The OpsTree Redis APIs are served: `Redis`, or `RedisReplication` + `RedisSentinel` in sentinel mode. | ✅ |

## ClickHouse

| Check | CR field | Type | A pass confirms | Implemented |
|---|---|---|---|---|
| `externalClickHouseCheck` | `spec.clickhouse.*.externalClickhouse` | connection | `replicated` is a valid boolean and has a `clusterName` when true. A query over HTTP succeeds and the database exists. The native `tcpPort` is reachable. When replicated, the cluster is defined in `system.clusters` and Keeper/ZooKeeper is reachable. | ✅ |
| `managedClickHouseApisCheck` | `spec.clickhouse.*.managedClickhouse` | cluster | The Altinity `ClickHouseInstallation` and `ClickHouseKeeperInstallation` APIs are served. | ✅ |

## Object store

| Check | CR field | Type | A pass confirms | Implemented |
|---|---|---|---|---|
| `externalObjectStoreCheck` | `spec.objectStore.*.externalObjectStore` | connection | The bucket or container exists and the credentials can access it. **S3/S3-compatible:** a signed `HeadBucket` succeeds and the region matches. **GCS:** the service account gets a token and can list objects. **Azure:** the account key is accepted and the container exists. Ambient credentials (IAM role, workload identity) report `SKIPPED`. | ✅ |
| — (extends `externalObjectStoreCheck`) | `spec.objectStore.*.externalObjectStore` | connection | Optional: a write/read/delete probe object succeeds, which proves the credentials can do more than read. | |
| `managedObjectStoreApisCheck` | `spec.objectStore.*.managedObjectStore` | cluster | The SeaweedFS `Seaweed` API is served. | ✅ |

## Managed infrastructure

| Check | CR field | Type | A pass confirms | Implemented |
|---|---|---|---|---|
| — | managed components (`spec.<component>.*.managed*`) | cluster | Exactly one default StorageClass exists, so PVCs for managed services can bind. | |
| — | managed components with a custom namespace | cluster | The custom namespace exists and the operator watches it. | |
| `clusterCapacityCheck` | `spec` | cluster | Some nodes are ready, schedulable, and tolerated by `spec.tolerations`. Together they have enough allocatable CPU and memory for the manifest's requests at `spec.size`, and every pod fits on at least one node. Pods already running are ignored, so a pass is necessary but not sufficient. | 🚧 `feat/Read-only-K8s-access-preflight` |

## Networking

| Check | CR field | Type | A pass confirms | Implemented |
|---|---|---|---|---|
| — | `spec.networking` (`mode: ingress`) | cluster | The named IngressClass exists, or, if none is named, a default IngressClass exists. | |
| — | `spec.networking` (`mode: gateway`) | cluster | The Gateway API is served. | |
| — | `spec.networking` (`mode: gateway`, managed Gateway) | cluster | The GatewayClass exists and is `Accepted`. | |
| — | `spec.networking` (`mode: gateway`, existing Gateway) | cluster | The Gateway and listener exist, and the listener admits routes from the W&B namespace. | |
| — | `spec.networking.tls` | cluster | Either the TLS Secret exists, is a valid certificate, isn't expired, and covers the hostname; or the cert-manager Issuer/ClusterIssuer exists and is `Ready`. | |

## Global settings

| Check | CR field | Type | A pass confirms | Implemented |
|---|---|---|---|---|
| `proxyCheck` | `spec.global.proxy` | static | The proxy URLs are valid `http`/`https` URLs, including Secret-backed ones. Any credentials come only from Secrets. No `noProxy` entry is empty or contains a comma. | ✅ |
| — (extends `proxyCheck`) | `spec.global.proxy` | static | `noProxy` covers in-cluster addresses (advisory). The operator already adds the in-cluster entries itself, so this is a low priority. | |
| `customCACertsCheck` | `spec.global.customCACerts` | static | Every entry is complete PEM containing only `CERTIFICATE` blocks. Every certificate parses and is a CA, and each entry has at least one currently valid CA. Expired cross-signs are reported but don't fail the check. | ✅ |
| — | `spec.global.caCertsConfigMap` | cluster | The ConfigMap exists and every certificate in it parses as PEM. | |
| — | `spec.global.imagePullSecrets` | cluster | Each referenced Secret exists and is of type `kubernetes.io/dockerconfigjson`. | |
| — | `spec.wandb.serviceAccount` | cluster | When `create: false`, the named ServiceAccount exists. | |

## W&B server

| Check | CR field | Type | A pass confirms | Implemented |
|---|---|---|---|---|
| `serverManifestCheck` | `spec` | connection | The manifest for `spec.wandb.version` is published in the effective manifest repository: `spec.wandb.manifestRepository`, or the one derived from `spec.global.imageRegistry`. When an operator version is given, it also satisfies the manifest's `requiredOperatorVersion`. | ✅ |
