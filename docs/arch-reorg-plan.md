# Architecture Reorganization Plan (v2)

Based on the analysis in [arch-reorg.md](./arch-reorg.md).

> Refresh: this plan supersedes the prior version. Several moves from the
> earlier plan are still valid, but the codebase has grown significantly
> (notably `reconcile_v2.go` to 2592 lines, plus a full networking layer:
> `gateway.go`, `infra_routes.go`, `ingress.go`, `networking_cleanup.go`),
> manifest-derived sizing has been added, and `infra/managed/minio/tenant/`
> has already migrated to the read=pure / write=side-effects pattern that
> the rest of the managed-infra packages still need.
>
> **Update (this round):** Move 3 is **DONE**. The cleanup also went deeper
> than originally planned — `internal/controller/translator/` and
> `internal/controller/translator/v2/` have been **deleted entirely**.
> The parallel `translator.XxxConnection`/`XxxStatus` type hierarchy
> collapsed into `apiv2.XxxConnection`/`apiv2.XxxInfraStatus`. Per-vendor
> constants moved into `infra/managed/{vendor}/spec.go`. Shared
> labels/retention/condition primitives moved into
> `internal/controller/common/`. See "What's Already Done" at the bottom
> for the full list.

---

## Current State

```
api/v2/
  application_types.go        <-- imports KEDA + Argo Rollouts + Gateway API
  weightsandbiases_types.go   <-- has SOME +kubebuilder:default annotations
                                  (OnDelete=detach, ServiceAccount.Create=true,
                                  ServiceAccountName="wandb",
                                  Telemetry.Enabled=true, gateway.managed=false)
                                  but webhook also sets some of these
                                  Connection/InfraStatus types are now the
                                  canonical internal types — no parallel
                                  hierarchy in translator/ anymore.
  weightsandbiases_conversion.go

internal/webhook/v2/
  weightsandbiases_webhook.go (522 lines)
                              <-- defaulter + validator in one file
                                  webhook-only defaults have no reconciler fallback
                                  immutability enforcement only for Redis
                                  validateNetworkingSpec is well-shaped
  application_webhook.go      <-- well-scoped
  *_test.go                   <-- already split per-service (tests ahead of impl)

internal/controller/
  common/                     <-- expanded this round
    labels.go                  <-- WandbXxxLabel constants + BuildWandbLabels
    retention.go               <-- OnDeletePolicy/Purge/Detach/OnDeleteRule + ToOnDeleteRule
    condition.go               <-- DefaultConditionExpiry + condition machinery
    state.go, resource.go, detach.go  (existing)

  (translator/ and translator/v2/ — DELETED this round)

  infra/
    external/                  <-- speaks apiv2.XxxConnection directly
      common.go                <-- InferExternalStatus (still wrong layer; Move 5 pending)
      redis/redis.go, mysql/mysql.go, kafka/kafka.go,
      clickhouse/clickhouse.go, objectstore/objectstore.go
    managed/
      redis/opstree/
        spec.go                <-- ToRedis* builders + RedisModuleName + image consts +
                                   DefaultSentinelGroup + telemetry exporter helper
        read.go                <-- still calls writeRedisConnInfo (write-in-read)
        conn.go                <-- writeRedisConnInfo (returns *apiv2.RedisConnection)
        write.go               <-- ensurePVCLabels, ensurePodLabels (cross-cutting)
        status.go, naming.go, detach.go, purge.go, values.go
      mysql/mysql/
        spec.go                <-- ToMysqlMySQLVendorSpec + MysqlModuleName +
                                   DefaultMySQLExporterImage + hardcoded mycnf
        read.go                <-- still calls writeMySQLConnInfo (write-in-read)
        conn.go                <-- writeMySQLConnInfo (returns *apiv2.MysqlConnection)
        write.go               <-- inline PVC label patching
        status.go, naming.go, detach.go, purge.go
      kafka/strimzi/
        spec.go                <-- ToKafkaVendorSpec + KafkaModuleName +
                                   KafkaVersion + KafkaMetadataVersion + metrics helper
        read.go                <-- still calls writeKafkaConnInfo (write-in-read)
        conn.go, write.go, restore.go
        status.go, naming.go, detach.go, purge.go, values.go
      clickhouse/altinity/
        spec.go                <-- ToClickHouseVendorSpec + ClickhouseModuleName
        read.go                <-- still calls writeClickHouseConnInfo (write-in-read)
        conn.go, write.go
        status.go, naming.go, detach.go, purge.go, values.go
      minio/tenant/             <-- REFERENCE: writes secret in WriteState
        spec.go                <-- ToObjectStoreVendorSpec + ObjectStoreModuleName +
                                   MinioImage + DevVolumesPerServer + telemetry env
        read.go                <-- pure observation (correct)
        conn.go                <-- writeWandbConnInfo (called from write.go)
        write.go               <-- WriteState calls writeMinioConfig+writeWandbConnInfo
                                  returns connection ref
        status.go, naming.go, detach.go, purge.go, config.go

  v2/
    reconcile_v2.go (2592 LoC) <-- monolith. ~17 concerns:
                                   finalizers, manifest fetch, ApplyInfraSizing,
                                   infra dispatch, telemetry secret reconcile,
                                   secret generation, kafka topics, mysql init,
                                   RBAC (~240 LoC), gateway dispatch,
                                   migrations (~160 LoC), application loop,
                                   container/init/JWT/inline-files resolution,
                                   resolveEnvvars (~280 LoC),
                                   sizing helpers (~190 LoC),
                                   telemetry env injection (lines 58-135),
                                   networking-mode cleanup, infra HTTPRoutes,
                                   ingress, hostname inference, status update
    mysql.go (260)             <-- credential generation in dispatcher (~65 lines)
    redis.go, kafka.go, clickhouse.go, objectstore.go  <-- clean dispatchers,
                                   no more translator/translatorv2 imports
    telemetry_config.go (91)   <-- domain types in controller layer
    telemetry_secret.go (117)  <-- telemetry secret reconciliation
    gateway.go (352)           <-- imports nginx-gateway-fabric
    infra_routes.go (465)      <-- imports gke-gateway-api
    ingress.go (201)           <-- consolidated Ingress
    networking_cleanup.go (57) <-- cross-mode cleanup
    manifest_order.go (25)     <-- ordering helpers (clean)
```

