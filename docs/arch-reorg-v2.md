# Architecture: Separation of Concerns Analysis (v2)

## `internal/controller/{infra,translator,v2}` · `internal/webhook/v2` · `api/v2`

---

## 1. Intended Architecture (Inferred)

The system is designed as a layered pipeline from user intent to vendor infrastructure, with an admission gate that intercepts resources before they enter the reconcile loop:

```
api/v2              CRD schema — what the user writes in YAML
webhook/v2          Admission gate: mutate defaults, validate invariants
translator/         Internal wire types (connection shapes, status shapes)
translator/v2/      Adaptation layer: api/v2 ↔ translator, translator → vendor CRs
infra/external/     External credential management (user-provided services)
infra/managed/      Vendor-specific lifecycle management (operator-deployed services)
controller/v2/      Reconciliation orchestration and dispatching
```

The intended direction of dependency is strictly downward: `controller/v2` → `infra/` → `translator/` → `api/v2`. The webhook is a lateral peer of the reconciler — both depend on `api/v2`, but neither should depend on the other. Nothing in the lower layers should call back up.

```
                    ┌──────────────────┐
                    │  Kubernetes API   │
                    └──────┬───────────┘
                           │ admission request
                    ┌──────▼───────────┐
                    │   webhook/v2     │──── mutate + validate
                    └──────┬───────────┘
                           │ persisted CR
                    ┌──────▼───────────┐
                    │  controller/v2   │──── reconcile loop
                    └──┬──────────┬────┘
                       │          │
              ┌────────▼──┐  ┌───▼──────────┐
              │ infra/     │  │ translator/  │
              │ managed/   │  │ v2/          │
              │ external/  │  └──────────────┘
              └────────────┘
```

The webhook fires synchronously on CREATE/UPDATE before the object is persisted to etcd. The reconciler fires asynchronously after. This ordering means the reconciler can assume that any invariant enforced by the webhook already holds — but only if the webhook is enabled.

---

## 2. Layer-by-Layer Analysis

### `api/v2`

**Intended:** CRD schema only — the types a cluster admin writes in YAML.

**Actual:** `application_types.go` imports KEDA (`kedav1alpha1.ScaledObjectSpec`) and Argo Rollouts (`v1alpha1.RolloutStatus`) directly. These types appear in `ApplicationSpec.ScaledObjectTemplate` and `ApplicationStatus.RolloutStatus`.

**Issue:** CRD types should not encode workload-scheduling implementation concerns. `ApplicationSpec` embeds a full `ScaledObjectSpec` from KEDA and a `RolloutStatus` from Argo, coupling the CRD schema to two external operator dependencies. If either operator changes its API, the CRD schema is forced to change. These types are workload orchestration details — they belong in the manifest layer or in the translator, not in the API surface that cluster admins interact with.

---

### `internal/webhook/v2/`

**Intended:** Admission gate — apply sensible defaults on CREATE/UPDATE (mutating webhook) and reject invalid specs before they reach the reconciler (validating webhook).

**Actual:** Two webhook registrations, each with a defaulter and a validator:

1. **`WeightsAndBiasesCustomDefaulter`** — mutating webhook that sets defaults on the `WeightsAndBiases` CR.
2. **`WeightsAndBiasesCustomValidator`** — validating webhook that rejects invalid specs.
3. **`ApplicationCustomDefaulter`** — mutating webhook for `Application` (currently a no-op scaffold).
4. **`ApplicationCustomValidator`** — validating webhook that enforces HPA/replicas mutual exclusivity and kind immutability.

Both are conditionally registered in `cmd/main.go` (lines 341–352) when `--enable-webhooks` AND `--enable-v2` are true.

#### WeightsAndBiases Defaulter

The defaulter (`weightsandbiases_webhook.go`) applies the following defaults:

| Default | Value | Notes |
|---------|-------|-------|
| `Spec.Size` | `SizeDev` | — |
| `Spec.RetentionPolicy.OnDelete` | `DetachOnDelete` | — |
| `Spec.Affinity`, `Spec.Tolerations` | Empty structs | Prevents nil-pointer in downstream code |
| `Spec.Manifest.Repository` | `oci://us-docker.pkg.dev/wandb-production/public/wandb/server-manifest` | Adds `oci://` scheme if missing |
| `Spec.InternalServiceAuth.Enabled` | `true` | — |
| `Spec.InternalServiceAuth.OIDCIssuer` | `https://kubernetes.default.svc.cluster.local` | — |
| `Spec.ServiceAccount.Create` | `true` | — |
| `Spec.ServiceAccount.Name` | `wandb` | — |
| Per-service namespace | Parent CR's namespace | Applied to MySQL, Redis, Kafka, ObjectStore, ClickHouse |
| Per-service name | `{wandb-name}-{service}` | e.g. `my-wandb-mysql` |
| Redis sentinel | Enabled for non-dev sizes | `applyRedisDefaults()` |
| ObjectStore root user | `admin` | — |
| ObjectStore browser | `on` | — |

