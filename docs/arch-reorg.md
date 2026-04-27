# Architecture: Separation of Concerns Analysis (v2)

## `internal/controller/{infra,translator,v2}` · `internal/webhook/v2` · `api/v2`

> Refresh: substantial new code has landed since the previous version of this
> document — networking (`gateway.go`, `infra_routes.go`, `ingress.go`,
> `networking_cleanup.go`), manifest-derived infra sizing
> (`ApplyInfraSizing` in `reconcile_v2.go`), telemetry connection secret
> reconciliation (`telemetry_secret.go`), and external infra credential
> management (`infra/external/`). This analysis reflects the current state.

---

## 1. Intended Architecture (Inferred)

```
api/v2              CRD schema — what the user writes in YAML
webhook/v2          Admission gate: mutate defaults, validate invariants
translator/         Internal wire types (connection shapes, status shapes)
translator/v2/      Adaptation: api/v2 ↔ translator, translator → vendor CRs
infra/external/     External credential management (user-provided services)
infra/managed/      Vendor-specific lifecycle management (operator-deployed)
controller/v2/      Reconciliation orchestration, dispatching, networking
pkg/wandb/manifest/ Server manifest definition (apps, sizing, topics, infra)
```

The intended dependency direction is downward:
`controller/v2` → `infra/` → `translator/` → `api/v2`.
The webhook is a lateral peer of the reconciler — both depend on `api/v2`,
but neither should depend on the other.

```
                    ┌──────────────────┐
                    │  Kubernetes API  │
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
              ┌────────▼──┐  ┌────▼──────────┐
              │ infra/    │  │ translator/v2 │
              │ managed/  │  │               │
              │ external/ │  │               │
              └───────────┘  └───────────────┘
```

The webhook fires synchronously on CREATE/UPDATE. The reconciler fires
asynchronously after persistence. The reconciler can assume webhook-applied
invariants — but only when the webhook is enabled (it is gated by
`--enable-webhooks` AND `--enable-v2`).

---

## 2. Layer-by-Layer Analysis

### `api/v2`

**Intended:** CRD schema only.

**Actual:**
- `weightsandbiases_types.go` — top-level `WeightsAndBiases` schema, including
  the new `NetworkingSpec` (`Mode`, `Ingress`, `GatewayAPI`, `TLS`,
  `Annotations`), `GatewayStatusSummary`, `IngressStatusSummary`.
- `application_types.go` still imports `kedav1alpha1.ScaledObjectSpec` and
  `argo v1alpha1.RolloutStatus` directly. Also now imports
  `gatewayv1` (Gateway API) for the new `HTTPRouteTemplateSpec` field
  (`Spec.HTTPRouteTemplate`) and `HTTPRouteStatusSummary`.
- `weightsandbiases_conversion.go` — added since the previous analysis.
- A handful of `+kubebuilder:default` annotations now exist:
  `RetentionPolicy.OnDelete=detach` (line 82), `GatewayConfig.Managed=false`
  (156), `ServiceAccount.Create=true` (260), `ServiceAccountName="wandb"`
  (262), `Telemetry.Enabled=true` (329), and empty-map defaults for
  `Applications` and `MySQLInit` (517, 522).

**Issue 1 — Vendor coupling (carried over):**
`application_types.go` embeds `*kedav1alpha1.ScaledObjectSpec` and
`*v1alpha1.RolloutStatus` from external operator APIs. Now also embeds
`gatewayv1.ParentReference` and `gatewayv1.Hostname` via
`HTTPRouteTemplateSpec`. The Gateway API embedding is more defensible
(Gateway API is upstream Kubernetes), but the KEDA/Argo couplings still
force the CRD schema to track external operator API changes.

**Issue 2 — Defaults overlap with the webhook:**
`+kubebuilder:default="detach"` exists on `RetentionPolicy.OnDelete`, yet
the webhook *also* sets `Spec.RetentionPolicy.OnDelete = appsv2.DetachOnDelete`
(`weightsandbiases_webhook.go:76`). Same for `ServiceAccount.Create=true`
and `ServiceAccountName="wandb"`. The webhook either runs after the API
server's defaults (in which case its assignments are no-ops on those fields),
or it runs in a context where the defaults haven't been applied yet —
either way, having both is a lurking source of confusion.

---

### `internal/webhook/v2/`

**Intended:** Admission gate.

**Actual:** Still a single 522-line `weightsandbiases_webhook.go` file
containing both the defaulter and the validator, plus all per-service helper
functions. Test files already follow per-service splits
(`weightsandbiases_defaulter_{mysql,redis,kafka,objectstore,clickhouse}_test.go`).

#### WeightsAndBiases Defaulter

Defaults applied in the webhook:

