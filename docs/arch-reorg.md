# Architecture: Separation of Concerns Analysis

## `internal/controller/{infra,translator,v2}` · `api/v2`

---

## 1. Intended Architecture (Inferred)

The system is designed as a layered pipeline from user intent to vendor infrastructure:

```
api/v2              CRD schema — what the user writes in YAML
translator/         Internal wire types (connection shapes, status shapes)
translator/v2/      Adaptation layer: api/v2 ↔ translator, translator → vendor CRs
infra/external/     External credential management (user-provided services)
infra/managed/      Vendor-specific lifecycle management (operator-deployed services)
controller/v2/      Reconciliation orchestration and dispatching
```

The intended direction of dependency is strictly downward: `controller/v2` → `infra/` → `translator/` → `api/v2`. Nothing in the lower layers should call back up.

---

## 2. Layer-by-Layer Analysis

### `api/v2`

**Intended:** CRD schema only — the types a cluster admin writes in YAML.

**Actual:** `application_types.go` imports KEDA (`kedav1alpha1.ScaledObjectSpec`) and Argo Rollouts (`v1alpha1.RolloutStatus`) directly. These types appear in `ApplicationSpec.ScaledObjectTemplate` and `ApplicationStatus.RolloutStatus`.

**Issue:** CRD types should not encode workload-scheduling implementation concerns. `ApplicationSpec` embeds a full `ScaledObjectSpec` from KEDA and a `RolloutStatus` from Argo, coupling the CRD schema to two external operator dependencies. If either operator changes its API, the CRD schema is forced to change. These types are workload orchestration details — they belong in the manifest layer or in the translator, not in the API surface that cluster admins interact with.

---

### `internal/controller/translator/`

**Intended:** Pure data shapes for internal use.

**Actual:** Exactly that. This package contains only type definitions:
- `InfraStatus` (`common.go:13`) — base status embedded by all infra status types
- `MysqlConnection`, `MysqlStatus` (`mysql.go`)
- `RedisConnection`, `RedisStatus` (`redis.go`)
- `KafkaConnection`, `KafkaStatus` (`kafka.go`)
- `ObjectStoreConnection`, `ObjectStoreStatus` (`objectstore.go`)
- `ClickHouseConnection`, `ClickHouseStatus` (`clickhouse.go`)
- `OnDeletePolicy`, `OnDeleteRule` (`retention.go`)

**Assessment:** Well-designed. This package serves as the lingua franca between the `infra/` layer and `controller/v2/`. No conversion logic lives here, which keeps it stable and easy to import without pulling in external dependencies.

---

### `internal/controller/translator/v2/`

**Intended:** Adaptation layer between `api/v2` types and `translator` types, plus vendor CR construction.

**Actual:** This package has two distinct responsibilities bundled together:

1. **Bidirectional type converters** (`common.go`) — field-for-field mappings between `api/v2` and `translator` types, e.g. `ToTranslatorMysqlConnection()` / `ToWbMysqlInfraStatus()`. These are shallow structural transforms with no business logic.

2. **Vendor spec builders** (per-service files) — functions that construct fully-formed vendor CRs: `ToMysqlMySQLVendorSpec()` → `*InnoDBCluster`, `ToKafkaVendorSpec()` → `*v1.Kafka`, `ToRedisStandaloneVendorSpec()` → `*redisv1beta2.Redis`, `ToObjectStoreVendorSpec()` → `*miniov2.Tenant`, `ToClickHouseVendorSpec()` → `*v1.ClickHouseInstallation`.

These are two very different jobs. Type converters are stable glue code; vendor spec builders contain real business logic (resource sizing, telemetry wiring, HA topology selection). They should be separated — the spec builders logically belong closer to the managed infra layer that uses them, or in a `translator/v2/vendor/` sub-package.

**Additional issue:** Configuration defaults are hardcoded in this package:
- `DefaultSentinelGroup = "gorilla"` (`redis.go:24`)
- Default sentinel replica count: `3` (`redis.go:145`)
- Default replication replica count: `3` (`redis.go:226`)

Defaults belong in the spec (as `+kubebuilder:default` annotations) or in a constants file closer to the spec definition, not in the translation layer where they are invisible to API consumers.

