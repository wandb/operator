# Running preflight checks with wsm

Preflight checks validate a `WeightsAndBiases` (`apps.wandb.com/v2`) CR **before you apply it**: they confirm that external databases, caches and buckets are reachable with the credentials in the CR, that the dependency operators for managed services are installed, that proxy and CA settings are well formed, and that the requested W&B server version is published and compatible with your operator.

The checks are defined in this repo (`pkg/preflight`) and run by [`wsm`](https://github.com/wandb/wsm) against your **current kubectl context**, so you can run them on a laptop against a local Kind cluster or against a production cluster before an install or upgrade.

This guide covers the **local checks**: the checks `wsm` runs from your own machine.

## Local checks vs. in-cluster checks

Preflight checks fall into two groups, depending on where they run:

| | Local checks | In-cluster checks |
|---|---|---|
| Runs from | The machine running `wsm` | A preflight pod that `wsm` creates in your cluster |
| Check types | `static`, `connection`, and read-only `cluster` | Checks that need the cluster's network or the cluster spec |
| Cluster access needed | None (`--offline`), or read access through your kubeconfig for Secrets and API discovery | Permission to create the preflight pod in the target namespace, plus read access to the cluster spec |
| Changes the cluster | No | Yes: creates a preflight pod in the cluster (in the target namespace) |
| Status | Available; the 11 checks listed below | Planned |

**Every check available today is a local check.** None of them creates or modifies anything in your cluster.

In-cluster checks will verify what only the cluster can see: endpoints reachable only from the cluster network, node and storage capacity, and ambient credentials such as workload identity. To run them, `wsm` will create a preflight pod in your cluster, so they need permission to create resources. Before creating anything, `wsm` lists the checks that create resources (access `resourceCreate`) and asks for confirmation. If you decline, it exits without running any check. Until in-cluster checks are available, see [Running from your machine vs. from the cluster](#running-from-your-machine-vs-from-the-cluster) for workarounds.

## Which operator's checks run?

`wsm` compiles in the operator's `pkg/preflight` package, so the checks that run are the ones from the operator version `wsm` was built against, not the operator installed in your cluster. To validate a CR for a specific operator release:

1. Use a `wsm` build that pins that operator version (check `github.com/wandb/operator` in wsm's `go.mod`), and
2. Pass `--operator-version <version>` so the server manifest check verifies that the W&B server version in the CR supports that operator.

## Install wsm

Requires Go 1.27.3+.

```bash
git clone https://github.com/wandb/wsm
cd wsm
make build
./wsm preflight --help
```

If `wsm preflight` reports `unknown flag: --operator-version` (or doesn't exist), you are running an older `wsm`; check `which wsm` and use the build above.

## Quick start

```bash
# 1. Point kubectl at the cluster you plan to deploy to
kubectl config use-context <your-context>

# 2. Run every check that applies to your CR
wsm preflight wandb.yaml --operator-version 2.0.0-beta.10
```

Example output:

```
Preflight checks found:
  customCACertsCheck (spec.global.customCACerts) [type: static, access: readOnly]
  externalMysqlCheck (spec.mysql.default.externalMysql) [type: connection, access: readOnly]
  managedRedisApisCheck (spec.redis.default.managedRedis) [type: cluster, access: readOnly]
  serverManifestCheck (spec) [type: connection, access: readOnly]

[PASS] customCACertsCheck (spec.global.customCACerts): 1 certificates
[PASS] externalMysqlCheck (spec.mysql.default.externalMysql): ...
[FAIL] managedRedisApisCheck (spec.redis.default.managedRedis): API not served: redis.redis.opstreelabs.in/v1beta2 Redis (install the OpsTree redis-operator)
[PASS] serverManifestCheck (spec): manifest 0.86.0 found in oci://...; operator 2.0.0-beta.10 satisfies ^2.0.0
```

`wsm` first lists every check it found and then runs them. It exits non-zero if any check **fails** or is **unknown**, so you can use it as a gate in CI or in an install script.

## Flags

| Flag | Default | Description |
|---|---|---|
| `<cr-file>` (argument) | — | Path to the v2 CR YAML. Required. |
| `--context` | current kubectl context | Kubeconfig context used for cluster checks and for reading Secrets. |
| `--wandb-namespace` | the CR's `metadata.namespace` | Namespace the CR will be deployed to. Secret references are read from here. |
| `--operator-version` | not checked | Operator version the CR will run under, checked against the server manifest's `requiredOperatorVersion`. |
| `--offline` | `false` | Never contact the cluster: cluster checks are skipped and Secrets aren't read. Static and connection checks still run. |

### Context and namespace

Without `--context`, `wsm` uses your current context (`kubectl config current-context`), the same way `kubectl` does. Pass `--context` to target a different cluster without switching:

```bash
wsm preflight wandb.yaml --context prod-us-east --wandb-namespace wandb
```

The cluster is used for two things only:

- **Cluster checks** (`managed*ApisCheck`) use API discovery to confirm that the CRDs the operator will create are served.
- **Secret-backed values** (`valueFrom.secretKeyRef`) in the CR are read from `--wandb-namespace` so connection checks can use the real credentials.

All of today's checks are read-only and don't create or modify anything in the cluster. In-cluster checks (planned) will create a preflight pod, and `wsm` will ask before creating it.

## Local checks

These are the checks available today. All of them run from the machine running `wsm`. A check runs when its CR field is present in your file. `*` matches every instance name in the `mysql`, `redis`, `objectStore` and `clickhouse` maps, so a CR with two MySQL instances runs the MySQL check twice.

| Check | CR field | Type | What it verifies |
|---|---|---|---|
| `externalMysqlCheck` | `spec.mysql.*.externalMysql` | connection | Connects with the CR's host, port, database, user, password and TLS settings. |
| `externalRedisCheck` | `spec.redis.*.externalRedis` | connection | Connects (with TLS/CA if set) and reports the Redis version. |
| `externalObjectStoreCheck` | `spec.objectStore.*.externalObjectStore` | connection | S3/S3-compatible: signed `HeadBucket` (bucket exists, credentials work, region matches). GCS: the service account can list objects. Azure: the account key works and the container exists. |
| `externalClickHouseCheck` | `spec.clickhouse.*.externalClickhouse` | connection | Queries over HTTP, checks the native TCP port, and when replicated, that `clusterName` exists and Keeper/ZooKeeper is reachable. |
| `managedMysqlApisCheck` | `spec.mysql.*.managedMysql` | cluster | MOCO `MySQLCluster` API is served. |
| `managedRedisApisCheck` | `spec.redis.*.managedRedis` | cluster | OpsTree `Redis` API is served (`RedisReplication` + `RedisSentinel` in sentinel mode). |
| `managedObjectStoreApisCheck` | `spec.objectStore.*.managedObjectStore` | cluster | SeaweedFS `Seaweed` API is served. |
| `managedClickHouseApisCheck` | `spec.clickhouse.*.managedClickhouse` | cluster | Altinity `ClickHouseInstallation` and `ClickHouseKeeperInstallation` APIs are served. |
| `proxyCheck` | `spec.global.proxy` | static | Proxy URLs are valid `http`/`https` URLs (including Secret-backed ones) and `noProxy` entries are non-empty with no commas. |
| `customCACertsCheck` | `spec.global.customCACerts` | static | Every entry is well-formed PEM containing only CA certificates, with at least one currently valid. |
| `serverManifestCheck` | `spec` | connection | The manifest for `spec.wandb.version` exists in the manifest repository (`spec.wandb.manifestRepository`, or derived from `spec.global.imageRegistry`), and, with `--operator-version`, that the operator satisfies its `requiredOperatorVersion`. |

Check types:

- **static**: validates the CR's content only.
- **cluster**: makes read-only queries to the Kubernetes API of the target context through your kubeconfig. No pod is created.
- **connection**: opens a network connection from the machine running `wsm` to an external service.

To run only the checks that don't touch the cluster at all (`static` and `connection`), use [`--offline`](#offline-mode).

## Reading results

| Outcome | Meaning |
|---|---|
| `PASS` | The prerequisite is satisfied right now. |
| `FAIL` | The prerequisite is not met; the message says what to fix. |
| `UNKNOWN` | The check couldn't decide (for example, discovery was forbidden or a registry returned an unexpected status). Treated as a failure for the exit code. |
| `SKIPPED` | Not run: skipped by annotation, a cluster check in `--offline` mode, or a Secret that couldn't be read. |

A passing preflight means the checked prerequisites hold at that moment. It doesn't guarantee that scheduling, migrations or application startup will succeed.

## Secrets

Credentials can be literal values or Secret references:

```yaml
password:
  valueFrom:
    secretKeyRef: {name: mysql-credentials, key: password}
```

Referenced Secrets must already exist in `--wandb-namespace` on the target cluster, and your kubeconfig user needs `get` on them. With `--offline`, Secret-backed values aren't read and the checks that need them report `SKIPPED`. Secret values are never printed; error messages are redacted.

## Running from your machine vs. from the cluster

Connection checks run **from the machine running `wsm`**, not from inside the cluster. An endpoint that only W&B pods can reach (a `*.svc` address, a private VPC host, a database behind a firewall that only allows cluster egress) may `FAIL` here even though the deployment would work. In-cluster checks (planned) will cover these endpoints by running from a preflight pod in the cluster. Until then:

- Run `wsm` from a host on the same network as the cluster (a bastion or jump host).
- Use a temporary copy of the CR whose hosts point to a `kubectl port-forward` or SSH tunnel.
- Skip that check with the annotation below and validate it separately.

Some checks can only be fully verified from inside the cluster and say so in their message, for example object stores that use ambient credentials or workload identity (no `accessKey`/`secretKey` in the CR). Also, CAs from `spec.global.customCACerts` are not applied to the object store check, so a private-CA endpoint may report as untrusted here.

## Offline mode

```bash
wsm preflight wandb.yaml --offline
```

`--offline` never contacts the cluster, which is useful before the cluster exists or when you don't have credentials for it yet. Static checks, connection checks with literal credentials, and the server manifest check still run.

## Skipping checks

Add the `skipPreflights` annotation with a comma-separated list of check names:

```yaml
metadata:
  annotations:
    skipPreflights: externalObjectStoreCheck,externalClickHouseCheck
```

Skipped checks report `SKIPPED` and don't affect the exit code.

## Notes

- **Only what's in the file is checked.** `wsm preflight` reads the CR as written and doesn't apply the operator's defaulting webhook. If you omit `spec.mysql`, the webhook would add managed MySQL at apply time, but `managedMysqlApisCheck` won't run. Spell out each backend to have it checked.
- **Registry credentials.** The server manifest is pulled with your Docker credentials (`docker login`). For a private or mirrored registry, log in first or the check reports that it requires registry credentials.
- Only `apps.wandb.com/v2` CRs are supported.

## Example: every check against a local Kind cluster

This CR triggers all 11 checks. It defines a managed (`default`) and an external (`ext`) instance for each backend. Replace hosts, ports and credentials with your own, or point them at port-forwards to a local Tilt environment.

```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
  namespace: wandb
spec:
  wandb:
    hostname: http://localhost:8080
    version: "0.86.0"
  global:
    proxy:
      httpsProxy:
        value: http://proxy.example.com:3128
      noProxy:
        - .corp.example.com
    customCACerts:
      - |
        -----BEGIN CERTIFICATE-----
        ...your CA certificate...
        -----END CERTIFICATE-----
  mysql:
    default:
      managedMysql: {}
    ext:
      externalMysql:
        host: {value: 127.0.0.1}
        port: {value: "13306"}
        database: {value: wandb_local}
        username: {value: wandb}
        password:
          valueFrom:
            secretKeyRef: {name: preflight-test, key: mysql-password}
  redis:
    default:
      managedRedis: {}
    ext:
      externalRedis:
        host: {value: 127.0.0.1}
        port: {value: "16379"}
  objectStore:
    default:
      managedObjectStore: {}
    ext:
      externalObjectStore:
        provider: {value: s3}
        endpoint: {value: 127.0.0.1}
        port: {value: "18333"}
        bucket: {value: wandb}
        region: {value: us-east-1}
        tlsEnabled: {value: "false"}
        forcePathStyle: {value: "true"}
        accessKey: {value: seaweedfs}
        secretKey:
          valueFrom:
            secretKeyRef: {name: preflight-test, key: s3-secret-key}
  clickhouse:
    default:
      managedClickhouse: {}
    ext:
      externalClickhouse:
        host: {value: 127.0.0.1}
        httpPort: {value: "18123"}
        tcpPort: {value: "19000"}
        database: {value: wandb}
        username: {value: wandb}
        password: {value: wandb}
```

Generate a test CA for `customCACerts`:

```bash
openssl req -x509 -newkey rsa:2048 -nodes -keyout /dev/null -out preflight-ca.crt -days 30 -subj "/CN=preflight-test-ca" -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign"
```

Create the Secret the CR references:

```bash
kubectl -n wandb create secret generic preflight-test --from-literal=mysql-password=<password> --from-literal=s3-secret-key=<secret-key>
```

Run the checks:

```bash
wsm preflight wandb.yaml --wandb-namespace wandb --operator-version 2.0.0-beta.10
```