| Default | Value |
|---------|-------|
| `Spec.Size` | `SizeDev` |
| `Spec.RetentionPolicy.OnDelete` | `DetachOnDelete` (also annotation) |
| `Spec.Affinity`, `Spec.Tolerations` | Empty struct/slice (nil-pointer guard) |
| `Spec.Wandb.ManifestRepository` | `oci://us-docker.pkg.dev/...` (and prepends `oci://` if scheme missing) |
| `Spec.Wandb.InternalServiceAuth.Enabled` | `true` |
| `Spec.Wandb.InternalServiceAuth.OIDCIssuer` | `https://kubernetes.default.svc.cluster.local` |
| `Spec.Wandb.ServiceAccount.Create` | `true` (also annotation) |
| `Spec.Wandb.ServiceAccount.ServiceAccountName` | `"wandb"` (also annotation) |
| `Spec.Wandb.Applications` map (status) | empty map |
| Per-service `Namespace` | parent CR's namespace |
| Per-service `Name` | `{wandb-name}-{service}` |
| Redis sentinel | enabled when `Size != SizeDev` |
| ObjectStore `RootUser` | `"admin"` |
| ObjectStore `MinioBrowserSetting` | `"on"` |

There is also a typo in `Default()`: `applyMySQLDefaults(wandb)` is called
twice (lines 117 and 121); `applyObjectStoreDefaults` is in between but
otherwise harmless.

**Issue 3 — Defaults are now split across four layers, not three:**

Since the previous analysis, manifest-derived sizing has been added.
Defaults for the same field can come from up to four places, applied at
different times:

| Layer | When | Visible to user? |
|-------|------|------------------|
| 1. `api/v2` `+kubebuilder:default` | Object create (API server) | Yes (`kubectl explain`) |
| 2. `webhook/v2` defaulter | Admission | Yes if webhook enabled |
| 3. `controller/v2/reconcile_v2.go:ApplyInfraSizing` | Each reconcile, reads `manifest.Mysql/Redis/Kafka/...` | No |
| 4. `translator/v2/*` builders | Each reconcile, vendor-spec construction | No |

`ApplyInfraSizing` (called from `Reconcile`, line 283) reads sizing from
the server manifest and assigns to `wandb.Spec.MySQL.ManagedMysql.Replicas`,
`spec.StorageSize`, `spec.Config.Resources`, etc. — but only when the user
has not set them. This is mutating *user-facing spec fields* during reconcile
based on an external artifact (the server manifest). The webhook doesn't
see these values and can't validate them.

**Issue 4 — Webhook is optional, but reconciler assumes defaulted fields:**

The webhook is gated behind `--enable-webhooks && --enable-v2`. When
disabled, the reconciler may receive specs with empty per-service `Name`
and `Namespace`, no `Affinity`/`Tolerations`, etc. `ApplyInfraSizing`
partially papers over the storage/replicas case, but per-service
`Name`/`Namespace` defaulting is still webhook-only.

#### WeightsAndBiases Validator

Same as before: per-service mutual exclusivity (managed vs. external),
Redis storage size quantity check, Redis storage/namespace/sentinel
immutability on UPDATE, and networking-mode consistency
(`validateNetworkingSpec` — Ingress vs. GatewayAPI mode mismatch, managed
gateway requires `gatewayClassName`, unmanaged requires `gatewayRef.name`,
cert-manager TLS warning when no mode is set).

**Issue 5 — Immutability still only enforced for Redis:**
MySQL, Kafka, ObjectStore, and ClickHouse have no equivalent of
`validateRedisChanges`. Changing namespace or storage size on those
services would orphan the underlying vendor CR or silently fail.

#### Application Validator

Unchanged: HPA/replicas mutual exclusivity, kind immutability.
`ApplicationCustomDefaulter.Default()` is still an empty stub.

---

### `internal/controller/translator/`

**Intended:** Pure data shapes.

**Actual:** Still pure: `InfraStatus`, `MysqlConnection`, `MysqlStatus`,
`RedisConnection`, `RedisStatus`, `KafkaConnection`, `KafkaStatus`,
`ObjectStoreConnection`, `ObjectStoreStatus`, `ClickHouseConnection`,
`ClickHouseStatus`, `OnDeletePolicy`, `OnDeleteRule`. Module-name and
image constants live here too. **Assessment: Well-designed; unchanged.**

---

### `internal/controller/translator/v2/`

**Intended:** Adaptation between `api/v2` and `translator`, plus vendor CR
construction.

**Actual:** Same two responsibilities mixed in one package:

1. **Bidirectional converters** (`common.go`, ~225 lines) — shallow structural
   transforms with no business logic.
2. **Vendor spec builders** (per-service files) — `ToMysqlMySQLVendorSpec`
   (`mysql.go`), `ToRedisStandalone/Sentinel/ReplicationVendorSpec`
   (`redis.go`), `ToKafkaVendorSpec`/`ToKafkaNodePoolVendorSpec` (`kafka.go`),
   `ToObjectStoreVendorSpec`/`ToObjectStoreEnvConfig` (`objectstore.go`),
   `ToClickHouseVendorSpec` (`clickhouse.go`).

The vendor spec builders import every vendored CRD; the converters import
nothing beyond `api/v2` and `translator`. They should be different packages,
or the builders should live next to the managed-infra code that calls them.