---

## Proposed Changes

Eighteen moves, grouped into four phases by risk and dependency. Phase 1
moves are mechanical relocations with no behavioral change. Phase 2 fixes
naming/boundary violations. Phase 3 hardens the webhook ↔ reconciler
contract. Phase 4 changes the CRD schema.

---

### Phase 1 — Pure moves (no behavioral change)

#### Move 1: Split `reconcile_v2.go` into focused files

**Why:** 2592 lines covering ~17 distinct concerns. Each is a separable
unit; nothing about the public API changes.

```
BEFORE                                  AFTER (within controller/v2/)
reconcile_v2.go (2592 LoC)                reconcile_v2.go      (orchestration skeleton)
                                          rbac.go              (SA/Role/RoleBinding/CRB)
                                          migrations.go        (runMigrations)
                                          mysql_init.go        (runMysqlInitJob)
                                          kafka_topics.go      (createKafkaTopics)
                                          secrets.go           (generateSecrets)
                                          applications.go      (reconcileApplications,
                                                                buildHTTPRouteTemplate,
                                                                resolveContainers,
                                                                resolveInitContainers,
                                                                resolveJWTTokens,
                                                                resolveInlineFiles)
                                          env_resolution.go    (resolveEnvvars,
                                                                resolveVolumeMounts,
                                                                resolveServiceURLFromManifest,
                                                                resolveServicePortFromManifest,
                                                                resolveCRFieldString)
                                          sizing.go            (ResolveResources,
                                                                ResolveAutoscaling,
                                                                ResolveInfraSizing,
                                                                ResolveKafkaSizing,
                                                                ApplyInfraSizing,
                                                                mergeResources)
                                          telemetry_inject.go  (injectManagedWorkloadTelemetryEnvvars,
                                                                applyWorkloadTelemetryDefaults,
                                                                appendMissingEnvVars,
                                                                hasWorkloadTelemetryConfig,
                                                                shouldInjectManagedWorkloadTelemetry,
                                                                managedWorkloadTelemetryApplications,
                                                                managedWorkloadTelemetryEnvVars)
                                          state.go             (inferState)
```

**Approach:** Move functions verbatim; do not alter signatures or behavior.
`reconcile_v2.go` retains the top-level `Reconcile` and
`ReconcileWandbManifest`.

**Files touched:** `controller/v2/*` (no test changes; same package).

---

#### Move 2: Split `weightsandbiases_webhook.go`

**Why:** 522 lines mixing defaulter and validator; per-service tests
already follow the split pattern.