**Issue 1 — Defaults are split across three layers:**

Defaults for the same fields are set in up to three places:

| Default | Webhook (`weightsandbiases_webhook.go`) | Translator (`translator/v2/*.go`) | API (`api/v2/`) |
|---------|----------------------------------------|----------------------------------|-----------------|
| Redis sentinel group name `"gorilla"` | Not set | `translator/v2/redis.go:24` | Not annotated |
| Redis sentinel replica count `3` | Not set | `translator/v2/redis.go:145` | Not annotated |
| Redis replication replica count `3` | Not set | `translator/v2/redis.go:226` | Not annotated |
| Redis sentinel enabled (non-dev) | Set | Not set | Not annotated |
| Per-service namespace | Set | Not set | Not annotated |
| Per-service resource name | Set | Not set | Not annotated |
| ObjectStore root user `"admin"` | Set | Not set | Not annotated |
| Deployment size `SizeDev` | Set | Not set | Not annotated |

The webhook defaults fire on admission, so the reconciler sees a fully-defaulted spec. But the translator defaults fire during reconciliation, so they are invisible to the webhook validator. This creates a split-brain: the webhook cannot validate translator-applied defaults because they don't exist yet at admission time.

**Recommendation:** All user-visible defaults should live in exactly one place. The natural home is `+kubebuilder:default` annotations in `api/v2/` for simple scalar values, and the webhook defaulter for computed defaults (like sentinel-for-non-dev). The translator should not apply defaults to fields the user could have set — it should only translate already-defaulted values into vendor shapes.

**Issue 2 — Webhook is optional, but reconciler assumes defaulted fields:**