**Issue 6 — Hardcoded translator defaults persist:**

`redis.go` still hardcodes:
- `DefaultSentinelGroup = "gorilla"` (`redis.go:25`)
- Sentinel size `int32(3)` (`redis.go:149`, with `// TODO I dont think we want to default this at all?` comment)
- Replication size `int32(3)` (`redis.go:233`, same TODO)

These defaults are invisible to webhook validation and to the user — there
is no CRD field exposing them.

`mysql.go:41` hardcodes a `mycnf` block (binlog format, sort buffer size,
etc.) with no override path. Likewise `clickhouse.go` hardcodes the user
settings (`ClickHouseUser`, `ClickHousePassword` constants — sourced from
`infra/managed/clickhouse/altinity`). These are configuration constants
masquerading as translation logic.

---

### `internal/controller/infra/external/`

**Intended:** Manage credentials for user-provided (external) services.

**Actual:** New since the previous analysis. Each service sub-package
(`redis/`, `mysql/`, `kafka/`, `clickhouse/`, `objectstore/`) follows a
consistent pattern: `WriteState()`, `ReadState()`, `DeleteConnectionSecret()`.
Shared utilities live in `external/common.go`: `ResolveSecretKey`,
`WriteConnectionSecret`, `ReadConnectionSecret`, `ResolveFields`,
`BuildWandbOwnerRef`, `DeleteConnectionSecret`, `InferExternalStatus`.

**Issue 7 — `InferExternalStatus` in the wrong layer:**
`external/common.go:InferExternalStatus` (line 134) computes health state
from connection presence. It is called from
`controller/v2/{redis,mysql,kafka,clickhouse,objectstore}.go`'s
`externalXxxInferStatus()` functions — never from within the external
infra layer itself. Belongs in `controller/v2/`.

---

### `internal/controller/infra/managed/{vendor}/`

**Intended:** Vendor-specific CR lifecycle.

**Actual:** Each vendor sub-package has the same file structure:
`naming.go`, `conn.go`, `read.go`, `write.go`, `detach.go`, `purge.go`,
`status.go`, plus per-vendor extras (`values.go` for redis/kafka/clickhouse,
`config.go` for minio, `restore.go` for kafka).

**Issue 8 — Write-in-read remains for four of five vendors:**

`read.go` still calls `writeXxxConnInfo()` to (re)create the connection
Secret on every reconcile, in:

- `managed/redis/opstree/read.go:108,149`
- `managed/mysql/mysql/read.go:109`
- `managed/kafka/strimzi/read.go:108`
- `managed/clickhouse/altinity/read.go:101`

**`managed/minio/tenant/` is the exception** — the connection Secret write
moved into `WriteState` (via `writeMinioConfig` → `writeWandbConnInfo`,
`write.go:111`), and `read.go` only inspects status. This is the pattern
the other four should follow. (The `objectStoreWriteState` dispatcher in
`controller/v2/objectstore.go` even returns the connection from
`WriteState` directly, so `ReadState` for ObjectStore returns no
connection at all — it has been simplified.)

**Issue 9 — Cross-cutting label patching bundled in `write.go`:**

`managed/redis/opstree/write.go:84-152` contains `ensurePVCLabels()` and
`ensurePodLabels()`. These patch labels on resources created by the
opstree operator (matching prefixes `redis-data-{crName}-` and
`{crName}-`). MySQL has equivalent inline label code. These belong in
`internal/controller/common/` (already exists with `HasAllLabelKeys`)
or per-package `labels.go`.

**Well-placed:** `status.go` in each vendor package — vendor-specific
condition/health mapping correctly close to the vendor.
`detach.go`/`purge.go`/`naming.go` — finalizer logic and naming strategy
correctly isolated.

---

### `internal/controller/v2/`

**Intended:** Top-level orchestration and dispatch.

**Actual:** Per-component dispatcher files (`redis.go`, `mysql.go`,
`kafka.go`, `clickhouse.go`, `objectstore.go`) are well-structured. The
problems are concentrated in `reconcile_v2.go` and the recently-added
networking files.

**File sizes** (current):

| File | Lines | What it does |
|------|------:|--------------|
| `reconcile_v2.go` | 2592 | Main orchestrator + RBAC + secrets + topics + migration + sizing + telemetry inject + app deploy |
| `infra_routes.go` | 465 | Gateway API HTTPRoutes for managed infra (objectstore, clickhouse) |
| `gateway.go` | 352 | Gateway resource lifecycle |
| `mysql.go` | 262 | Dispatcher + MySQL credential generation (~65 lines) |
| `redis.go` | 225 | Dispatcher (clean) |
| `kafka.go` | 226 | Dispatcher (clean) |
| `objectstore.go` | 218 | Dispatcher (clean — uses connection-from-WriteState pattern) |
| `clickhouse.go` | 200 | Dispatcher (clean) |
| `ingress.go` | 201 | Consolidated Ingress reconciliation |
| `telemetry_secret.go` | 117 | Telemetry connection secret reconciliation |
| `telemetry_config.go` | 91 | Domain types: `TelemetryRuntimeConfig`, `TelemetryEndpoints`, `TelemetryOTelConfig` |
| `networking_cleanup.go` | 57 | Cross-mode cleanup |
| `manifest_order.go` | 25 | Sorted iteration helpers |