---

### `internal/controller/infra/external/`

**Intended:** Manage credentials for user-provided (external) services.

**Actual:** Each service sub-package (`redis/`, `mysql/`, `kafka/`, `clickhouse/`, `objectstore/`) follows a consistent pattern:
- `WriteState()` — resolves user-specified secret references, normalizes them into a single connection secret
- `ReadState()` — reads the connection secret back, returns a `translator.XxxConnection`
- `DeleteConnectionSecret()` — cleans up on deletion

Shared utilities live in `external/common.go`: `ResolveSecretKey`, `WriteConnectionSecret`, `ReadConnectionSecret`, `ResolveFields`.

**Issue:** `external/common.go:InferExternalStatus()` (line 134) computes the health state of an external service based on whether a connection was established. This is status-inference logic — it determines `state`, `ready`, and merges conditions. This function is called from `controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go`'s `externalXxxInferStatus()` functions, not from within the external infra layer itself. It belongs closer to where it's called (the `controller/v2` layer), not in the external credential management layer.

---

### `internal/controller/infra/managed/{vendor}/`

**Intended:** Vendor-specific CR lifecycle: create/update, read status, manage finalizers.

**Actual:** Each vendor sub-package (opstree Redis, mysql MySQL, strimzi Kafka, altinity ClickHouse, minio Tenant) follows a consistent file structure: `naming.go`, `conn.go`, `read.go`, `write.go`, `detach.go`, `purge.go`, `status.go`.

**Issue 1 — Writes during reads (most significant):**

`read.go` in every managed infra package calls the `writeXxxConnInfo()` function from `conn.go` to create or update the connection Secret. Concretely:

- `managed/redis/opstree/read.go:108` — `connection, err = writeRedisConnInfo(ctx, client, wandbOwner, nsnBuilder, connInfo)`
- `managed/redis/opstree/read.go:149` — same call for sentinel mode
- `managed/mysql/mysql/read.go` — equivalent `writeMySQLConnInfo()` call
- `managed/minio/tenant/conn.go:writeWandbConnInfo()` — called from `write.go`, but returns the connection reference

The naming `ReadState` implies a pure observation. Callers (in `controller/v2/`) call `WriteState` then `ReadState` expecting the latter to be idempotent and side-effect-free. Instead, `ReadState` unconditionally creates or overwrites a Kubernetes Secret on every reconcile. The connection secret write is logically a consequence of the managed service being ready, which is a write concern — it should move to `WriteState` or be explicitly named (e.g. `EnsureConnectionSecret`).

**Issue 2 — `write.go` bundles label patching:**

Redis's `write.go` contains `ensurePVCLabels()` and `ensurePodLabels()` (lines 84–155) alongside the vendor CR write. MySQL's `write.go` similarly patches `datadir-{clusterName}-*` PVCs. Label injection on existing Kubernetes objects (Pods, PVCs) is a cross-cutting concern orthogonal to CR lifecycle. It could be a shared utility in `internal/controller/common/` or extracted to a `labels.go` file in each package.

**Assessment (well-placed):** `status.go` in each vendor package is correctly placed — it computes `InfraStatus` from observed conditions, events, and readiness. This is vendor-adjacent logic that correctly lives close to the vendor.

---

### `internal/controller/v2/`

**Intended:** Top-level orchestration and dispatch — route to managed vs. external infra, sequence reconciliation steps.

**Actual:** The per-component dispatcher files (`redis.go`, `mysql.go`, `kafka.go`, `clickhouse.go`, `objectstore.go`) are well-structured: each exposes `{component}WriteState`, `{component}ReadState`, `{component}InferStatus`, `{component}PurgeFinalizer`, `{component}DetachFinalizer`.

**Issue 1 — Password generation in the dispatcher (`mysql.go:102–168`):**

