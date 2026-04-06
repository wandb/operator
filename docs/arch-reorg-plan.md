# Architecture Reorganization Plan (v2)

Based on the analysis in [arch-reorg-v2.md](./arch-reorg-v2.md).

---

## Current State

```
api/v2/
  application_types.go        <-- imports KEDA + Argo Rollouts directly
  weightsandbiases_types.go   <-- no +kubebuilder:default for many fields

internal/webhook/v2/
  weightsandbiases_webhook.go <-- 522 lines: defaulter + validator in one file
                                  sets defaults that have no reconciler fallback
                                  immutability only enforced for Redis, not others
  application_webhook.go      <-- well-scoped
  *_test.go                   <-- already split per-service (tests ahead of impl)

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

Fifteen moves, grouped into four phases by risk and dependency.

---

### Phase 1 — Pure moves (no behavioral change)

These relocate code without altering any function signatures or call semantics. Tests should pass identically before and after.

#### Move 1: Split `reconcile_v2.go` into focused files

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

#### Move 2: Split `weightsandbiases_webhook.go` into defaulter and validator files

**Why:** This 522-line file contains both the mutating defaulter and the validating webhook, plus all per-service helper functions. The test files already follow a per-service split (`weightsandbiases_defaulter_mysql_test.go`, etc.), but the implementation is monolithic.

```
BEFORE                                          AFTER
webhook/v2/                                     webhook/v2/
  weightsandbiases_webhook.go (522 lines)         weightsandbiases_defaulter.go
    WeightsAndBiasesCustomDefaulter                  WeightsAndBiasesCustomDefaulter
    Default()                                        Default()
    applyMySQLDefaults()                             applyMySQLDefaults()
    applyRedisDefaults()                             applyRedisDefaults()
    applyKafkaDefaults()                             applyKafkaDefaults()
    applyObjectStoreDefaults()                       applyObjectStoreDefaults()
    applyClickHouseDefaults()                        applyClickHouseDefaults()
    WeightsAndBiasesCustomValidator
    ValidateCreate()                               weightsandbiases_validator.go
    ValidateUpdate()                                 WeightsAndBiasesCustomValidator
    ValidateDelete()                                 ValidateCreate()
    validateSpec()                                   ValidateUpdate()
    validateChanges()                                ValidateDelete()
    validateMySQLSpec()                              validateSpec()
    validateRedisSpec()                              validateChanges()
    validateKafkaSpec()                              validateMySQLSpec()
    validateObjectStoreSpec()                        validateRedisSpec()
    validateClickHouseSpec()                         validateKafkaSpec()
    validateRedisChanges()                           validateObjectStoreSpec()
    validateNetworkingSpec()                         validateClickHouseSpec()
    SetupWeightsAndBiasesWebhookWithManager          validateRedisChanges()
                                                     validateNetworkingSpec()

                                                   weightsandbiases_webhook.go (kept, slim)
                                                     SetupWeightsAndBiasesWebhookWithManager