**Issue 10 — `reconcile_v2.go` has roughly doubled in scope.**

It now orchestrates:
1. Finalizer dispatch (lines 137-268)
2. Manifest fetch + feature override + `ApplyInfraSizing` (270-283)
3. Infra write/read/inferStatus dispatch (285-365)
4. Telemetry connection secret reconciliation (339-342)
5. `ReconcileWandbManifest`: secret generation, kafka topics,
   mysql init, RBAC, networking-mode cleanup, gateway, migrations,
   applications, infra HTTPRoutes, ingress (382-485)
6. `reconcileApplications`: per-app loop, env-var resolution, volume
   resolution, container build, Service template, HTTPRoute template,
   stale-app pruning, Ingress rebuild, hostname resolution (487-682)
7. Sizing helpers `ResolveResources`, `ResolveAutoscaling`,
   `ResolveInfraSizing`, `ResolveKafkaSizing`, `ApplyInfraSizing`,
   `mergeResources` (764-1054)
8. `resolveContainers`, `resolveInitContainers`,
   `applyWorkloadTelemetryDefaults`, `injectManagedWorkloadTelemetryEnvvars`,
   `appendMissingEnvVars`, `hasWorkloadTelemetryConfig` (1056-1235)
9. JWT/inline-files/volume resolution (1237-1412)
10. `resolveEnvvars` — multi-source env composition with intermediate
    variables, supporting `mysql`/`redis`/`kafka`/`clickhouse`/`bucket`/
    `telemetry`/`generatedSecret`/`service`/`jwt-issuer-map`/`custom-resource`
    (1414-1693)
11. `resolveVolumeMounts` (1695-1738)
12. `runMysqlInitJob` (1740-1866)
13. `runMigrations` (1868-2028)
14. `createKafkaTopics` (2030-2125)
15. `generateSecrets` (2127-2207)
16. `manifestFeaturesEnabled`,
    `resolveServiceURLFromManifest`,
    `resolveServicePortFromManifest`,
    `resolveCRFieldString`,
    `inferState` (2209-2346)
17. `createOrUpdateServiceAccount`,
    `createOrUpdateRole`,
    `createOrUpdateRoleBinding`,
    `createOrUpdateOIDCDiscoveryClusterRoleBinding`,
    `isOwnedBy` (2348-2592)

Plus the package-level constants at the top:
- `managedWorkloadTelemetryApplications` map (lines 58-72) — 13 hard-coded
  application names; must be edited whenever a new app is added.
- `managedWorkloadTelemetryEnvVars` slice (lines 74-135) — 11 env vars
  describing how OTLP env should be resolved from the telemetry secret.

**Issue 11 — Networking files at orchestration level pull in vendor CRDs:**

- `gateway.go:9` imports `nginxGatewayv1alpha1` from
  `github.com/nginx/nginx-gateway-fabric` to handle nginx-specific gateway
  policies.
- `infra_routes.go:9` imports `gkeGatewayApiNetworkingv1` from
  `github.com/GoogleCloudPlatform/gke-gateway-api` to handle GKE-specific
  `HealthCheckPolicy` resources (e.g. `infra_routes.go:198-241`).

This is the same shape of problem as the KEDA/Argo coupling in `api/v2`,
but at the orchestrator level. Vendor-specific behavior (nginx, GKE)
should at least be feature-gated or factored into provider sub-packages,
otherwise every operator deployment carries the union of vendor CRDs.

**Issue 12 — MySQL credential generation in the dispatcher:**

`controller/v2/mysql.go:102-168` (~65 lines) still generates a random root
and user password and creates a `{name}-db-password` Secret before
delegating to `mysql.WriteState`. The managed MySQL package
(`infra/managed/mysql/mysql/`) does not know about these credentials —
the secret name is plumbed through `translator/v2/mysql.go:58`
(`SecretName: fmt.Sprintf("%s-%s", spec.Name, "db-password")`). This
remains misplaced; the credential lifecycle should be co-located with the
MySQL resource lifecycle.

**Issue 13 — Telemetry types and allowlist still in controller package:**

- `telemetry_config.go` defines `TelemetryRuntimeConfig`,
  `TelemetryOTelConfig`, `TelemetryEndpoints` — these are configuration
  domain types, not controller plumbing. They are referenced from
  `cmd/main.go` and `weightsandbiases_controller.go` (`TelemetryConfig` field)
  in addition to `reconcile_v2.go` and `telemetry_secret.go`.
- `managedWorkloadTelemetryApplications` map (`reconcile_v2.go:58-72`)
  is configuration-as-code for which applications receive OTLP env vars.
  It belongs in the manifest layer (`pkg/wandb/manifest/`) — when a new
  app is added to the manifest, this map must be updated in lockstep.

