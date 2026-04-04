# Architecture Reorganization Plan

Based on the analysis in [arch-reorg.md](./arch-reorg.md).

---

## Current State

```
api/v2/
  application_types.go        <-- imports KEDA + Argo Rollouts directly
  weightsandbiases_types.go

internal/controller/
  translator/
    common.go                  <-- pure data shapes (good)
    mysql.go, redis.go, ...
    utils/
      resources.go
    v2/
      common.go                <-- type converters (good)
      mysql.go                 <-- vendor spec builder + defaults (mixed)
      redis.go                 <-- vendor spec builder + hardcoded defaults (mixed)
      kafka.go                 <-- vendor spec builder (mixed)
      objectstore.go           <-- vendor spec builder (mixed)
      clickhouse.go            <-- vendor spec builder (mixed)
      retention.go

  infra/
    external/
      common.go                <-- InferExternalStatus (wrong layer)
      redis/redis.go
      mysql/mysql.go
      kafka/kafka.go
      clickhouse/clickhouse.go
      objectstore/objectstore.go
    managed/
      redis/opstree/
        read.go                <-- calls writeRedisConnInfo (write-in-read)
        conn.go                <-- writeRedisConnInfo
        write.go               <-- ensurePVCLabels, ensurePodLabels (cross-cutting)
        status.go, naming.go, detach.go, purge.go, values.go
      mysql/mysql/
        read.go                <-- calls writeMySQLConnInfo (write-in-read)
        conn.go                <-- writeMySQLConnInfo
        write.go
        status.go, naming.go, detach.go, purge.go
      kafka/strimzi/
        read.go, conn.go, write.go, restore.go
        status.go, naming.go, detach.go, purge.go, values.go
      clickhouse/altinity/
        read.go, conn.go, write.go
        status.go, naming.go, detach.go, purge.go, values.go
      minio/tenant/
        read.go, conn.go, write.go, config.go
        status.go, naming.go, detach.go, purge.go

  v2/
    reconcile_v2.go            <-- monolith: RBAC, secrets, kafka topics,
                                   migrations, app deploy, telemetry inject,
                                   hardcoded telemetry allowlist (lines 58-72)
    mysql.go                   <-- credential generation (lines 102-168)
    redis.go, kafka.go, clickhouse.go, objectstore.go
    telemetry_config.go        <-- domain types in controller layer
    telemetry_secret.go
    gateway.go, infra_routes.go, ingress.go
```

---

## Proposed Changes

Ten moves, grouped into three phases by risk and dependency.

---

### Phase 1 — Pure moves (no behavioral change)

These relocate code without altering any function signatures or call semantics. Tests should pass identically before and after.

#### Move 1: Extract vendor spec builders from `translator/v2/` to `infra/managed/`

**Why:** `translator/v2/` bundles two unrelated jobs: shallow type converters and vendor-specific spec construction. The spec builders import all vendored CRDs and contain real business logic (sizing, HA topology). They belong next to the managed infra code that calls them.

```
BEFORE                                  AFTER
translator/v2/                          translator/v2/
  common.go    (converters)               common.go    (converters)
  mysql.go     (converter + vendor spec)  mysql.go     (converter only)
  redis.go     (converter + vendor spec)  redis.go     (converter only)
  kafka.go     (converter + vendor spec)  kafka.go     (converter only)
  objectstore.go  (same)                  objectstore.go  (converter only)
  clickhouse.go   (same)                  clickhouse.go   (converter only)
                                          retention.go (unchanged)
                                        
                                        infra/managed/
                                          mysql/mysql/spec.go       <-- ToMysqlMySQLVendorSpec
                                          redis/opstree/spec.go     <-- ToRedis*VendorSpec
                                          kafka/strimzi/spec.go     <-- ToKafkaVendorSpec
                                          minio/tenant/spec.go      <-- ToObjectStoreVendorSpec
                                          clickhouse/altinity/spec.go <-- ToClickHouseVendorSpec
```

**Files touched:**
- `translator/v2/{mysql,redis,kafka,objectstore,clickhouse}.go` — remove vendor spec builder functions
- New `infra/managed/*/spec.go` — receive vendor spec builder functions
- `infra/managed/*/write.go` — update imports (callers of spec builders)