```

**Files touched:**
- `webhook/v2/weightsandbiases_webhook.go` — split into three files
- No import changes needed (same package)

---

#### Move 3: Extract vendor spec builders from `translator/v2/` to `infra/managed/`

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

#### Move 4: Move telemetry domain types to `internal/telemetry/`

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
- `controller/v2/telemetry_config.go` — delete
- New `internal/telemetry/config.go` — receive type definitions
- `controller/v2/telemetry_secret.go`, `reconcile_v2.go` — update imports

---

#### Move 5: Move `InferExternalStatus` to `controller/v2/`

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

#### Move 6: Move telemetry application allowlist to manifest layer

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

#### Move 7: Extract label patching from `managed/*/write.go`

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

### Phase 2 — Semantic corrections (behavioral clarification)

These change the call graph to fix naming/boundary violations. Require careful testing.

#### Move 8: Fix write-in-read — relocate connection secret writes to `WriteState`

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

#### Move 9: Move MySQL credential generation to managed infra layer

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

### Phase 3 — Webhook ↔ Reconciler hardening

These address the implicit contract between the webhook and reconciler. The webhook is optional (`--enable-webhooks`), but the reconciler currently assumes webhook-applied defaults and invariants hold.

#### Move 10: Consolidate static defaults into `api/v2` kubebuilder annotations

**Why:** Static scalar defaults are set in the webhook defaulter but have no fallback when webhooks are disabled. `+kubebuilder:default` annotations are always applied (by the API server when creating the object), are visible in `kubectl explain`, and don't require the webhook.

```
BEFORE                                          AFTER
webhook/v2/weightsandbiases_defaulter.go        api/v2/weightsandbiases_types.go
  Spec.Size = SizeDev                             // +kubebuilder:default="dev"
  Spec.RetentionPolicy.OnDelete = Detach          // +kubebuilder:default="detach"
  Spec.ServiceAccount.Create = true               // +kubebuilder:default=true
  Spec.ServiceAccount.Name = "wandb"              // +kubebuilder:default="wandb"
  Spec.InternalServiceAuth.Enabled = true         // +kubebuilder:default=true

translator/v2/redis.go                         api/v2/weightsandbiases_types.go
  DefaultSentinelGroup = "gorilla"                // +kubebuilder:default="gorilla"
  sentinel replicas: 3                            // +kubebuilder:default=3
  replication replicas: 3                         // +kubebuilder:default=3

                                                webhook/v2/weightsandbiases_defaulter.go
                                                  (remove static defaults, keep computed only:
                                                   sentinel-for-non-dev, per-service namespace,
                                                   per-service name, manifest repo URI scheme)

                                                translator/v2/redis.go
                                                  (reads from spec, no longer sets defaults)
```

**Files touched:**
- `api/v2/weightsandbiases_types.go` — add `+kubebuilder:default` annotations
- `webhook/v2/weightsandbiases_webhook.go` (or new `weightsandbiases_defaulter.go`) — remove static defaults
- `translator/v2/redis.go` — remove hardcoded defaults, read from spec
- CRD regeneration required (`make manifests`)

**Risk:** CRD schema change — existing CRs without explicit values will get defaults from the API server on next write. Requires migration consideration for existing clusters.

---

#### Move 11: Add reconciler-side fallback defaults for computed webhook defaults

**Why:** Computed defaults (sentinel-for-non-dev, per-service namespace/name) are applied only by the webhook. When webhooks are disabled, the reconciler sees zero-value fields and may nil-pointer or create resources with empty names.

```
BEFORE                                          AFTER
controller/v2/redis.go                          controller/v2/redis.go
  redisWriteState()                               redisWriteState()
    // assumes Spec.ManagedRedis.Namespace          if spec.ManagedRedis.Namespace == "" {
    // is already populated by webhook                spec.ManagedRedis.Namespace = wandb.Namespace
                                                    }
                                                    // same for Name, SentinelEnabled, etc.
```

**Approach:** Add a `ensureDefaults()` or inline nil-checks at the top of each dispatcher function in `controller/v2/`. These are cheap no-ops when the webhook has already run (the field will already be set), and serve as safety nets when it hasn't.

**Files touched:**
- `controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go` — add fallback defaults at entry points
- Possibly a shared `controller/v2/defaults.go` if the pattern is repetitive enough

---

#### Move 12: Add immutability enforcement for non-Redis infra types in webhook

**Why:** Only Redis has update-time change validation (`validateRedisChanges`). MySQL, Kafka, ObjectStore, and ClickHouse have no equivalent, even though changing their namespaces or storage sizes is equally dangerous (orphaned resources, data loss).

```
BEFORE                                          AFTER
webhook/v2/weightsandbiases_validator.go        webhook/v2/weightsandbiases_validator.go
  validateChanges()                               validateChanges()
    validateRedisChanges()                          validateRedisChanges()     (existing)
                                                    validateMySQLChanges()     <-- NEW
                                                    validateKafkaChanges()     <-- NEW
                                                    validateObjectStoreChanges() <-- NEW
                                                    validateClickHouseChanges()  <-- NEW
```

**Immutable fields per infra type (proposed):**

| Infra Type | Immutable Fields |
|------------|-----------------|
| Redis | StorageSize, Namespace, SentinelEnabled (existing) |
| MySQL | Namespace, Name (once vendor CR exists) |
| Kafka | Namespace, Name (once vendor CR exists) |
| ObjectStore | Namespace, Name (once vendor CR exists) |
| ClickHouse | Namespace, Name (once vendor CR exists) |

**Files touched:**
- `webhook/v2/weightsandbiases_webhook.go` (or new `weightsandbiases_validator.go`) — add `validateXxxChanges()` functions
- New test files: `weightsandbiases_validator_{mysql,kafka,objectstore,clickhouse}_test.go`

---

#### Move 13: Add reconciler-side immutability guards

**Why:** Even with webhook validation, the reconciler should defensively check immutability for cases where the webhook is disabled. The reconciler check doesn't need to produce user-friendly validation errors — it just needs to set an error condition and stop reconciling.

```
BEFORE                                          AFTER
controller/v2/redis.go                          controller/v2/redis.go
  redisWriteState()                               redisWriteState()
    // trusts webhook has prevented                  if statusHas(oldNamespace) && oldNamespace != newNamespace {
    // namespace changes                               setCondition("ImmutabilityViolation", ...)
                                                       return requeueWithError(...)
                                                     }
```

**Approach:** Before calling `WriteState`, compare the current spec against the last-known state in `wandb.Status.XxxStatus`. If a field that should be immutable has changed, set an error condition on the status and return without proceeding.

**Files touched:**
- `controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go` — add pre-write immutability checks

---

### Phase 4 — High-risk structural changes

These change the CRD schema representation. Require migration strategy.

#### Move 14: Relocate hardcoded defaults to `api/v2` (fields that don't exist yet)

**Why:** Some translator defaults (`DefaultSentinelGroup`, replica counts) have no corresponding CRD field — users cannot override them. This move adds the CRD fields first, then Move 10 can apply the defaults.

```
BEFORE                                          AFTER
api/v2/weightsandbiases_types.go                api/v2/weightsandbiases_types.go
  ManagedRedisSpec                                ManagedRedisSpec
    StorageSize string                              StorageSize string
    SentinelEnabled *bool                           SentinelEnabled *bool
                                                    SentinelGroupName string   <-- NEW
                                                    SentinelReplicas  *int32   <-- NEW
                                                    ReplicationReplicas *int32 <-- NEW
```

**Files touched:**
- `api/v2/weightsandbiases_types.go` — add new fields with `+kubebuilder:default` annotations
- `translator/v2/redis.go` — read from spec instead of hardcoding
- `webhook/v2/` — add validation for new fields if needed
- CRD regeneration required (`make manifests`)
- `make generate` for DeepCopy methods

**Risk:** CRD schema change. New fields are additive (no breaking change for existing CRs), but need to ensure existing CRs get sensible defaults.

---

#### Move 15: Decouple `api/v2` from KEDA and Argo Rollouts

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
                KEDA  |  Argo          <-- external deps leak into CRD
                      |
          webhook/v2  |                <-- sets defaults, no reconciler fallback
            (monolith)|                    immutability only for Redis
                      |
               translator/v2
              /    |    |    \
         converters + vendor specs     <-- mixed concerns
            |      |    |      |          + invisible defaults
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
                      |                <-- no external operator deps
                      |                    +kubebuilder:default for all static defaults
                      |
          webhook/v2  |
            defaulter.go               <-- computed defaults only
            validator.go               <-- all infra types have immutability checks
                      |
               translator/v2
                 (converters only)     <-- clean, stable, no defaults
                      |
         +-----------++-----------+
         |            |           |
   managed/redis  managed/mysql  managed/kafka ...
     spec.go        spec.go       spec.go          <-- vendor specs live here
     credentials.go                <-- credential lifecycle co-located
     write.go                      <-- writes secrets in write path
     read.go                       <-- pure reads
         |            |           |
         +-----------++-----------+
                      |
               controller/v2
                 reconcile_v2.go       <-- slim orchestration
                 rbac.go
                 migrations.go
                 kafka_topics.go
                 secrets.go
                 applications.go
                 external_status.go    <-- InferExternalStatus lives here
                 defaults.go           <-- reconciler fallback defaults
                      |
              internal/telemetry/      <-- domain types
                      |
             pkg/wandb/manifest/
               telemetry.go           <-- app allowlist
```

---

## Execution Order

| Order | Move | Phase | Risk | Touches Tests | CRD Change |
|-------|------|-------|------|--------------|------------|
| 1 | Move 1: Split `reconcile_v2.go` | 1 | Low | No | No |
| 2 | Move 2: Split `weightsandbiases_webhook.go` | 1 | Low | No | No |
| 3 | Move 4: Telemetry types to `internal/telemetry/` | 1 | Low | No | No |
| 4 | Move 5: `InferExternalStatus` to `controller/v2/` | 1 | Low | No | No |
| 5 | Move 6: Telemetry allowlist to manifest | 1 | Low | No | No |
| 6 | Move 7: Extract label patching | 1 | Low | No | No |
| 7 | Move 3: Vendor spec builders to `infra/managed/` | 1 | Medium | Update imports in tests | No |
| 8 | Move 9: MySQL credentials to managed layer | 2 | Medium | Possibly | No |
| 9 | Move 8: Fix write-in-read | 2 | Medium | Yes | No |
| 10 | Move 11: Reconciler fallback defaults | 3 | Low | Yes (new tests) | No |
| 11 | Move 12: Non-Redis immutability in webhook | 3 | Low | Yes (new tests) | No |
| 12 | Move 13: Reconciler immutability guards | 3 | Low | Yes (new tests) | No |
| 13 | Move 10: Static defaults to kubebuilder annotations | 3 | Medium | Yes | Yes |
| 14 | Move 14: New CRD fields for translator defaults | 4 | Medium | Yes | Yes |
| 15 | Move 15: Decouple KEDA/Argo from CRD | 4 | High | Yes | Yes |

Phase 1 (pure moves, 1–7) can be done in parallel across PRs since they touch different files. Phase 2 (semantic corrections, 8–9) must be sequential. Phase 3 (webhook ↔ reconciler hardening, 10–13) can mostly be parallelized. Phase 4 (CRD changes, 14–15) must go last and each needs its own PR.

Each move should be a separate PR.