---

## 3. Webhook ↔ Reconciler Contract

The implicit contract is unchanged in shape, but the addition of
`ApplyInfraSizing` makes the picture more nuanced.

### Defaults Contract

| Guarantee | Provided By | Consumed By | Holds when webhook disabled? |
|-----------|-------------|-------------|------------------------------|
| `Spec.Size` is non-empty | Webhook | Sentinel decision; sizing lookup | No (downstream code falls through "default" sizing entry) |
| `Spec.RetentionPolicy.OnDelete` non-empty | API server (annotation) + Webhook | Finalizer dispatch | **Yes** (annotation) |
| `Spec.Affinity` non-nil | Webhook | Translator (vendor specs deref) | No (nil-deref risk) |
| Per-service `Namespace` populated | Webhook | Translator, managed infra | No |
| Per-service `Name` populated | Webhook | Translator, managed infra | No |
| Redis sentinel enabled for non-dev | Webhook | Translator topology selection | No |
| `Spec.Wandb.ServiceAccount.Create` set | Annotation + Webhook | Reconciler dereferences `*spec.Create` | **Yes** (annotation) |
| `Spec.Wandb.ServiceAccount.ServiceAccountName` set | Annotation + Webhook | Reconciler | **Yes** (annotation) |
| MySQL/Redis/Kafka/ObjectStore/CH replica/storage/resources defaults | `ApplyInfraSizing` from manifest | Translator | **Yes** (sizing always runs) |
| Sentinel group name `"gorilla"` | Translator | Managed infra | **Yes** (translator always runs) |
| Sentinel/replication replica count `3` | Translator | Vendor CR spec | **Yes** (translator always runs) |
| MySQL `mycnf` block | Translator | Vendor CR spec | **Yes** |

`ApplyInfraSizing` doesn't *fix* the webhook-required defaults — it adds
*more* fields where defaults can come from a non-webhook source. A user
who edits the manifest sizing entries can affect every CR in the cluster
on the next reconcile, with no admission validation.

### Validation Contract

| Invariant | Enforced By | If violated at runtime |
|-----------|-------------|------------------------|
| Managed and external mutually exclusive | Webhook | Reconciler routes to managed; external silently ignored |
| Redis storage size is a valid quantity | Webhook | `resource.MustParse` panics in `translator/v2/clickhouse.go` (uses MustParse for ClickHouse), translator returns error for Redis |
| Redis storage size/namespace/sentinel immutable | Webhook | Operator may reject change or orphan resources |
| Networking mode matches config | Webhook | Wrong resource type created (Ingress vs HTTPRoute) |
| Networking gatewayClassName/gatewayRef per managed flag | Webhook | Reconciler may panic on nil pointer or create unmanaged Gateway |
| MySQL/Kafka/ObjectStore/CH namespace/storage immutable | **Not enforced** | Resources orphaned, data lost |

The networking validations are the only new admission rules added since the
previous analysis; they are well-shaped but, like all webhook rules, gated
behind `--enable-webhooks`.

---

## 4. Mutation Access Map

| Object | Who Writes | Who Reads Back | Verdict |
|--------|-----------|---------------|---------|
| `WeightsAndBiases` spec | Webhook defaulter; **also `ApplyInfraSizing` at reconcile time** | Reconciler | Webhook write is correct; reconcile-time spec mutation is novel and surprising |
| `WeightsAndBiases.Status` | `controller/v2/*.go` via `client.Status().Update()` | Same package | Correct — single writer |
| `Status.GeneratedSecrets` | `generateSecrets` (reconcile_v2.go) | `resolveEnvvars` cross-tick | Correct — explicit cache |
| `Status.GatewayStatus` / `IngressStatus` | `gateway.go` / `ingress.go` | Same package | Correct |
| Connection Secret (managed) | Mostly `infra/managed/*/conn.go` from `read.go` (**redis, mysql, kafka, clickhouse**); from `write.go` for **minio** | Cross-tick via `Status.XxxStatus.Connection` | Naming violation for 4 of 5 |
| Connection Secret (external) | `infra/external/common.go:WriteConnectionSecret` | `infra/external/*/ReadState` | Correct |
| `db-password` Secret | `controller/v2/mysql.go:managedMysqlWriteState` | `translator/v2/mysql.go` reads name; `infra/managed/mysql/mysql/read.go` reads value | Wrong layer — credential lifecycle in dispatcher |
| Vendor CRs (Redis/MySQL/Kafka/MinIO/CH) | `infra/managed/*/write.go` | `infra/managed/*/read.go` | Correct |
| Telemetry Secret (`wandb-otel-connection`) | `controller/v2/telemetry_secret.go:reconcileTelemetryConnectionSecret` | Workload pods via env-var resolution | Correct, but the input config (`TelemetryRuntimeConfig`) is in the wrong package |
| MySQL init Job | `controller/v2/reconcile_v2.go:runMysqlInitJob` | Same package next tick | Correct |
| Migration Jobs | `controller/v2/reconcile_v2.go:runMigrations` | Same package next tick | Correct |
| RBAC (SA/Role/RoleBinding/ClusterRoleBinding) | `controller/v2/reconcile_v2.go` | Not read back | Correct |
| `KafkaTopic` resources | `controller/v2/reconcile_v2.go:createKafkaTopics` | Not read back | Correct |
| `Application` CRs | `controller/v2/reconcile_v2.go:reconcileApplications` | `application_controller.go` | Correct |
| `Gateway` CR | `controller/v2/gateway.go` | Same package via Watch | Correct |
| `HTTPRoute` (apps) | `application_controller.go` | Application reconciler | Correct |
| `HTTPRoute` (infra: minio, clickhouse) | `controller/v2/infra_routes.go:reconcileInfraHTTPRoutes` | Not read back | Correct |
| `HealthCheckPolicy` (GKE) | `controller/v2/infra_routes.go` | Same package | Correct, but ties `controller/v2` to a GKE-specific CRD |
| `Ingress` (consolidated) | `controller/v2/ingress.go:reconcileConsolidatedIngress` | Same package | Correct |
| `ConfigMap` (inline files) | `controller/v2/reconcile_v2.go:resolveInlineFiles` | Mounted into pods | Correct |
| PVC/Pod labels | `infra/managed/{redis,mysql}/write.go` | Not read programmatically | Cross-cutting concern bundled in CR write |