```
BEFORE                              AFTER
weightsandbiases_webhook.go (522)     weightsandbiases_defaulter.go
                                        WeightsAndBiasesCustomDefaulter
                                        Default()
                                        applyMySQLDefaults / Redis / Kafka /
                                          ObjectStore / ClickHouse
                                      weightsandbiases_validator.go
                                        WeightsAndBiasesCustomValidator
                                        ValidateCreate/Update/Delete
                                        validateSpec, validateChanges
                                        validateMySQLSpec / Redis / Kafka /
                                          ObjectStore / ClickHouse
                                        validateRedisChanges
                                        validateNetworkingSpec
                                      weightsandbiases_webhook.go
                                        SetupWeightsAndBiasesWebhookWithManager
```

**Files touched:** `webhook/v2/weightsandbiases_webhook.go` (split into
three; no test changes).

---

#### Move 3: Extract vendor spec builders from `translator/v2/` to `infra/managed/` — **DONE**

Vendor spec builders relocated to `infra/managed/{vendor}/spec.go`. The
move went further than originally scoped:

- `translator/v2/{mysql,redis,kafka,objectstore,clickhouse}.go` — deleted.
- New `infra/managed/{mysql/mysql,redis/opstree,kafka/strimzi,minio/tenant,clickhouse/altinity}/spec.go`
  files contain `ToXxxVendorSpec` builders, `BuildWandbXxxLabels`,
  `ToXxxOnDeleteRule`, plus the per-vendor module-name and image constants.
- `controller/v2/{mysql,redis,kafka,objectstore,clickhouse}.go` updated
  to call vendor packages directly (e.g. `mysql.ToMysqlMySQLVendorSpec`,
  `opstree.ToRedisStandaloneVendorSpec`).
- The follow-on type-hierarchy collapse and shared-utility consolidation
  is documented in "What's Already Done" below.

---

#### Move 4: Move telemetry domain types to `internal/telemetry/`

**Why:** `TelemetryRuntimeConfig`, `TelemetryOTelConfig`, `TelemetryEndpoints`
are configuration domain types referenced from `cmd/main.go`,
`weightsandbiases_controller.go`, `reconcile_v2.go`, and `telemetry_secret.go`.
Other future packages may need them.

```
BEFORE                              AFTER
controller/v2/                      controller/v2/
  telemetry_config.go  (types)        telemetry_secret.go (imports internal/telemetry)
  telemetry_secret.go

                                    internal/telemetry/         <-- NEW
                                      config.go                <-- types moved
                                      endpoints.go             <-- ResolveEndpoints, resolveServiceHost
```

**Files touched:**
- New `internal/telemetry/`.
- `controller/v2/telemetry_config.go` — delete.
- `controller/v2/telemetry_secret.go`, `reconcile_v2.go`,
  `cmd/main.go`, `internal/controller/weightsandbiases_controller.go` —
  update imports.

---

#### Move 5: Move `InferExternalStatus` to `controller/v2/`

**Why:** Defined in `infra/external/common.go` but called only from
`controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go`. Computes
health state from connection presence — an orchestration concern.

```
BEFORE                                  AFTER
infra/external/common.go                infra/external/common.go
  ResolveSecretKey                        (unchanged)
  WriteConnectionSecret                   (unchanged)
  ReadConnectionSecret                    (unchanged)
  ResolveFields                           (unchanged)
  BuildWandbOwnerRef                      (unchanged)
  DeleteConnectionSecret                  (unchanged)
  InferExternalStatus    (MOVES)

controller/v2/                          controller/v2/
  ...                                     external_status.go     <-- NEW: InferExternalStatus
```

**Files touched:**
- `infra/external/common.go` — remove function.
- New `controller/v2/external_status.go`.
- `controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go` — update call sites.

---

#### Move 6: Move telemetry application allowlist to manifest layer

**Why:** `managedWorkloadTelemetryApplications` (lines 58-72) and
`managedWorkloadTelemetryEnvVars` (74-135) are configuration that must be
edited whenever the manifest changes. They belong with the manifest.

```
BEFORE                                  AFTER
controller/v2/reconcile_v2.go           controller/v2/telemetry_inject.go (after Move 1)
  managedWorkloadTelemetryApplications    (imports from pkg/wandb/manifest)
  managedWorkloadTelemetryEnvVars

                                        pkg/wandb/manifest/telemetry.go     <-- NEW
                                          ManagedWorkloadTelemetryApplications
                                          ManagedWorkloadTelemetryEnvVars
```

**Files touched:**
- `controller/v2/reconcile_v2.go` (or `telemetry_inject.go` after Move 1)
  — remove constants.
- New `pkg/wandb/manifest/telemetry.go`.

---

#### Move 7: Extract label patching to a shared utility

**Why:** `ensurePVCLabels` and `ensurePodLabels` in
`managed/redis/opstree/write.go:84-152` and similar inline code in
`managed/mysql/mysql/write.go` patch labels on resources created by
vendor operators — a cross-cutting concern.