`managedMysqlWriteState` generates a random root and user password, creates a `{name}-db-password` Secret, and only then calls `mysql.WriteState`. This is business logic — generating and persisting database credentials — sitting in the orchestration/dispatch layer. The managed infra layer (`infra/managed/mysql/mysql/`) does not know about these credentials: `write.go` receives a fully-constructed `InnoDBCluster` spec that already references the secret by name. The credential generation belongs in `infra/managed/mysql/mysql/`, ideally in `write.go` or a new `credentials.go`, so that the credential lifecycle is co-located with the resource lifecycle.

**Issue 2 — Telemetry types in the controller package (`telemetry_config.go`):**

`TelemetryRuntimeConfig`, `TelemetryOTelConfig`, `TelemetryEndpoints`, `TelemetryOTelConfig` are domain types that describe the telemetry configuration system. They are not controller-specific plumbing — they are configuration domain concepts. They belong in `pkg/telemetry/` or `internal/telemetry/`, where they could be imported by both the controller and any other package that needs to reason about telemetry configuration.

**Issue 3 — Hard-coded application allowlist (`reconcile_v2.go:58–72`):**

`managedWorkloadTelemetryApplications` is a `map[string]struct{}` listing application names that receive telemetry env vars (`api`, `executor`, `weave`, etc.). This is configuration-as-code embedded in the reconciler. When a new application is added to the manifest, this map must also be updated. It belongs in the server manifest layer (`pkg/wandb/manifest/`), not hardcoded in the orchestrator.

**Issue 4 — `reconcile_v2.go` size and scope:**

The main reconcile file handles: finalizer dispatch, infra readiness gating, secret generation, Kafka topic creation, MySQL init jobs, RBAC (ServiceAccount/Role/RoleBinding), Gateway reconciliation, migration jobs, application deployment, telemetry injection, and HTTPRoute reconciliation. These are logically separable concerns each deserving their own file.

---

## 3. Mutation Access Map

| Object | Who Writes | Who Reads Back | Verdict |
|--------|-----------|---------------|---------|
| `WeightsAndBiases.Status` | `controller/v2/*.go` via `client.Status().Update()` | Same package (next reconcile) | Correct — single writer |
| Connection Secrets | `infra/managed/*/conn.go` (from `ReadState`) | `infra/managed/*/read.go` returns pointer to secret keys | Naming violation — write in read path |
| External Connection Secrets | `infra/external/common.go:WriteConnectionSecret` | `infra/external/*/ReadState` → returns `translator.XxxConnection` | Correct |
| db-password Secret | `controller/v2/mysql.go:managedMysqlWriteState` | `translator/v2/mysql.go:ToMysqlMySQLVendorSpec` (reads secret name from spec) | Wrong layer — should be in managed infra |
| Vendor CRs (Redis/MySQL/Kafka/etc.) | `infra/managed/*/write.go` | `infra/managed/*/read.go` (reads status) | Correct |
| PVC/Pod labels | `infra/managed/*/write.go` (`ensurePVCLabels`, `ensurePodLabels`) | Not read back programmatically | Orthogonal to CR write — should be a shared util |
| HTTPRoutes | `controller/v2/infra_routes.go` | Not read back (Gateway controller handles them) | Correct |
| RBAC resources | `controller/v2/reconcile_v2.go` | Not read back | Correct |

---

## 4. Downstream Mutation Reads (Cross-Reconcile Data Flow)

The reconcile loop is stateful across ticks via the `WeightsAndBiases.Status` subresource:

```
Tick N:
  {component}WriteState  →  creates/updates vendor CR in Kubernetes
  {component}ReadState   →  reads vendor CR status
                         →  WRITES connection Secret (side effect)
                         →  returns translator.XxxConnection (pointer into secret)
  {component}InferStatus →  reads wandb.Status.XxxStatus (from previous tick)
                         →  merges old + new conditions
                         →  writes wandb.Status.XxxStatus via Status().Update()

Tick N+1:
  {component}InferStatus →  reads wandb.Status.XxxStatus (written in tick N)
                         →  ToTranslatorMysqlConnection(wandb.Status.MySQLStatus.Connection)
                         →  uses as fallback if ReadState returned nil connection
  ReconcileWandbManifest →  reads wandb.Status.*.Ready to gate app deployment
```