---

#### Move 2: Move telemetry domain types to `internal/telemetry/`

**Why:** `TelemetryRuntimeConfig`, `TelemetryOTelConfig`, `TelemetryEndpoints` are configuration domain concepts, not controller plumbing. Other packages may need to reason about telemetry configuration.

```
BEFORE                              AFTER
controller/v2/                      controller/v2/
  telemetry_config.go  (types)        telemetry_secret.go (unchanged)
  telemetry_secret.go                 (imports internal/telemetry)

                                    internal/telemetry/       <-- NEW
                                      config.go              <-- types moved here
```

**Files touched:**
- `controller/v2/telemetry_config.go` — delete (or reduce to re-exports if needed transitionally)
- New `internal/telemetry/config.go` — receive type definitions
- `controller/v2/telemetry_secret.go`, `reconcile_v2.go` — update imports

---

#### Move 3: Move `InferExternalStatus` to `controller/v2/`

**Why:** This function is defined in `infra/external/common.go` but called exclusively from `controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go`. It computes health state from connection status — an orchestration concern, not a credential-management concern.

```
BEFORE                                  AFTER
infra/external/                         infra/external/
  common.go                               common.go
    ResolveSecretKey       (stays)          ResolveSecretKey
    WriteConnectionSecret  (stays)          WriteConnectionSecret
    ReadConnectionSecret   (stays)          ReadConnectionSecret
    ResolveFields          (stays)          ResolveFields
    InferExternalStatus    (MOVES) ---+
                                      |
controller/v2/                        | controller/v2/
  redis.go                            |   external_status.go  <-- NEW
  mysql.go                            +-> InferExternalStatus
  ...                                     (called from redis.go, mysql.go, etc.)
```

**Files touched:**
- `infra/external/common.go` — remove `InferExternalStatus`
- New `controller/v2/external_status.go` — receive function
- `controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go` — update call sites (package-local now)

---

#### Move 4: Move telemetry application allowlist to manifest layer

**Why:** `managedWorkloadTelemetryApplications` (reconcile_v2.go:58-72) is a hardcoded list of app names that must be updated whenever a new application is added to the manifest. It belongs with the manifest definition.

```
BEFORE                                  AFTER
controller/v2/                          controller/v2/
  reconcile_v2.go                         reconcile_v2.go
    managedWorkloadTelemetryApplications    (imports from manifest)
    = map[string]struct{}{
      "api": {}, "executor": {}, ...    pkg/wandb/manifest/
    }                                     telemetry.go  <-- NEW
                                            ManagedWorkloadTelemetryApplications
```

**Files touched:**
- `controller/v2/reconcile_v2.go` — remove map, import from `pkg/wandb/manifest`
- New `pkg/wandb/manifest/telemetry.go` — receive map

---

### Phase 2 — Semantic corrections (behavioral clarification)

These change the call graph to fix naming/boundary violations. Require careful testing.

#### Move 5: Fix write-in-read — relocate connection secret writes to `WriteState`

**Why:** Every `managed/*/read.go:ReadState` silently creates/overwrites a Kubernetes Secret via `writeXxxConnInfo()`. Callers expect `ReadState` to be idempotent. The write belongs in the write path.

```
BEFORE (per managed vendor)             AFTER (per managed vendor)

write.go                                write.go
  WriteState()                            WriteState()
    creates/updates vendor CR               creates/updates vendor CR
                                            calls writeXxxConnInfo() if CR ready
                                            returns connection ref
read.go                                 read.go
  ReadState()                             ReadState()
    reads vendor CR status                  reads vendor CR status
    calls writeXxxConnInfo()  <-- BAD       reads connection secret  <-- pure read
    returns connection                      returns connection
conn.go                                 conn.go
  writeXxxConnInfo()                      writeXxxConnInfo()  (called from write.go)
  readXxxConnectionDetails()              readXxxConnectionDetails()  (called from read.go)
```