```
BEFORE                                  AFTER
managed/redis/opstree/write.go          managed/redis/opstree/write.go
  WriteState()                            WriteState()
  ensurePVCLabels()                         (calls common.EnsurePVCLabels)
  ensurePodLabels()                         (calls common.EnsurePodLabels)
  matchesAnyPrefix()

managed/mysql/mysql/write.go            internal/controller/common/labels.go
  WriteState()                            (already exists, with HasAllLabelKeys)
  (inline PVC label patching)             EnsurePVCLabels                 <-- NEW
                                          EnsurePodLabels                 <-- NEW
                                          matchesAnyPrefix                <-- NEW
```

**Files touched:**
- `internal/controller/common/labels.go`.
- `managed/redis/opstree/write.go` — replace with common calls.
- `managed/mysql/mysql/write.go` — same.

---

#### Move 8: Provider-gate networking-vendor imports

**Why:** `controller/v2/gateway.go` imports `nginx-gateway-fabric`
unconditionally, and `controller/v2/infra_routes.go` imports
`gke-gateway-api` unconditionally. Currently every operator deployment
carries the union of all provider-specific CRDs.

```
BEFORE                                  AFTER
controller/v2/gateway.go                controller/v2/networking/
  imports nginxGatewayv1alpha1           gateway.go               (provider-agnostic)
                                         providers/
controller/v2/infra_routes.go              nginx/policies.go      (nginxGatewayv1alpha1)
  imports gkeGatewayApiNetworkingv1        gke/health_check.go    (gkeGatewayApiNetworkingv1)
                                         infra_routes.go          (provider-agnostic)
```

**Approach:** Keep gateway.go/infra_routes.go provider-agnostic. Move
provider-specific resource construction (`nginxGatewayv1alpha1.*`,
`gkeGatewayApiNetworkingv1.HealthCheckPolicy`) into provider sub-packages
that are conditionally registered in `cmd/main.go` via
`utils.IsRegistered(scheme, ...)`.

**Risk:** Medium — touches networking dispatch flow. Tested by
`networking_route_builders_test.go` and
`weightsandbiases_controller_networking_test.go`.

**Files touched:**
- `controller/v2/gateway.go`, `controller/v2/infra_routes.go` — remove
  provider-specific code paths.
- New `controller/v2/networking/providers/{nginx,gke}/`.
- `cmd/main.go` — provider registration helpers.

---

### Phase 2 — Semantic corrections

These change the call graph to fix naming/boundary violations.

#### Move 9: Fix write-in-read for redis/mysql/kafka/clickhouse

**Why:** Every `managed/{redis,mysql,kafka,clickhouse}/read.go` calls
`writeXxxConnInfo()` on each reconcile, which silently creates/overwrites
a Kubernetes Secret. Callers expect `ReadState` to be idempotent.
**Reference implementation: `managed/minio/tenant/`** — `WriteState` calls
`writeWandbConnInfo` and returns the connection; `ReadState` is pure.

```
BEFORE (per managed vendor)             AFTER (per managed vendor)

write.go                                write.go
  WriteState()                            WriteState() returns connection
    creates/updates vendor CR               creates/updates vendor CR
    (nothing else)                          calls writeXxxConnInfo() if CR exists
                                            returns connection ref
read.go                                 read.go
  ReadState()                             ReadState()
    reads vendor CR status                  reads vendor CR status
    calls writeXxxConnInfo()  <-- BAD       (no side effects)
    returns connection                      returns nil for connection
                                            (caller already has it from WriteState)
```

**Approach (matches minio):** Change the dispatcher pattern so that
`WriteState` returns the connection (already done for objectstore in
`controller/v2/objectstore.go`), and `ReadState` becomes pure-read.

**Risk:** Medium-high. This changes the timing of when connection Secrets
are written. Must verify that `WriteState` has sufficient information to
write the secret (e.g. for ClickHouse, the endpoint is in
`Status.Endpoint`, which only exists after the vendor operator has
processed the CR).

**Mitigation:** For vendors where the connection is only available after
the vendor CR has reconciled (clickhouse, kafka), `WriteState` may need to
return `nil` connection on first reconcile and `ReadState` reads the
secret if `WriteState` returned nil. This is how minio currently
sequences (note `writeMinioConfig` uses `goutils.RandomAlphabetic`
internally for the password if missing).

**Files touched per vendor (redis/opstree, mysql/mysql, kafka/strimzi,
clickhouse/altinity):**
- `read.go` — remove `writeXxxConnInfo` call; read existing secret instead.
- `write.go` — `WriteState` signature changes to return
  `(conditions, *translator.XxxConnection)`.
