# Plan: Enforce minimum infrastructure versions on W&B upgrades

## Context

When a user bumps `spec.wandb.version` on a `WeightsAndBiases` CR, the new W&B server image requires its dependent infrastructure (MySQL, Redis, ClickHouse, Kafka, MinIO/object store) to be at or above a known minimum version, and managed deployments need a concrete pin so that the operator's own infrastructure rolls forward in lockstep with W&B. Today the operator has no notion of either:

- **Managed infra** (operator-deployed) is created from vendor-spec builders with no image pin — versions float to whatever the underlying vendor operator defaults to.
- **External infra** (user-supplied) is stored as connection secrets only; the operator never knows what version the user is actually pointing at.

This means an upgrade can succeed at apply time and then fail at runtime with cryptic SQL/Redis/Kafka errors.

The goal is to make the W&B server manifest the source of truth for two per-component versions so that:

1. For **managed** components, the reconciler deploys the manifest's **target version** (the version the W&B release is built against). This keeps managed deployments predictable and lets W&B advance the floor by simply publishing a new manifest.
2. For **external** components, the reconciler probes the live component each loop and writes the detected version into `status.{component}Status.version`. The validating webhook then compares `status` against the new manifest's **minimum version** on `spec.wandb.version` updates and **hard-rejects** the update if the user-supplied infra is below it. There is no upper bound on user-supplied infra: a user running a newer-than-target version is intentionally supported. (The webhook can't probe — admission must be fast and synchronous — so it relies on the version the reconciler last cached into status.)

There is one edge case the webhook can't catch: the very first apply, where status is empty. That case is left to the reconciler, which surfaces a not-ready condition with a clear message.

## Design summary

| Concern | Where versions live |
|---|---|
| Minimum supported version per component, per W&B release | New `minVersion` field in each per-component manifest section |
| Managed deployed version (image tag) | New `targetVersion` field in each per-component manifest section, consumed by spec builders |
| External detected version | New `version` field on each `*InfraStatus` in the CRD status |
| Enforcement on update | Validating webhook compares status `version >= minVersion` |

Key decisions:

- Requirements live **on the existing per-component sections** in the manifest (`Bucket`, `Clickhouse`, `Mysql`, `Redis`, `Kafka`), not in a new top-level section.
- Two fields per component: **`minVersion`** (the floor enforced for external infra) and **`targetVersion`** (what managed infra is deployed at).
- External version detection uses **active protocol probes** during reconcile.
- Managed components deploy the **manifest's `targetVersion`** unconditionally; user overrides on the CR are not honored.
- Webhook **hard-rejects** updates whose external infra version is below `minVersion`. No upper bound — newer is always allowed for external infra.

`github.com/Masterminds/semver/v3` is already a direct dependency — used for parsing and comparison.

---

## 1. Manifest changes — `pkg/wandb/manifest/manifest.go`

Add two optional version fields, grouped in a small embedded struct, to each existing component config:

- **`MinVersion`** is the floor enforced for external infra (webhook check). Empty means "no minimum".
- **`TargetVersion`** is the version managed infra is deployed at (spec builders consume it). Empty preserves today's behavior of letting the underlying vendor operator pick a default.

```go
// ComponentVersions declares the version policy for an infrastructure
// component for a given W&B release. MinVersion is the floor required of
// external/user-supplied components; the validating webhook rejects
// upgrades when the live external version is below it. TargetVersion is
// the version managed deployments are pinned to by the spec builders.
// Both are exact semver strings (e.g. "8.0.36"); pre-release suffixes
// are allowed. Either may be empty to disable that role.
type ComponentVersions struct {
    MinVersion    string `yaml:"minVersion,omitempty"`
    TargetVersion string `yaml:"targetVersion,omitempty"`
}

type InfraConfig struct {
    Sizing            map[v2.Size]SizingConfig `yaml:"sizing"`
    Ingress           *AppIngressSpec          `yaml:"ingress,omitempty"`
    ComponentVersions `yaml:",inline"`
}

type KafkaConfig struct {
    Sizing            map[v2.Size]KafkaSizingConfig `yaml:"sizing"`
    Topics            []KafkaTopic                  `yaml:"topics"`
    ComponentVersions `yaml:",inline"`
}
```