The webhook is gated behind `--enable-webhooks && --enable-v2`. When webhooks are disabled (e.g. during development, in certain deployment modes, or in tests that don't register them), the reconciler receives un-defaulted specs. If the reconciler relies on webhook-applied defaults (e.g. non-nil `Affinity`, non-empty `ServiceAccount.Name`), it will encounter nil pointers or empty strings.

**Recommendation:** The reconciler must be defensive against un-defaulted specs, OR the webhook must be unconditionally required. If the webhook is optional, every default it applies should also have a fallback in the reconciler or a `+kubebuilder:default` annotation.

#### WeightsAndBiases Validator

The validator enforces:

| Rule | Scope |
|------|-------|
| Mutual exclusivity: managed vs. external per infra type | CREATE, UPDATE |
| Redis storage size is a valid Kubernetes quantity | CREATE, UPDATE |
| Redis storage size is immutable once set | UPDATE |
| Redis namespace is immutable | UPDATE |
| Redis sentinel enabled/disabled is immutable | UPDATE |
| Networking mode consistency (Ingress vs. GatewayAPI config) | CREATE, UPDATE |
| GatewayAPI managed requires `gatewayClassName` | CREATE, UPDATE |
| GatewayAPI unmanaged requires `gatewayRef.name` | CREATE, UPDATE |
| Cert-manager TLS annotations only valid in Ingress mode | CREATE, UPDATE (warning) |

**Issue 3 — Immutability enforcement is only in the webhook:**

The redis storage size, namespace, and sentinel toggle are enforced as immutable only by the validating webhook (`validateRedisChanges`). If the webhook is disabled, a user can change these fields, and the reconciler will silently attempt to apply the change — potentially causing data loss or vendor operator conflicts. There is no reconciler-side guard.

**Recommendation:** Immutability constraints should be enforced in the validating webhook (the primary enforcement point) AND checked defensively in the reconciler (the fallback). The reconciler check can be a simple "if changed, set error condition and return" rather than a full validation pass.

**Issue 4 — No immutability enforcement for non-Redis infra types:**

Only Redis has update-time change validation (`validateRedisChanges`). MySQL, Kafka, ObjectStore, and ClickHouse have no equivalent. If changing these services' namespaces or storage sizes is equally dangerous, the same immutability rules should apply.

#### Application Validator

The validator enforces:
- HPA and replicas are mutually exclusive
- Application kind cannot change on update

**Assessment:** Well-scoped and correct. The `ApplicationCustomDefaulter` is currently empty, which is appropriate — Application resources are generated by the reconciler, not by users, so defaulting is less relevant.

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

**Additional issue — Defaults that bypass the webhook:**

Configuration defaults are hardcoded in this package:
- `DefaultSentinelGroup = "gorilla"` (`redis.go:24`)
- Default sentinel replica count: `3` (`redis.go:145`)
- Default replication replica count: `3` (`redis.go:226`)

These defaults are applied during vendor spec construction in the reconciler, *after* the webhook has already run. The webhook validator cannot see or validate them. If a user wants to override the sentinel group name or replica count, the CRD schema has no field for it — the value is buried in the translator.

**Recommendation:** Expose these as fields in `api/v2` with `+kubebuilder:default` annotations, or make them configurable through the existing spec fields. The translator should translate, not default.

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

## 3. Webhook ↔ Reconciler Contract

The webhook and reconciler are peers that share the `api/v2` types but have no direct code dependency. Their contract is implicit: the webhook shapes the spec before the reconciler sees it, and the reconciler assumes certain properties hold.

### Defaults Contract

| Guarantee | Provided By | Consumed By | Enforced If Webhook Disabled? |
|-----------|-------------|-------------|-------------------------------|
| `Spec.Size` is non-empty | Webhook defaulter | Reconciler (sentinel decision, resource sizing) | No |
| `Spec.RetentionPolicy.OnDelete` is non-empty | Webhook defaulter | Reconciler (finalizer selection) | No |
| `Spec.Affinity` is non-nil | Webhook defaulter | Translator (vendor spec construction) | No |
| Per-service namespace is populated | Webhook defaulter | Translator, managed infra | No |
| Per-service resource name is populated | Webhook defaulter | Translator, managed infra | No |
| Redis sentinel enabled for non-dev | Webhook defaulter | Translator (topology selection) | No |
| Sentinel group name `"gorilla"` | Translator | Managed infra (conn.go) | Yes (translator always runs) |
| Sentinel/replication replica count `3` | Translator | Vendor CR spec | Yes (translator always runs) |

The "No" column reveals a fragility: when webhooks are disabled, the reconciler may receive specs with zero-value fields it doesn't expect. This is a latent bug source, particularly in development and testing environments.

### Validation Contract

| Invariant | Enforced By | What Happens If Violated At Runtime? |
|-----------|-------------|--------------------------------------|
| Managed and external infra are mutually exclusive | Webhook validator | Reconciler routes to managed; external config is silently ignored |
| Redis storage size is a valid quantity | Webhook validator | `resource.ParseQuantity()` panics or returns error in translator |
| Redis storage size doesn't change | Webhook validator | Vendor operator may reject the change or silently ignore it |
| Redis namespace doesn't change | Webhook validator | Managed infra creates resources in new namespace, orphaning old ones |
| Redis sentinel toggle doesn't change | Webhook validator | Topology mismatch; potential data loss |
| Networking mode matches config | Webhook validator | Reconciler creates wrong resource type (Ingress vs HTTPRoute) |

Each row where the "What Happens" column describes silent corruption or data loss indicates a missing reconciler-side guard.

---

## 4. Mutation Access Map

| Object | Who Writes | Who Reads Back | Verdict |
|--------|-----------|---------------|---------|
| `WeightsAndBiases` spec (admission) | Webhook defaulter (mutates on admission) | Reconciler (reads persisted spec) | Correct — but only when webhook is enabled |
| `WeightsAndBiases.Status` | `controller/v2/*.go` via `client.Status().Update()` | Same package (next reconcile) | Correct — single writer |
| Connection Secrets | `infra/managed/*/conn.go` (from `ReadState`) | `infra/managed/*/read.go` returns pointer to secret keys | Naming violation — write in read path |
| External Connection Secrets | `infra/external/common.go:WriteConnectionSecret` | `infra/external/*/ReadState` → returns `translator.XxxConnection` | Correct |
| db-password Secret | `controller/v2/mysql.go:managedMysqlWriteState` | `translator/v2/mysql.go:ToMysqlMySQLVendorSpec` (reads secret name from spec) | Wrong layer — should be in managed infra |
| Vendor CRs (Redis/MySQL/Kafka/etc.) | `infra/managed/*/write.go` | `infra/managed/*/read.go` (reads status) | Correct |
| PVC/Pod labels | `infra/managed/*/write.go` (`ensurePVCLabels`, `ensurePodLabels`) | Not read back programmatically | Orthogonal to CR write — should be a shared util |
| HTTPRoutes | `controller/v2/infra_routes.go` | Not read back (Gateway controller handles them) | Correct |
| RBAC resources | `controller/v2/reconcile_v2.go` | Not read back | Correct |
| Application CRs | `controller/v2/reconcile_v2.go` | Application reconciler | Correct |
| Application workloads (Deployments, etc.) | `application_controller.go` | Not read back by WandB reconciler | Correct |

---

## 5. Downstream Mutation Reads (Cross-Reconcile Data Flow)

The reconcile loop is stateful across ticks via the `WeightsAndBiases.Status` subresource:

```
Admission (one-time, on CREATE/UPDATE):
  WeightsAndBiasesCustomDefaulter.Default()  →  mutates spec in-place
  WeightsAndBiasesCustomValidator.Validate()  →  rejects or admits
  ↓ (persisted to etcd)

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

## 6. Business Logic Placement

### Well-placed

- `webhook/v2/weightsandbiases_webhook.go` — mutual exclusivity validation is correctly in the admission gate
- `webhook/v2/weightsandbiases_webhook.go` — immutability enforcement on update is correctly in the validator
- `webhook/v2/application_webhook.go` — HPA/replicas mutual exclusivity is the right granularity for admission
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
| Computed defaults (sentinel-for-non-dev, resource naming) | `webhook/v2/weightsandbiases_webhook.go` only | Webhook + reconciler fallback, or `api/v2` annotations |
| Immutability guards for non-Redis infra | Not enforced anywhere | `webhook/v2/` validator (primary) + reconciler (fallback) |

### The Three-Layer Default Problem

Defaults currently live in three places, applied at different times:

```
Layer 1: api/v2       +kubebuilder:default annotations   → compile-time, in CRD schema
Layer 2: webhook/v2   CustomDefaulter.Default()           → admission-time, before persist
Layer 3: translator/v2 vendor spec builders               → reconcile-time, during translation
```

**Layer 1** is the most visible to users (shows up in `kubectl explain`) and the most reliable (always applied, even without webhooks). It is currently underused.

**Layer 2** is appropriate for computed defaults that depend on other fields (e.g. sentinel mode depends on size). It is currently doing work that Layer 1 should handle (e.g. static strings like `SizeDev`, `DetachOnDelete`).

**Layer 3** is invisible to users and the webhook validator. Defaults applied here cannot be overridden by users because the CRD has no field for them. It is currently setting defaults that should be in Layer 1 or Layer 2.

**Recommended policy:**
- **Static scalar defaults** → Layer 1 (`+kubebuilder:default`)
- **Computed defaults** (depend on other spec fields) → Layer 2 (webhook defaulter), with reconciler fallback
- **Translation-internal constants** (not user-facing, not overridable) → Layer 3, but rename them from "defaults" to "constants" to signal intent

---

## 7. File Organization Issues

### `webhook/v2/weightsandbiases_webhook.go` (522 lines)

This single file contains both the defaulter and the validator, plus all per-service default and validation functions. At 522 lines it is manageable, but the two concerns (defaulting vs. validation) and the five services create a natural split:

- `weightsandbiases_defaulter.go` — `Default()` + `applyXxxDefaults()`
- `weightsandbiases_validator.go` — `ValidateCreate/Update/Delete` + `validateXxxSpec()` + `validateXxxChanges()`

Or, per service:
- `weightsandbiases_defaults_mysql.go`, `weightsandbiases_defaults_redis.go`, etc.
- `weightsandbiases_validate_mysql.go`, `weightsandbiases_validate_redis.go`, etc.

The test files already follow the per-service split pattern (`weightsandbiases_defaulter_mysql_test.go`, etc.), suggesting the implementation should follow suit.

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

---

## 8. Summary of Cross-Cutting Concerns

### Concern: Where do defaults live?

| Severity | Issue |
|----------|-------|
| High | Three-layer default split makes it unclear where a value comes from |
| Medium | Translator defaults are invisible to webhook validation |
| Medium | Webhook-only defaults have no fallback when webhooks are disabled |

### Concern: Where are invariants enforced?

| Severity | Issue |
|----------|-------|
| High | Immutability rules exist only in webhook; no reconciler fallback |
| High | Non-Redis infra types have no immutability enforcement at all |
| Medium | Mutual exclusivity (managed vs. external) is only in webhook |

### Concern: Read/write boundary violations

| Severity | Issue |
|----------|-------|
| High | `ReadState` writes connection secrets on every reconcile |
| Medium | MySQL credential generation sits in the dispatcher, not managed infra |

### Concern: Layer violations

| Severity | Issue |
|----------|-------|
| Medium | `api/v2` imports KEDA and Argo Rollouts types |
| Medium | Vendor spec builders in translator import all vendor CRDs |
| Low | External status inference in wrong package |
| Low | Telemetry domain types in controller package |