- `controller/v2/{redis,mysql,kafka,clickhouse}.go` — dispatchers update
  call sites and return-value plumbing.

---

#### Move 10: Move MySQL credential generation into managed-infra

**Why:** `controller/v2/mysql.go:102-168` (~65 lines) generates the random
root and user passwords and writes the `db-password` Secret before
delegating to `mysql.WriteState`. The managed MySQL package is
oblivious to this credential's existence; it only references the secret
by name (plumbed via `translator/v2/mysql.go:58`).

```
BEFORE                                  AFTER
controller/v2/mysql.go                  controller/v2/mysql.go
  managedMysqlWriteState()                managedMysqlWriteState()
    generate root password                  calls mysql.WriteState()
    generate user password                    (credentials handled internally)
    create db-password Secret
    calls mysql.WriteState()            infra/managed/mysql/mysql/credentials.go  <-- NEW
                                          EnsureCredentials()
infra/managed/mysql/mysql/                  - generates if secret missing
  write.go                                  - returns secret name
    WriteState()                          (called from write.go)

                                        infra/managed/mysql/mysql/write.go
                                          WriteState()
                                            calls EnsureCredentials()
                                            builds spec with secret ref
```

**Files touched:**
- `controller/v2/mysql.go` — remove credential generation (~65 lines).
- New `infra/managed/mysql/mysql/credentials.go`.
- `infra/managed/mysql/mysql/write.go` — call `EnsureCredentials`.

**Note:** The MySQL init job in `reconcile_v2.go:runMysqlInitJob` reads
`MYSQL_ROOT_PASSWORD` and `MYSQL_PASSWORD` from the same `db-password`
Secret. Move 10 doesn't change the Secret's location or shape, only who
writes it.

---

### Phase 3 — Webhook ↔ Reconciler hardening

#### Move 11: Consolidate static defaults to `+kubebuilder:default`

**Why:** Layer-1 defaults are always applied (by the API server) and
visible in `kubectl explain`; they don't require the webhook. Several
fields currently set in the webhook (and in some cases *also* annotated)
should be Layer 1 only.

**Currently both Layer 1 and Layer 2** (pick Layer 1, drop Layer 2):
- `RetentionPolicy.OnDelete = "detach"`
- `ServiceAccount.Create = true`
- `ServiceAccount.ServiceAccountName = "wandb"`
- `Telemetry.Enabled = true`

**Currently Layer 2 only** (move to Layer 1):
- `Spec.Size = "dev"`
- `InternalServiceAuth.Enabled = true`
- `InternalServiceAuth.OIDCIssuer = "https://kubernetes.default.svc.cluster.local"`
- `Spec.Wandb.ManifestRepository = "oci://us-docker.pkg.dev/wandb-production/public/wandb/server-manifest"`
- ObjectStore `RootUser = "admin"`
- ObjectStore `MinioBrowserSetting = "on"`

**Currently Layer 4 (translator)** (move to Layer 1 if user-facing,
otherwise rename to "constant"):
- `DefaultSentinelGroup = "gorilla"`
- Sentinel size `3`
- Replication size `3`
- `DefaultRedisExporterImage`, `DefaultRedisExporterPort`
- `DefaultMySQLExporterImage`

**Computed defaults that stay in the webhook**:
- Per-service `Name = {wandb-name}-{service}` (depends on parent CR name)
- Per-service `Namespace = wandb.Namespace` (depends on parent CR namespace)
- Redis sentinel enabled when `Size != SizeDev` (depends on size)
- ManifestRepository scheme-prefix repair (`oci://` prepend)
- Affinity/Tolerations nil-pointer guards (these can also be removed by
  fixing downstream code to handle nil)

**Files touched:**
- `api/v2/weightsandbiases_types.go` — add annotations, possibly add new
  fields for sentinel-group / replica counts (see Move 16).
- `webhook/v2/weightsandbiases_defaulter.go` (post-Move 2) — drop
  redundant defaults.
- `translator/v2/redis.go` — read from spec instead of hardcoding
  (depends on Move 16 adding the spec fields).
- `make manifests` regeneration.

**Risk:** Medium. Changes CRD schema. Existing CRs will gain defaults
on next admission write.

---

#### Move 12: Add reconciler-side fallback defaults

**Why:** When the webhook is disabled, computed defaults
(per-service `Name`/`Namespace`, sentinel-for-non-dev) are not applied
and the reconciler may nil-pointer or create resources with empty names.