Update `mergeSimple` / `mergeInfraConfigs` to carry `MinVersion` and `TargetVersion` through (preserve dst when set, else copy from src — same precedence as today).

A typical manifest entry for `0.79.0.yaml` will gain:

```yaml
mysql:
  default:
    minVersion: "8.0.36"
    targetVersion: "8.0.36"
    sizing: {...}
clickhouse:
  default:
    minVersion: "24.3"
    targetVersion: "24.8"
    sizing: {...}
kafka:
  minVersion: "3.6"
  targetVersion: "3.7.1"
  sizing: {...}
```

No new top-level field, no new collections. Backward compatible — empty fields mean "no policy" and older manifests deserialize cleanly. Sanity check: `TargetVersion` should be `>= MinVersion` if both are set; the manifest loader logs a warning if not (the manifest is W&B-authored, so this is a release-engineering smell, not a user-actionable error).

---

## 2. CRD status changes — `api/v2/weightsandbiases_types.go`

Add a `Version` field to the shared base `WBInfraStatus` so it appears on every component status:

```go
type WBInfraStatus struct {
    Ready      bool               `json:"ready"`
    State      string             `json:"state,omitempty" default:"Unknown"`
    Version    string             `json:"version,omitempty"`
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

Single, semver-formatted string (e.g. `"8.0.36"`, `"3.6.1"`, `"24.3.1.2305"`). Empty means "not yet detected" or "probe failed" (the reconciler will set a condition explaining why).

This field is populated:
- For **managed** components: from the deployed image tag (the spec builder already knows it).
- For **external** components: from the live protocol probe.

The `+kubebuilder:printcolumn` markers stay as they are; we don't need a column for version.

Run `make manifests generate` to regenerate the CRD YAML and `zz_generated.deepcopy.go`.

Also: **remove (or tombstone) the existing `Version` field on `ManagedClickHouseSpec`** at [api/v2/weightsandbiases_types.go:459](api/v2/weightsandbiases_types.go:459). It is the only managed spec with such a field today, and once the manifest is the source of truth, leaving it would create two competing pins. Removal is a breaking change to the v2 CRD; if any production CRs set it, the field should be tombstoned (parsed-but-ignored with a deprecation warning) for one release before deletion.

---

## 3. Reconciler changes

### 3a. Managed: pin the deployed version to the manifest's target

The reconciler already fetches the manifest at [internal/controller/v2/reconcile_v2.go:272](internal/controller/v2/reconcile_v2.go:272) and passes it into the per-component flows. The vendor-specific spec builders live under `internal/controller/infra/managed/{component}/{vendor}/spec.go`:

- [internal/controller/infra/managed/mysql/mysql/spec.go](internal/controller/infra/managed/mysql/mysql/spec.go) — `ToMysqlMySQLVendorSpec`
- [internal/controller/infra/managed/redis/opstree/spec.go](internal/controller/infra/managed/redis/opstree/spec.go) — `ToRedis…VendorSpec`
- [internal/controller/infra/managed/kafka/strimzi/spec.go](internal/controller/infra/managed/kafka/strimzi/spec.go) — `ToKafkaVendorSpec` / `ToKafkaNodePoolVendorSpec`
- [internal/controller/infra/managed/clickhouse/altinity/spec.go](internal/controller/infra/managed/clickhouse/altinity/spec.go) — `ToClickHouseVendorSpec`
- [internal/controller/infra/managed/minio/tenant/spec.go](internal/controller/infra/managed/minio/tenant/spec.go) — MinIO Tenant builder

For each managed component:

1. Plumb the manifest's per-component `TargetVersion` from the v2 reconciler dispatch layer ([internal/controller/v2/{mysql,redis,kafka,clickhouse,objectstore}.go](internal/controller/v2)) into the corresponding `To*VendorSpec` builder. The reconciler already has the parsed `Manifest` in scope at the dispatch site; thread the relevant `ComponentVersions` through the existing `managed*WriteState` functions (`managedMysqlWriteState` at [internal/controller/v2/mysql.go:100](internal/controller/v2/mysql.go:100), and the equivalents in the other component files).
2. The builder translates `TargetVersion` into the appropriate field on the underlying vendor CR — e.g. `InnoDBCluster.Spec.Version` (mysql-operator), `Kafka.Spec.Kafka.Version` (Strimzi), the Altinity ClickHouseInstallation image tag, the Opstree Redis image tag, the MinIO Tenant image tag.
3. **Managed-side version is operator-owned.** The manifest's `TargetVersion` is always used; user overrides on the CR are not honored for managed components.
4. Write the resulting deployed version into `wandb.Status.{Component}Status.Version` during the existing `*InferStatus()` call.

For components without an existing CRD `Version` field (everything except ClickHouse), no spec change is needed — the builder pins via the underlying vendor CR's image/version field. If the manifest leaves `TargetVersion` empty (older manifests), preserve today's behavior of letting the vendor operator pick a default.

### 3b. External: probe the live component for its version

Add a small per-component probe helper at `internal/controller/infra/external/{component}/probe.go`. Each helper:

1. Resolves the connection secret already written by `WriteState` (reuse `external.ReadConnectionSecret` and the existing per-component decoders that build the typed connection struct in `*ReadState`).
2. Opens a short-lived client, queries the version, closes.
3. Returns `(version string, err error)`.

| Component | Library | Query |
|---|---|---|
| MySQL | `database/sql` + `github.com/go-sql-driver/mysql` (add) | `SELECT VERSION()` |
| ClickHouse | `github.com/ClickHouse/clickhouse-go/v2` (add) | `SELECT version()` |
| Redis | `github.com/redis/go-redis/v9` (add) | `INFO server` -> parse `redis_version` |
| Kafka | `github.com/twmb/franz-go` (add) | `ApiVersions` request -> derived broker version, or `DescribeClusterRequest` |
| Object store | `github.com/minio/minio-go/v7` (already vendored) | `HEAD /` (S3 list-buckets); parse `Server` header for MinIO; for AWS/GCS, version isn't applicable — record `"s3"` |

Wire each probe into the existing `*ReadState` -> `*InferStatus` flow under the external branch. The probe runs only when the connection secret is present (otherwise we have nothing to dial). Probe failures degrade gracefully:

- Set `WBInfraStatus.Conditions` with type `VersionProbeReady` = False and the error.
- Leave `WBInfraStatus.Version` empty (or last-known — preserve through the existing condition merge if helpful).
- Don't fail the whole reconcile — the rest of the loop continues, and the webhook treats empty as "unknown" (which is itself a rejection signal on update — see §4).

Probe timeouts must be tight (e.g. 5s) so a flaky external infra doesn't stall the reconcile loop.

Files most affected:

- [internal/controller/v2/mysql.go](internal/controller/v2/mysql.go) — call probe in `externalMysqlInferStatus`
- [internal/controller/v2/redis.go](internal/controller/v2/redis.go) — same
- [internal/controller/v2/kafka.go](internal/controller/v2/kafka.go)
- [internal/controller/v2/clickhouse.go](internal/controller/v2/clickhouse.go)
- [internal/controller/v2/objectstore.go](internal/controller/v2/objectstore.go)
- New: `internal/controller/infra/external/{mysql,redis,kafka,clickhouse,objectstore}/probe.go`
- Optionally extend [internal/controller/infra/external/common.go](internal/controller/infra/external/common.go) with a shared `InferExternalStatusWithVersion` helper that wraps the existing `InferExternalStatus` and folds in the probed version.

---

## 4. Webhook changes — `internal/webhook/v2/weightsandbiases_webhook.go`

The validating webhook today is empty-struct only. To enforce minimums on update:

1. **Validator dependencies** — `WeightsAndBiasesCustomValidator` only needs additional fields if other validations require injected state (e.g. logger, optional `client.Reader`). The version-check uses `manifest.GetServerManifest`, which is package-level. Today's call sites at [cmd/main.go:359](cmd/main.go:359), [internal/webhook/v2/webhook_suite_test.go:115](internal/webhook/v2/webhook_suite_test.go:115), and [internal/controller/suite_test.go:159](internal/controller/suite_test.go:159) stay unchanged unless dependencies are added.

2. **New validation step** invoked from `ValidateUpdate` only — Create has no status to compare, and the reconciler will surface a not-ready condition on first deploy:
   - If `oldWandb.Spec.Wandb.Version == newWandb.Spec.Wandb.Version`, skip — only enforce on actual upgrades.
   - Fetch the new manifest: `manifest.GetServerManifest(ctx, newWandb.Spec.Wandb.ManifestRepository, newWandb.Spec.Wandb.Version)`. `GetServerManifest` already caches OCI pulls to a local volume at `/tmp/server-manifest` and resolves locally before going remote (see [pkg/wandb/manifest/manifest.go:540](pkg/wandb/manifest/manifest.go:540)), so the webhook does not need its own cache layer — once the controller has reconciled a version, the webhook's lookup for that same version is a fast local read.
   - For each external component (i.e., where `Spec.{Component}.External{Component} != nil`), look up the manifest's per-component `MinVersion`:
     - `Bucket["default"].MinVersion` (object store)
     - `Mysql["default"].MinVersion`
     - `Redis["default"].MinVersion`
     - `Clickhouse["default"].MinVersion`
     - `Kafka.MinVersion`
   - Compare against `newWandb.Status.{Component}Status.Version`:
     - If `MinVersion` is empty -> no requirement, skip.
     - If status version is empty -> reject: *"the version of external X has not been detected yet; wait for the operator to populate status.{x}Status.version before upgrading"*.
     - If status version is below `MinVersion` -> reject: *"external X is at version 8.0.20; W&B 0.79.0 requires at least 8.0.36"*.
     - Otherwise -> accept (no upper bound on external infra).
   - Use `github.com/Masterminds/semver/v3` for parsing; comparison is a direct `<` against parsed `*semver.Version`. A small helper in `pkg/wandb/manifest/version.go` (`(ComponentVersions).MeetsMinimum(v string) (ok bool, reason string)`) keeps the webhook code clean.
   - Return errors as `field.Invalid(field.NewPath("spec","wandb","version"), newVersion, msg)` collected into `field.ErrorList`, then wrapped with `apierrors.NewInvalid` — matching the existing pattern in this file.

3. **Managed components are not validated by the webhook.** The reconciler is the sole owner of managed-component versions: it pins to the manifest's `TargetVersion`, no user override, no second source of truth.

4. **No new defaulter logic is required.** Defaults are unchanged.

5. The fetched manifest, if it can't be loaded (network error, malformed), should fail-closed: reject the update with *"could not load manifest for version X: …"*. This is a hard guard against silently allowing upgrades the operator can't verify.

---

## 5. Files to change (summary)

**Modified**

- [pkg/wandb/manifest/manifest.go](pkg/wandb/manifest/manifest.go) — add `ComponentVersions` (with `MinVersion`, `TargetVersion`) inlined into `InfraConfig` and `KafkaConfig`; merge support.
- [internal/controller/v2/{mysql,redis,kafka,clickhouse,objectstore}.go](internal/controller/v2) — call probes (external), thread `ComponentVersions` into `managed*WriteState`, write versions into status (both branches).
- `internal/controller/infra/managed/{mysql/mysql,redis/opstree,kafka/strimzi,clickhouse/altinity,minio/tenant}/spec.go` — extend the `To*VendorSpec` builders to accept and apply the manifest `TargetVersion` to the underlying vendor CR's image/version field.
- [internal/controller/infra/external/common.go](internal/controller/infra/external/common.go) — optional shared `InferExternalStatusWithVersion` helper.
- [internal/webhook/v2/weightsandbiases_webhook.go](internal/webhook/v2/weightsandbiases_webhook.go) — add `validateInfraVersionRequirements` step in `ValidateUpdate`.
- [api/v2/weightsandbiases_types.go](api/v2/weightsandbiases_types.go) — add `Version` to `WBInfraStatus`; remove (or tombstone) `Version` from `ManagedClickHouseSpec`.
- `go.mod` — add `go-sql-driver/mysql`, `clickhouse-go/v2`, `redis/go-redis/v9`, `twmb/franz-go`. Verify `minio-go/v7` is sufficient for object store.
- Regenerated: `config/crd/bases/apps.wandb.com_weightsandbiases.yaml`, `api/v2/zz_generated.deepcopy.go`.

**New**

- `internal/controller/infra/external/{mysql,redis,kafka,clickhouse,objectstore}/probe.go` — one per component.
- `pkg/wandb/manifest/version.go` — defines `ComponentVersions.MeetsMinimum(v string) (ok bool, reason string)` and a `Versions(component, instance string) ComponentVersions` lookup over a `Manifest`.
- `internal/webhook/v2/weightsandbiases_webhook_version_test.go` — table-driven tests covering: at/above min (accept), below min (reject), missing status version (reject), empty min (accept), no-op when wandb version unchanged, manifest fetch error (reject).

---

## Verification

Run from the worktree root.

1. **Code generation & lint**
   ```sh
   make generate && make manifests
   make lint
   ```

2. **Unit & integration tests** — the existing webhook suite uses `envtest`; the new version-check tests will plug in there.
   ```sh
   make test
   ```
   Confirm the new webhook tests cover both accept and reject paths, and that probe code has unit tests with mocked clients.

3. **Local end-to-end with Tilt** (per `Tiltfile` / `DEVELOPMENT.md`):
   - Apply a `WeightsAndBiases` with `spec.wandb.version = 0.78.0` and an *external* MySQL pointing at an 8.0.20 instance.
   - Wait for `status.mysqlStatus.version = "8.0.20"` to appear.
   - `kubectl patch` the CR to bump `spec.wandb.version = 0.79.0` (assuming the manifest declares `mysql.default.minVersion: "8.0.36"` for that release).
   - Expect the API server to reject the patch with the *below-min* error.
   - Upgrade the external MySQL to 8.0.36, wait for status to refresh, retry the patch — expect success.
   - Upgrade the external MySQL to 9.0.0, retry the patch — expect success (no upper bound on external infra).

4. **Managed-side check**:
   - With managed MySQL on 0.78.0, bump `spec.wandb.version = 0.79.0`.
   - The reconciler should re-translate `InnoDBCluster.Spec.Version` to the manifest's `targetVersion` and `status.mysqlStatus.version` should reflect the upgraded version once mysql-operator finishes the rollout.
   - Confirm that user-set values on managed specs are not honored: e.g. setting `spec.clickhouse.managedClickhouse.version` (during the deprecation window if tombstoned, or rejected outright if removed) does not change the deployed image — the deploy stays pinned to manifest `targetVersion`.

5. **Failure-mode checks**:
   - Point an external probe at unreachable host -> confirm `VersionProbeReady` condition is False and reconcile doesn't loop-crash.
   - Bump version while status version is still empty -> confirm webhook rejects with the "not yet detected" message.
   - Manifest fetch fails on update -> confirm webhook fails closed.