**Files touched per vendor (redis/opstree, mysql/mysql, kafka/strimzi, clickhouse/altinity, minio/tenant):**
- `read.go` — remove `writeXxxConnInfo` call, read secret instead
- `write.go` — add `writeXxxConnInfo` call after vendor CR is ready, return connection ref
- Possibly adjust `ReadState`/`WriteState` return types

**Risk:** This changes the reconcile timing of when connection secrets are written. Must verify that `WriteState` has sufficient info to write the secret at that point.

---

#### Move 6: Move MySQL credential generation to managed infra layer

**Why:** `controller/v2/mysql.go:102-168` generates random passwords and creates a `{name}-db-password` Secret. This is resource-lifecycle logic sitting in the orchestration layer. The managed MySQL package doesn't know about these credentials.

```
BEFORE                                  AFTER
controller/v2/mysql.go                  controller/v2/mysql.go
  managedMysqlWriteState()                managedMysqlWriteState()
    generate root password                  calls mysql.WriteState()
    generate user password                    (credentials handled internally)
    create db-password Secret
    calls mysql.WriteState()            infra/managed/mysql/mysql/
                                          credentials.go  <-- NEW
infra/managed/mysql/mysql/                  EnsureCredentials()
  write.go                                    generate passwords if secret missing
    WriteState()                              create/read db-password Secret
    (receives pre-built spec)             write.go
                                            WriteState()
                                              calls EnsureCredentials()
                                              builds spec with secret ref
```

**Files touched:**
- `controller/v2/mysql.go` — remove credential generation (~65 lines)
- New `infra/managed/mysql/mysql/credentials.go` — receive credential logic
- `infra/managed/mysql/mysql/write.go` — call `EnsureCredentials`

---

#### Move 7: Relocate hardcoded defaults to `api/v2` or constants

**Why:** Defaults like `DefaultSentinelGroup = "gorilla"`, sentinel replica count `3`, replication replica count `3` are invisible to API consumers when buried in `translator/v2/redis.go`.

```
BEFORE                                  AFTER
translator/v2/redis.go                  api/v2/weightsandbiases_types.go
  DefaultSentinelGroup = "gorilla"        // +kubebuilder:default="gorilla"
  sentinel replicas: 3                    // +kubebuilder:default=3
  replication replicas: 3                 (or explicit constants in api/v2/)

                                        translator/v2/redis.go
                                          (reads defaults from spec, no longer sets them)
```

**Files touched:**
- `api/v2/weightsandbiases_types.go` — add kubebuilder default annotations or constants
- `translator/v2/redis.go` — remove hardcoded defaults, read from spec fields
- CRD regeneration required (`make manifests`)

**Risk:** CRD schema change — existing CRs without explicit values will get defaults via webhook/defaulting. Requires migration consideration for existing clusters.

---

### Phase 3 — Structural splits (readability, no behavioral change)

#### Move 8: Split `reconcile_v2.go` into focused files

**Why:** This file handles finalizers, infra gating, secret generation, Kafka topics, MySQL init, RBAC, Gateway, migrations, app deployment, telemetry injection, and HTTPRoutes. Each is a separable concern.

```
BEFORE                                  AFTER
controller/v2/                          controller/v2/
  reconcile_v2.go (everything)            reconcile_v2.go   (orchestration skeleton)
                                          rbac.go           (ServiceAccount/Role/RoleBinding)
                                          migrations.go     (migration job orchestration)
                                          kafka_topics.go   (Kafka topic creation)
                                          secrets.go        (application secret generation)
                                          applications.go   (application deployment loop)
```

**Approach:** Extract functions, not rewrite. Each new file gets the functions that belong to it. `reconcile_v2.go` retains the top-level `ReconcileV2` method that calls into the others.

---

#### Move 9: Extract label patching from `managed/*/write.go`

**Why:** `ensurePVCLabels()` and `ensurePodLabels()` patch labels on resources created by vendor operators, not by this controller. This is a cross-cutting concern.

```
BEFORE                                  AFTER
managed/redis/opstree/write.go          managed/redis/opstree/write.go
  WriteState()                            WriteState()
  ensurePVCLabels()                         (calls common.EnsurePVCLabels)
  ensurePodLabels()
                                        internal/controller/common/labels.go  (exists)
managed/mysql/mysql/write.go              EnsurePVCLabels()  <-- generalized
  WriteState()                            EnsurePodLabels()  <-- generalized
  (PVC label patching inline)
```