```
BEFORE                                          AFTER
controller/v2/redis.go                          controller/v2/defaults.go (NEW)
  redisWriteState()                               ensureManagedRedisDefaults(wandb)
    // assumes Spec.ManagedRedis.Namespace          if spec.Namespace == "" {
    // populated by webhook                            spec.Namespace = wandb.Namespace
                                                    }
                                                    if spec.Name == "" {
                                                      spec.Name = fmt.Sprintf("%s-redis", wandb.Name)
                                                    }
                                                    // sentinel for non-dev
                                                  ensureManagedMysqlDefaults(wandb)
                                                  // ... etc
```

**Approach:** Single `ensureDefaults` per dispatcher, called at the top of
each `redisWriteState`/`mysqlWriteState`/etc. No-op when webhook has run.

**Files touched:**
- New `controller/v2/defaults.go`.
- `controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go` — call at entry.

---

#### Move 13: Add immutability validators for non-Redis infra

**Why:** Only Redis has `validateRedisChanges`. MySQL, Kafka, ObjectStore,
ClickHouse have no equivalent — changing their namespace or storage size
silently orphans the underlying vendor CR.

```
BEFORE                                          AFTER
webhook/v2/weightsandbiases_webhook.go          webhook/v2/weightsandbiases_validator.go (post-Move 2)
  validateChanges()                               validateChanges()
    validateRedisChanges()                          validateRedisChanges()       (existing)
                                                    validateMySQLChanges()       <-- NEW
                                                    validateKafkaChanges()       <-- NEW
                                                    validateObjectStoreChanges() <-- NEW
                                                    validateClickHouseChanges()  <-- NEW
```

| Infra | Immutable fields once vendor CR exists |
|-------|----------------------------------------|
| Redis | StorageSize, Namespace, Sentinel.Enabled (existing) |
| MySQL | Namespace, Name |
| Kafka | Namespace, Name |
| ObjectStore | Namespace, Name |
| ClickHouse | Namespace, Name |

**Files touched:**
- `webhook/v2/weightsandbiases_validator.go` (post-Move 2) — add functions.
- New test files mirroring the per-service pattern:
  `weightsandbiases_validator_{mysql,kafka,objectstore,clickhouse}_test.go`.

---

#### Move 14: Add reconciler-side immutability guards

**Why:** Defense in depth for the webhook-disabled case. The reconciler
guard doesn't need user-friendly errors — it sets an error condition and
stops reconciling.

```
BEFORE                                          AFTER
controller/v2/redis.go                          controller/v2/redis.go
  redisWriteState()                               redisWriteState()
    // trusts webhook                               if hasMaterialChange(oldStatus, newSpec) {
                                                      setErrorCondition(wandb, ...)
                                                      return errCondition
                                                    }
                                                    ...existing...
```

**Approach:** Compare current spec against last-known state stored in
`wandb.Status.XxxStatus` (or use a new annotation like
`wandb.io/last-applied-spec` for fields not in status).

**Files touched:**
- `controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go` — add
  pre-write immutability checks.
- Possibly extend `Status.XxxStatus` to surface the observed
  namespace/name (often already there in the connection refs).

---

#### Move 15: Surface `ApplyInfraSizing` mutations in status

**Why:** `ApplyInfraSizing` mutates user-facing spec fields (`Replicas`,
`StorageSize`, `Resources`) at reconcile time, but the user has no way to
see that happened. The webhook can't validate values it doesn't know about.

**Two options, prefer A:**

**Option A (recommended):** Move `ApplyInfraSizing` to the webhook as a
defaulter. Then the API server returns a fully-populated spec to the
user, the webhook validator sees the same values the reconciler does,
and `ApplyInfraSizing` can be deleted from the reconcile path.

**Option B (interim):** Keep `ApplyInfraSizing` at reconcile time but
record the applied values in `Status.AppliedInfraSizing` so they are
inspectable. This is lower risk but doesn't fix the root issue.

```
BEFORE                                          AFTER (Option A)
controller/v2/reconcile_v2.go (line 283)        webhook/v2/weightsandbiases_defaulter.go
  ApplyInfraSizing(wandb, manifest)               ApplyInfraSizing(wandb, manifest)
                                                  (manifest fetched by webhook)

                                                api/v2/weightsandbiases_types.go
                                                  (no schema changes)
```

**Risk for Option A:** High. The webhook would need to fetch the server
manifest at admission time, which adds external network dependency to
admission and may slow down the admission path. Consider gating on
"manifest available" — fall back to reconcile-time sizing if the manifest
fetch fails.

**Files touched (Option A):**
- `webhook/v2/weightsandbiases_defaulter.go` — call `ApplyInfraSizing`.
- `controller/v2/reconcile_v2.go` — remove call.
- New: manifest cache (avoid fetching on every admission).