---

## 5. Downstream Mutation Reads (Cross-Reconcile Data Flow)

```
Admission (one-time, on CREATE/UPDATE):
  +kubebuilder:default annotations → applied by API server
  WeightsAndBiasesCustomDefaulter.Default()  → mutates spec
  WeightsAndBiasesCustomValidator.Validate() → reject or admit
  ↓ persisted to etcd

Tick N:
  Reconcile starts
    ApplyInfraSizing                        → mutates spec (sizing from manifest)
                                              [in-memory only; not persisted]
    {component}WriteState                   → vendor CR create/update (and for
                                              minio: connection Secret too)
    {component}ReadState                    → vendor CR status
                                              (redis/mysql/kafka/clickhouse:
                                               WRITES connection Secret)
    {component}InferStatus                  → status.Update()
  ReconcileWandbManifest
    reconcileTelemetryConnectionSecret      → Secret CRUD
    generateSecrets                         → status.GeneratedSecrets cache
    createKafkaTopics                       → KafkaTopic CRUD
    runMysqlInitJob                         → Job CRUD
    cleanupNetworkingModeResources          → cross-mode resource deletion
    reconcileGateway (Gateway mode)         → Gateway CRUD + status
    runMigrations                           → Job CRUD
    reconcileApplications                   → Application CRUD + Ingress
    reconcileInfraHTTPRoutes (Gateway mode) → HTTPRoute + HealthCheckPolicy CRUD

Tick N+1:
  inferStatus reads previous tick's wandb.Status.XxxStatus.Connection;
  resolveEnvvars reads cached secret selectors; HTTPRoute references
  Application services that may not exist yet.
```

The status-as-cache pattern (connection selectors in
`Status.XxxStatus.Connection`, generated-secret selectors in
`Status.GeneratedSecrets`) is intentional and correct — `utils.Coalesce`
in each `InferStatus` provides a fallback when the current tick can't
resolve a connection. It is a deliberate design decision that should be
documented.

---

## 6. Business Logic Placement

### Well-placed

- `webhook/v2/weightsandbiases_webhook.go` — mutual exclusivity, networking
  consistency, and the existing Redis immutability rules are correctly in
  the admission gate.
- `application_webhook.go` — HPA/replicas mutual exclusivity, kind
  immutability.
- `managed/*/status.go` — vendor condition→state mapping close to the vendor.
- `managed/*/detach.go`, `managed/*/purge.go`, `managed/*/naming.go` —
  finalizer logic and naming strategy correctly isolated per vendor.
- `translator/v2/common.go` — bidirectional converters are shallow, stable,
  and correctly placed.
- `controller/v2/{redis,kafka,clickhouse,objectstore}.go` —
  dispatcher-only, clean. (MySQL is the exception — see below.)
- `controller/v2/manifest_order.go` — small, focused helpers for ordering.
- `controller/v2/infra_routes.go` — Gateway API HTTPRoute orchestration
  for infra is at the right layer; the GKE-specific HealthCheckPolicy
  inside it is the layer-violation, not the file's existence.
- `infra/managed/minio/tenant/{write,read}.go` — exemplar of the
  read=pure / write=side-effects boundary.

### Misplaced