**Files touched:**
- `internal/controller/common/labels.go` — already exists, add generalized functions
- `managed/redis/opstree/write.go` — replace inline implementations with common calls
- `managed/mysql/mysql/write.go` — same

---

#### Move 10: Decouple `api/v2` from KEDA and Argo Rollouts

**Why:** `application_types.go` imports KEDA `ScaledObjectSpec` and Argo `RolloutStatus`, coupling the CRD schema to two external operator APIs. These are workload orchestration details.

```
BEFORE                                  AFTER
api/v2/application_types.go             api/v2/application_types.go
  import kedav1alpha1                     ScaledObjectTemplate *apiextv1.JSON
  import argo v1alpha1                    RolloutStatus        *apiextv1.JSON
  ScaledObjectTemplate *ScaledObjectSpec    (or operator-local mirror types)
  RolloutStatus *v1alpha1.RolloutStatus
                                        controller/v2/ (or translator/v2/)
                                          typed conversion when needed
```

**Risk:** Highest-risk move. Changes the CRD schema representation. Requires:
- CRD regeneration
- Conversion webhooks or migration strategy for existing CRs
- Downstream consumers that read these fields must handle the new representation

**Recommendation:** Defer this to a separate PR after the other moves are validated.

---

## Dependency Graph (Before vs After)

### Before

```
                    api/v2
                   /  |   \
                KEDA  |  Argo        <-- external deps leak into CRD
                      |
               translator/v2
              /    |    |    \
         converters + vendor specs   <-- mixed concerns
            |      |    |      |
            v      v    v      v
      managed/   managed/  managed/  managed/
      redis      mysql     kafka     clickhouse ...
        |          |
        |     [conn secret     <-- write during ReadState
        |      written in
        |      read.go]
        |
   controller/v2
     reconcile_v2.go             <-- monolith
       mysql.go                  <-- credential gen here
       telemetry_config.go       <-- domain types here
       (calls InferExternalStatus
        from infra/external)     <-- wrong layer
```

### After

```
                    api/v2
                      |              <-- no external operator deps
                      |
               translator/v2
                 (converters only)   <-- clean, stable
                      |
         +-----------++-----------+
         |            |           |
   managed/redis  managed/mysql  managed/kafka ...
     spec.go        spec.go       spec.go        <-- vendor specs live here
     credentials.go               <-- credential lifecycle co-located
     write.go                     <-- writes secrets in write path
     read.go                      <-- pure reads
         |            |           |
         +-----------++-----------+
                      |
               controller/v2
                 reconcile_v2.go     <-- slim orchestration
                 rbac.go
                 migrations.go
                 kafka_topics.go
                 secrets.go
                 applications.go
                 external_status.go  <-- InferExternalStatus lives here
                      |
              internal/telemetry/    <-- domain types
                      |
             pkg/wandb/manifest/
               telemetry.go         <-- app allowlist
```

---

## Execution Order

| Order | Move | Risk | Touches Tests | CRD Change |
|-------|------|------|--------------|------------|
| 1 | Move 8: Split `reconcile_v2.go` | Low | No | No |
| 2 | Move 2: Telemetry types to `internal/telemetry/` | Low | No | No |
| 3 | Move 3: `InferExternalStatus` to `controller/v2/` | Low | No | No |
| 4 | Move 4: Telemetry allowlist to manifest | Low | No | No |
| 5 | Move 9: Extract label patching | Low | No | No |
| 6 | Move 1: Vendor spec builders to `infra/managed/` | Medium | Update imports in tests | No |
| 7 | Move 6: MySQL credentials to managed layer | Medium | Possibly | No |
| 8 | Move 5: Fix write-in-read | Medium | Yes | No |
| 9 | Move 7: Defaults to `api/v2` | Medium | Yes | Yes |
| 10 | Move 10: Decouple KEDA/Argo from CRD | High | Yes | Yes |

Low-risk pure moves first (1-5), then medium-risk semantic corrections (6-8), then CRD-changing moves last (9-10). Each move should be a separate PR.