**Files touched (Option B):**
- `api/v2/weightsandbiases_types.go` — add `Status.AppliedInfraSizing`.
- `controller/v2/sizing.go` (post-Move 1) — record values in status.

---

### Phase 4 — High-risk structural changes

#### Move 16: Add CRD fields for currently-hidden translator constants

**Why:** Some translator defaults have no corresponding CRD field — users
cannot override them. After Move 11 wants to relocate them to Layer 1,
they need fields to live in.

```
BEFORE                                          AFTER
api/v2/weightsandbiases_types.go                api/v2/weightsandbiases_types.go
  ManagedRedisSpec                                ManagedRedisSpec
    StorageSize string                              StorageSize string
    Sentinel RedisSentinelSpec                      Sentinel RedisSentinelSpec
    Telemetry Telemetry                             Replicas *int32              <-- NEW
                                                      // +kubebuilder:default=3
                                                    Telemetry Telemetry

  RedisSentinelSpec                               RedisSentinelSpec
    Enabled bool                                    Enabled bool
    Config RedisSentinelConfig                      Replicas *int32              <-- NEW
                                                      // +kubebuilder:default=3
                                                    Config RedisSentinelConfig

  RedisSentinelConfig                             RedisSentinelConfig
    MasterName string                               MasterName string
                                                      // +kubebuilder:default="gorilla"
    Resources corev1.ResourceRequirements           Resources corev1.ResourceRequirements
```

`MasterName` already exists in `RedisSentinelConfig` and is honored by
`translator/v2/redis.go:153`; only the default annotation is missing.

**Files touched:**
- `api/v2/weightsandbiases_types.go` — annotations and new fields.
- `translator/v2/redis.go` (or post-Move 3, `infra/managed/redis/opstree/spec.go`) —
  read from spec, drop hardcoded constants.
- `make manifests`, `make generate`.

**Risk:** Medium. New fields are additive (existing CRs unaffected).

---

#### Move 17: Decouple `api/v2/application_types.go` from KEDA / Argo Rollouts

**Why:** `application_types.go` imports `kedav1alpha1.ScaledObjectSpec`
and `argo v1alpha1.RolloutStatus` directly. Coupling the CRD schema to
two external operator APIs forces the schema to change whenever those
projects rev their APIs.

The Gateway API embedding (`gatewayv1.ParentReference`,
`gatewayv1.Hostname`) added since the previous analysis is more
defensible — Gateway API is upstream Kubernetes and is part of the
operator's deliberate networking story. It can stay.

```
BEFORE                                  AFTER
api/v2/application_types.go             api/v2/application_types.go
  import kedav1alpha1                     ScaledObjectTemplate *apiextv1.JSON
  import argo v1alpha1                    RolloutStatus        *apiextv1.JSON
  ScaledObjectTemplate *ScaledObjectSpec    (typed conversion happens in
  RolloutStatus *v1alpha1.RolloutStatus      controller/translator at use sites)
```

**Risk:** Highest-risk move. Changes CRD schema representation.
Requires CRD regeneration, careful migration of existing CRs (conversion
webhook or one-shot migration).

**Recommendation:** Defer to its own PR after the rest of the moves are
validated.

---

#### Move 18: Reconsider whether `ApplyInfraSizing` should mutate `Spec`

**Why:** Even if Move 15 is taken, mutating user-facing spec fields at
reconcile is surprising. A clearer model: keep the manifest-derived
sizing as a *separate input* to translator builders rather than baking
it into the spec.

```
BEFORE                                          AFTER
controller/v2/reconcile_v2.go                   controller/v2/sizing.go (post-Move 1)
  ApplyInfraSizing(wandb, manifest)               ResolveInfraSizing(wandb, manifest)
    spec.Replicas = sizing.Replicas               returns InfraSizingResult
    spec.StorageSize = sizing.VolumeSize          (without mutating spec)
    ...
                                                infra/managed/{vendor}/spec.go (post-Move 3)
                                                  ToVendorSpec(wandb, manifest, scheme)
                                                    reads spec OR sizing as fallback
```

**Risk:** High; intersects with Moves 1, 3, 11, 15. Last move to apply.

**Recommendation:** Defer to a follow-up after Phase 3 has stabilized.
This is more about data-flow clarity than an active bug.

---

## Dependency Graph (After all phases)