| Logic | Current Location | Should Be |
|-------|-----------------|-----------|
| MySQL credential generation | `controller/v2/mysql.go:102-168` | `infra/managed/mysql/mysql/credentials.go` |
| Connection secret writes (redis/mysql/kafka/clickhouse) | called from `infra/managed/*/read.go` | called from `write.go` (matching minio) |
| Telemetry domain types | `controller/v2/telemetry_config.go` | `internal/telemetry/` or `pkg/telemetry/` |
| Telemetry application allowlist | `controller/v2/reconcile_v2.go:58-72` | `pkg/wandb/manifest/` |
| Telemetry env-var blueprint | `controller/v2/reconcile_v2.go:74-135` | `pkg/wandb/manifest/` or `internal/telemetry/` |
| Sentinel group name `"gorilla"` | `translator/v2/redis.go:25`, `infra/managed/redis/opstree/conn.go` | `api/v2` annotation or spec field |
| Sentinel/replication replica defaults `3` | `translator/v2/redis.go:149,233` | `api/v2` field with `+kubebuilder:default=3` |
| MySQL `mycnf` block | `translator/v2/mysql.go:41-50` | `api/v2` field, or manifest-driven, with sane default |
| External status inference | `infra/external/common.go:InferExternalStatus` | `controller/v2/external_status.go` |
| Computed defaults (sentinel-for-non-dev, per-service Name/Namespace) | Webhook only | Webhook + reconciler fallback |
| Immutability for non-Redis infra | Not enforced anywhere | Webhook + reconciler |
| `ApplyInfraSizing` (manifest-derived spec defaults) | `controller/v2/reconcile_v2.go:875-1054` | Either move sizing to the webhook (so it's visible to validation), or keep at reconcile but extract to `controller/v2/sizing.go` and document that it mutates spec |
| `nginxGatewayv1alpha1` (nginx-specific gateway) | `controller/v2/gateway.go` | Provider-specific sub-package, feature-gated |
| `gkeGatewayApiNetworkingv1` (GKE HealthCheckPolicy) | `controller/v2/infra_routes.go` | Provider-specific sub-package, feature-gated |
| `RBAC` (SA/Role/RoleBinding/ClusterRoleBinding) creation | `controller/v2/reconcile_v2.go:2348-2583` | `controller/v2/rbac.go` (within same package) |
| Migration job orchestration | `controller/v2/reconcile_v2.go:1868-2028` | `controller/v2/migrations.go` |
| MySQL init job orchestration | `controller/v2/reconcile_v2.go:1740-1866` | `controller/v2/mysql_init.go` |
| Kafka topic creation | `controller/v2/reconcile_v2.go:2030-2125` | `controller/v2/kafka_topics.go` |
| Generated secrets | `controller/v2/reconcile_v2.go:2127-2207` | `controller/v2/secrets.go` |
| Env-var/volume resolution | `controller/v2/reconcile_v2.go:1414-1738` | `controller/v2/env_resolution.go` |
| Application reconciliation loop | `controller/v2/reconcile_v2.go:487-682` | `controller/v2/applications.go` |
| Sizing helpers | `controller/v2/reconcile_v2.go:764-1054` | `controller/v2/sizing.go` |

### The Four-Layer Default Problem

```
Layer 1: api/v2 +kubebuilder:default        compile-time, in CRD schema
Layer 2: webhook/v2 CustomDefaulter          admission-time, before persist
Layer 3: ApplyInfraSizing (reconcile_v2.go)  reconcile-time, from manifest
Layer 4: translator/v2 vendor builders       reconcile-time, in code
```

`ApplyInfraSizing` is the new layer. It lives between the webhook and the
translator: it mutates the user-facing spec at reconcile time using values
from the server manifest. The webhook cannot validate values it places
there, and the user has no way to inspect them via `kubectl describe`
beyond observing the mutated spec.

**Recommended policy:**
- **Static scalar defaults** → Layer 1 (`+kubebuilder:default`).
- **Computed defaults** depending on other fields → Layer 2 (webhook),
  with reconciler fallback for the webhook-disabled case.
- **Manifest-derived sizing** → Layer 3, but renamed from "defaults"
  to "manifest-derived sizing" with explicit doc that it mutates spec at
  reconcile time, and exposed in status (`Status.AppliedSizing` or similar)
  so the user can see what was applied.
- **Translator-internal constants** → Layer 4, but renamed to "constants"
  if not user-facing, OR pulled up to Layer 1/2 if they should be.

---

## 7. File Organization Issues

### `webhook/v2/weightsandbiases_webhook.go` (522 lines, unchanged)

Same recommendation as before: split into
- `weightsandbiases_defaulter.go` — `Default()` + `applyXxxDefaults()`
- `weightsandbiases_validator.go` — `ValidateCreate/Update/Delete` +
  `validateXxxSpec()` + `validateXxxChanges()` + `validateNetworkingSpec()`

The test files already follow per-service patterns (defaulter tests already
split per service); the implementation should follow.

### `controller/v2/reconcile_v2.go` (2592 lines)

This is the single largest file in the codebase and has accumulated
responsibility for:

- Top-level reconcile flow
- All finalizer dispatch
- Manifest fetch
- `ApplyInfraSizing` and all sizing helpers (~190 lines)
- Telemetry env-var injection plumbing (~70 lines including the const
  `managedWorkloadTelemetryEnvVars` slice)
- Application reconciliation loop and helpers (~200 lines)
- Container/volume/init-container resolution
- JWT/inline-files volume resolution (~180 lines)
- `resolveEnvvars` multi-source env composition (~280 lines)
- Volume mount resolution
- MySQL init job orchestration (~125 lines)
- Migration job orchestration (~160 lines)
- Kafka topic creation (~95 lines)
- Generated secret creation (~80 lines)
- RBAC creation (SA/Role/RoleBinding/ClusterRoleBinding) (~240 lines)
- `inferState` helper

Each section can become its own file within the same package; nothing
about the public API changes.

### `infra/managed/{redis,mysql,kafka,clickhouse}/conn.go`

These files mix two responsibilities: parsing connection details out of a
vendor CR (`readXxxConnectionDetails`) and writing a Kubernetes Secret
(`writeXxxConnInfo`). Because the secret-write is invoked from `read.go`,
the read/write boundary is already crossed. The fix is to move the write
call into `write.go` (matching `infra/managed/minio/tenant/`) and have
`write.go` return the connection reference for the dispatcher.

### `infra/managed/{redis,mysql}/write.go`

PVC/Pod label patching (`ensurePVCLabels`, `ensurePodLabels` in opstree;
similar inline code in mysql) is bundled with vendor CR writes. These are
label-maintenance operations on resources created by the vendor operator;
they belong in a shared utility, not in vendor-specific write paths.

### `controller/v2/gateway.go` and `controller/v2/infra_routes.go`

These files import provider-specific CRDs (`nginx-gateway-fabric`,
`gke-gateway-api`). Provider-specific behavior should be feature-gated
(`utils.IsRegistered` is already used at controller setup for `gatewayv1`),
or extracted into a `controller/v2/networking/{provider}/` sub-tree. As
written, every operator deployment carries the union of all supported
provider CRDs.

### `translator/v2/`

Same issue as before: vendor spec builders import all vendored operator
CRDs; converters import nothing beyond `api/v2` and `translator`. They
belong in different packages (`translator/v2/convert/` vs.
`translator/v2/vendor/`), or the vendor builders should move next to the
managed-infra code that owns them.

---

## 8. Summary of Cross-Cutting Concerns

### Concern: Where do defaults live?

| Severity | Issue |
|----------|-------|
| High | Four-layer default split (was three) — `ApplyInfraSizing` is now a major hidden source of spec mutation |
| Medium | `+kubebuilder:default` annotations and webhook defaulter set the *same* fields (e.g. `OnDelete=detach`, `ServiceAccount.Create=true`) — pick one |
| Medium | Translator defaults invisible to webhook validation (`gorilla`, replica counts of 3, MySQL `mycnf`) |
| Medium | Webhook-only defaults still have no fallback when webhook disabled (per-service `Name`/`Namespace`, sentinel-for-non-dev) |

### Concern: Where are invariants enforced?

| Severity | Issue |
|----------|-------|
| High | Immutability rules exist only in webhook; no reconciler fallback for any infra type |
| High | Non-Redis infra (MySQL, Kafka, ObjectStore, ClickHouse) has zero immutability enforcement |
| Medium | Mutual exclusivity (managed vs. external) is only in webhook |
| Low | `validateNetworkingSpec` is well-formed but webhook-only |

### Concern: Read/write boundary violations

| Severity | Issue |
|----------|-------|
| High | `ReadState` writes connection Secrets in 4 of 5 managed vendors (redis, mysql, kafka, clickhouse). Minio is correct — match it. |
| Medium | MySQL credential generation in dispatcher (`controller/v2/mysql.go`), not in managed-infra layer |
| Low | `ApplyInfraSizing` mutates user-facing spec at reconcile time, not at admission |

### Concern: Layer violations

| Severity | Issue |
|----------|-------|
| Medium | `api/v2/application_types.go` imports KEDA, Argo Rollouts, and Gateway API |
| Medium | `controller/v2/gateway.go` imports nginx-gateway-fabric (provider-specific CRD at orchestration layer) |
| Medium | `controller/v2/infra_routes.go` imports gke-gateway-api (provider-specific CRD) |
| Medium | Vendor spec builders in `translator/v2` import all vendor CRDs |
| Low | `InferExternalStatus` in wrong package (`infra/external` instead of `controller/v2`) |
| Low | Telemetry domain types in `controller/v2` (should be `internal/telemetry`) |
| Low | Telemetry application allowlist hardcoded in `reconcile_v2.go` (should be in `pkg/wandb/manifest/`) |

### Concern: File scope and size

| Severity | Issue |
|----------|-------|
| High | `reconcile_v2.go` at 2592 lines covers ~17 distinct concerns |
| Medium | `webhook/v2/weightsandbiases_webhook.go` at 522 lines mixes defaulter and validator |
| Medium | `infra_routes.go` at 465 lines mixes Gateway API and provider-specific (GKE) policies |