The status-as-fallback pattern means that if `ReadState` returns `nil` (e.g. the vendor CR is temporarily unavailable), `InferStatus` falls back to the connection from the previous tick's status. This is intentional and correct — the `utils.Coalesce(newInfraConn, &oldInfraConn)` calls in each `InferStatus` function implement this.

The pattern also means the `WeightsAndBiases.Status.XxxStatus.Connection` fields store `SecretKeySelector` references — they point to the connection Secret written by `ReadState`. The Status subresource is acting as a cache/index of connection pointers across reconcile ticks, not just a reporting surface. This is a subtle but important design decision that should be documented.

---

## 5. Business Logic Placement

### Well-placed

- `managed/*/status.go` — vendor-specific condition-to-status mapping belongs close to the vendor
- `managed/*/detach.go`, `managed/*/purge.go` — finalizer logic correctly encapsulated per vendor
- `managed/*/naming.go` — resource naming strategy correctly isolated
- `translator/v2/common.go` — bidirectional converters are shallow, stable, and correctly placed
- `controller/v2/reconcile_v2.go:runRetentionFinalizer()` — retention policy dispatch is an orchestration concern
- `controller/v2/infra_routes.go` — Gateway API HTTPRoute management is correctly at the orchestration level

### Misplaced

| Logic | Current Location | Should Be |
|-------|-----------------|-----------|
| MySQL credential generation | `controller/v2/mysql.go:102–168` | `infra/managed/mysql/mysql/write.go` or `credentials.go` |
| Connection secret writes | `infra/managed/*/conn.go` called from `read.go` | Should be called from `write.go` or renamed to `EnsureConnectionSecret` |
| Telemetry domain types | `controller/v2/telemetry_config.go` | `pkg/telemetry/` or `internal/telemetry/` |
| Application telemetry allowlist | `controller/v2/reconcile_v2.go:58–72` | `pkg/wandb/manifest/` or manifest definition |
| Sentinel group name default `"gorilla"` | `translator/v2/redis.go:24` (also `conn.go:31`) | `api/v2` kubebuilder default annotation or spec constant |
| Sentinel/replication replica defaults | `translator/v2/redis.go:145,226` | `api/v2` kubebuilder default annotations |
| External status inference | `infra/external/common.go:InferExternalStatus` | `controller/v2/` (called only from there) |

---

## 6. File Organization Issues

### `managed/*/conn.go`

This file currently does two things: parses connection details out of a vendor CR (`readStandaloneConnectionDetails`, `readSentinelConnectionDetails`) and writes a Kubernetes Secret (`writeRedisConnInfo`, `writeMySQLConnInfo`). The parsing is a read concern; the secret write is a write concern. Because `writeRedisConnInfo` is called from `read.go`, the read/write boundary is already crossed. The fix is to move the `writeXxxConnInfo` call into `write.go` (or a new `secrets.go`) and have it return the connection reference, which `read.go` can then just read back. Alternatively, rename `ReadState` to `EnsureState` to signal that it has side effects.

### `managed/*/write.go`

PVC and Pod label patching (`ensurePVCLabels`, `ensurePodLabels`) is bundled in the same file as vendor CR writes. These are label-maintenance operations on resources created by the vendor operator, not by this controller. They belong in a shared `internal/controller/common/labels.go` or a per-package `labels.go`.

### `controller/v2/reconcile_v2.go`

This file orchestrates: finalizers, infra readiness gating, secret generation, Kafka topics, MySQL init, RBAC, Gateway, migrations, application deployment, telemetry injection, and infra HTTPRoutes. Suggested splits:

- `rbac.go` — ServiceAccount, Role, RoleBinding
- `migrations.go` — migration job orchestration  
- `kafka_topics.go` — Kafka topic creation
- `secrets.go` — application secret generation
- `applications.go` — application deployment loop (already large enough)

### `translator/v2/`

The vendor spec builders (`ToMysqlMySQLVendorSpec`, `ToKafkaVendorSpec`, etc.) import all vendored operator CRDs. The type converters in `common.go` import only `api/v2` and `translator`. These should be in separate sub-packages (`translator/v2/convert/` and `translator/v2/vendor/`) or the vendor spec builders should move to the managed infra layer that uses them.