```
                api/v2
                (no KEDA/Argo deps after Move 17;
                 +kubebuilder:default for static defaults;
                 new fields for previously-hidden translator constants)
                  |
        +---------+---------+
        |                   |
   webhook/v2          (lateral peer)
     defaulter.go     <-- computed defaults only, possibly hosts
                          ApplyInfraSizing (Move 15A)
     validator.go     <-- per-service immutability checks for all infra
                  |
         translator/v2
           common.go (converters only)
                  |
        +-----------------+
        |                 |
   managed/{vendor}    external/
     spec.go            common.go (no InferExternalStatus)
     credentials.go     {service}/redis.go etc
     write.go           <-- writes connection secret
     read.go            <-- pure read
     status.go, ...
        |
   controller/v2
     reconcile_v2.go    <-- slim orchestration
     applications.go    <-- reconcileApplications loop
     rbac.go
     migrations.go
     mysql_init.go
     kafka_topics.go
     secrets.go
     sizing.go          <-- ResolveInfraSizing helpers
     env_resolution.go  <-- resolveEnvvars
     telemetry_inject.go
     telemetry_secret.go
     defaults.go        <-- reconciler fallback defaults
     external_status.go <-- InferExternalStatus
     gateway.go         <-- provider-agnostic
     ingress.go         <-- consolidated ingress
     infra_routes.go    <-- provider-agnostic
     networking_cleanup.go
     networking/providers/{nginx,gke}/  <-- conditionally registered
     state.go
        |
        +--> internal/telemetry/  <-- domain types
        |
        +--> pkg/wandb/manifest/   <-- includes telemetry.go
                                       (allowlist + envvar blueprint)
```

---

## Execution Order

| # | Move | Phase | Risk | Touches Tests | CRD Change |
|---|------|-------|------|--------------|------------|
| 1 | Split `reconcile_v2.go` | 1 | Low | No (same package) | No |
| 2 | Split `weightsandbiases_webhook.go` | 1 | Low | No | No |
| 3 | Vendor spec builders → `infra/managed/` | 1 | Medium | Update test imports | No |
| 4 | Telemetry types → `internal/telemetry/` | 1 | Low | Update imports | No |
| 5 | `InferExternalStatus` → `controller/v2/` | 1 | Low | Update imports | No |
| 6 | Telemetry allowlist → manifest layer | 1 | Low | No | No |
| 7 | Extract label patching | 1 | Low | No | No |
| 8 | Provider-gate networking imports | 1 | Medium | Networking tests | No |
| 9 | Fix write-in-read (4 vendors) | 2 | Medium-High | Yes | No |
| 10 | MySQL credentials → managed layer | 2 | Medium | Possibly | No |
| 11 | Static defaults → kubebuilder | 3 | Medium | Yes | Yes |
| 12 | Reconciler fallback defaults | 3 | Low | Yes | No |
| 13 | Non-Redis immutability validators | 3 | Low | Yes (new tests) | No |
| 14 | Reconciler immutability guards | 3 | Low | Yes (new tests) | No |
| 15 | Surface/move `ApplyInfraSizing` | 3 | Medium-High | Yes | No (Option A) / Yes (Option B) |
| 16 | New CRD fields for hidden constants | 4 | Medium | Yes | Yes |
| 17 | Decouple KEDA/Argo from CRD | 4 | High | Yes | Yes |
| 18 | Stop mutating spec in `ApplyInfraSizing` | 4 | High | Yes | No |

Phase 1 (Moves 1-8) can be parallelized across PRs since they touch
different files; some intersect (Move 1 + Move 6 both modify
`reconcile_v2.go`, so order them). Phase 2 (Moves 9-10) should be
sequential. Phase 3 (Moves 11-15) is mostly parallelizable but Move 11
needs `make manifests` and depends on Move 16 for fields that don't
exist. Phase 4 (Moves 16-18) goes last; each is its own PR.

Each move should be a separate PR with a focused commit message.

---

## What's Already Done Since the Previous Plan

Worth acknowledging — these don't need re-doing:

- **`infra/managed/minio/tenant/`** moved its connection-secret write into
  `WriteState` (the pattern Move 9 wants for the other four vendors).
- **External infra credential management** is implemented end-to-end
  under `infra/external/` with a clean per-service pattern.
- **A handful of `+kubebuilder:default` annotations** were added to
  `api/v2` — this is partial progress on Move 11.
- **`weightsandbiases_conversion.go`** added (groundwork for v1↔v2
  conversion).
- **Per-service defaulter test split** is complete (the implementation
  side of Move 2 is the only remaining piece).
- **Networking layer** (Gateway API + Ingress + cross-mode cleanup)
  added — but introduced new layer-violation issues (Move 8 addresses
  these).
- **`ApplyInfraSizing`** added — solves a real problem (manifest-driven
  sizing) but introduced the four-layer default issue (Moves 11, 15, 18).
