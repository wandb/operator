# Legacy env-var → CR field mapping: from conversion special-case to a reconcile-time registry

**Status:** Proposal (not yet implemented)
**Related:** [legacy_overrides.md](legacy_overrides.md) (the `spec.wandb.legacyOverrides` mechanism this builds on)

## Problem

v1 users configured server behavior by setting raw environment variables
(`env` / `extraEnv` in helm values). v2 models many of those knobs as typed CR
fields. Today the v1 → v2 conversion webhook special-cases exactly **two** env
vars — `WF_CLICKHOUSE_REPLICATED` and `WF_CLICKHOUSE_REPLICATED_CLUSTER` — to
turn them into typed ClickHouse topology fields. Every new env var we discover
would need the same bespoke machinery bolted into the webhook.

That does not scale, and the webhook is the wrong place for it:

- **It is stateless.** The conversion webhook cannot create Secrets, so every
  literal it can't express as a selector is stashed in a
  `legacy.operator.wandb.com/*-pending` annotation and drained later by the
  reconciler. Env-var mapping inherits that whole two-hop dance.
- **It resolves the server manifest at admission time** (with a failure-cooldown
  cache) just to know which values sections are applications.
- **The mapping is bound to the v2 CRD shape**, which the reconciler owns. Field
  layout, instance keying, and managed-vs-external rules live on the reconcile
  side; expressing them in `api/v1` couples the spoke version to the hub's
  internals.

## Key insight

`spec.wandb.legacyOverrides` **already carries every other env var verbatim**
into the CR. `mapLegacyOverrides` walks the global and per-application sections
and copies their `env`/`extraEnv` into
`spec.wandb.legacyOverrides[<app|"global">].Env` as `[]corev1.EnvVar`. The only
reason the reconciler can't already see the two ClickHouse vars is that
`legacyEnvFromSection` explicitly skips them (`isClickHouseReplicationEnv`).

Remove that one exclusion and **all** env vars flow into `legacyOverrides`. The
reconciler already holds the CR, a `client.Client` (so it can materialize
Secrets), and the resolved manifest. So the env → CR mapping is a natural
reconcile-time step, driven by a **hardcoded registry** bound to the CRD.

## Current architecture

Two independent mechanisms exist today.

### Mechanism A — `legacyOverrides` passthrough (generic, no typing)

```
v1 values (env/extraEnv)
  └─ conversion: mapLegacyOverrides → legacyEnvFromSection → legacyEnvVar
        · env beats extraEnv; sorted by name; helm-template + non-scalar dropped
        · isClickHouseReplicationEnv(k) → SKIP   ← the special case
  → spec.wandb.legacyOverrides[<app|"global">].Env  ([]corev1.EnvVar)

reconcile: reconcileApplications → per-app env pipeline
  └─ applyLegacyOverrideEnv(...) applied LAST (beats manifest + injected env)
        · global layer, then per-app layer (per-app wins)
```

Env vars land as raw pod env. They are never promoted to typed fields.
`validateLegacyOverrides` only *logs* keys that map to no manifest application.

Files: [weightsandbiases_conversion_overrides.go](../../../api/v1/weightsandbiases_conversion_overrides.go),
[legacy_overrides.go](../../../internal/controller/reconciler/legacy_overrides.go).

### Mechanism B — ClickHouse replication special-case (typed, bespoke)

```
v1 values
  └─ conversion:
        legacyEnvFromSection EXCLUDES the two vars (isClickHouseReplicationEnv)
        mapClickHouseReplication → harvestClickHouseReplication
           · scans every section's env/extraEnv (env beats extraEnv)
           · per-app beats global; apps that disagree → hard error
           · fallback: structured global.clickhouse.replicated (env beats flag)
           · only attaches when an ExternalClickHouse connection exists
             (managed derives its own topology → drop)
        → writeAnnotation(ClickHousePendingAnnotation, {replicated, replicatedCluster})

reconcile:
  migrateLegacyClickHouse drains the pending annotation
     → <cr>-clickhouse-converted Secret (keys replicated / replicatedCluster)
     → sets ExternalClickHouse.Replicated / .ClusterName selectors (only-fill-if-zero)

pod build (read side):
  resolveEnvvars, manifest source {type: clickhouse, field: replicated|replicated-cluster}
     → SecretKeyRef into the connection Secret
     → SKIPS the env var when the selector is unpublished  (pods.go:283-294)
```

Files: [weightsandbiases_conversion_mapping.go](../../../api/v1/weightsandbiases_conversion_mapping.go)
(lines ~618-802), [migrate_legacy.go](../../../internal/controller/reconciler/migrate_legacy.go)
(lines ~197-260), [pods.go](../../../internal/controller/reconciler/pods.go) (lines ~259-301).

### The load-bearing fact

Every field of every `*Connection` struct (`ClickHouseConnection`,
`MysqlConnection`, `RedisConnection`, `ObjectStoreConnection`) and the four
OIDC credential fields are `corev1.SecretKeySelector`. There are **no plain
`string`/`*bool` connection fields.** `Replicated` and `ClusterName` are
`SecretKeySelector`, not `*bool`/`string`. So "map an env var into a connection
field" always means **materialize a Secret value + point a selector at it** —
never "assign a Go string." Only a handful of `spec.wandb.*` / `spec.global.*`
scalars (`OidcSpec.SessionLength`, `Wandb.License`, `Wandb.BucketProxy`, …) are
plain fields.

The round-trip only works because the manifest **re-injects** the value from the
mapped field on the read side. Removing the env var from `legacyOverrides` is
safe *only* when a manifest source reads it back out of the field we wrote (for
replication, the `clickhouse` source does, and skips it when unpublished). This
is a hard constraint on the registry (see [Round-trip safety](#round-trip-safety)).

## Proposed design

### Division of responsibility

| Input | Owner | Why |
|-------|-------|-----|
| **Raw env vars** (`env`/`extraEnv`) | **Reconciler**, via the registry | Env values already ride into `legacyOverrides`; the reconciler can materialize Secrets and knows the CRD shape. |
| **Structured helm values** (`global.clickhouse.host`, …) | Conversion webhook (unchanged) | These are already typed and unambiguous; `mapClickHouse` / `clickHouseFields` etc. stay as-is. |

The conversion webhook stops intercepting env vars. A new reconcile step maps
registered env vars from `legacyOverrides` into typed fields and removes them.

### The registry

A hardcoded, declarative table — one entry per mappable env var — living in a new
`internal/controller/reconciler/legacy_env_mapping.go`. It generalizes the
existing `clickHouseFields` `setRef`-closure pattern
([weightsandbiases_conversion_mapping.go:532](../../../api/v1/weightsandbiases_conversion_mapping.go)).

```go
// legacyEnvMappings is the hardcoded registry of v1 env vars the reconciler
// promotes into typed v2 fields, then removes from legacyOverrides. Bound to
// the CRD; grows as we model more knobs. Only add an env var here when a
// manifest source re-injects its value from the target field (see round-trip
// safety) — otherwise removal silently drops it from the pod.
var legacyEnvMappings = []legacyEnvMapping{
    {
        env:      "WF_CLICKHOUSE_REPLICATED",
        scope:    scopeDatastore, // one value for the datastore; sources must agree
        onExist:  overrideConversionDerived,
        apply:    externalClickHouseSelector("replicated"),
    },
    {
        env:      "WF_CLICKHOUSE_REPLICATED_CLUSTER",
        scope:    scopeDatastore,
        onExist:  overrideConversionDerived,
        apply:    externalClickHouseSelector("clusterName"),
    },
}

type legacyEnvMapping struct {
    env     string
    scope   mappingScope   // scopeDatastore | scopeGlobal | scopePerApp
    onExist conflictPolicy // keepCR (default) | overrideConversionDerived
    apply   applyFn
}

// applyFn owns instance resolution, the managed-vs-external guard, Secret
// materialization / selector repointing, and setting the field. It receives the
// full scope-resolved EnvVar (literal Value OR ValueFrom). It returns remove=true
// when the env should be stripped from legacyOverrides (mapped, or intentionally
// dropped); remove=false leaves it as a raw-env passthrough (a valueFrom shape the
// Secret-only target can't represent).
type applyFn func(ctx context.Context, c client.Client, w *v2.WeightsAndBiases, env corev1.EnvVar) (remove bool, err error)
```

`scope`:
- **`scopeDatastore`** — a property of the datastore, not a workload. Collect the
  env from every `legacyOverrides` section; per-app beats global; sections that
  disagree are a hard error (preserves today's conflict semantics). One resolved
  value.
- **`scopeGlobal`** / **`scopePerApp`** — future shapes for workload-scoped env
  that maps to per-instance or global fields.

`onExist`:
- **`keepCR`** (default, and the rule for all *future* generic entries) —
  CR value is source of truth; if the target field is already set, don't
  overwrite, just drop the env. Mirrors migrate_legacy's `fill` (only-fill-zero).
- **`overrideConversionDerived`** — for datastore-topology entries whose only
  pre-set source is the conversion webhook's structured-flag mapping. The v1 env
  var is the most explicit v1 signal, so it overrides. Safe because a v2-native
  user does not carry a v1 env in `legacyOverrides` (the map is conversion-owned).

### The reconcile step

Add `mapLegacyEnvToCR(ctx, client, wandb)` and wire it in **immediately after
`migrateLegacyAnnotations`** and before the manifest load, reusing the same
short-circuit idiom ([reconcile_v2.go:167-169](../../../internal/controller/reconciler/reconcile_v2.go)):

```go
// Migrate legacy v1 conversion annotations into typed spec fields
if res, err := migrateLegacyAnnotations(ctx, client, wandb); err != nil || res.RequeueAfter > 0 {
    return res, err
}

// NEW: promote known legacy env vars into typed fields, drop them from overrides
if res, err := mapLegacyEnvToCR(ctx, client, wandb); err != nil || res.RequeueAfter > 0 {
    return res, err
}

// Fetch manifest early so infra sizing can be applied before provisioning
manifest, err := serverManifest.GetServerManifest(...)
```

**Placement rationale.** Because `migrateLegacyAnnotations` short-circuits and
requeues on any change, `mapLegacyEnvToCR` only runs on passes where migration is
already a no-op — i.e. after every `*-pending` annotation is drained and the
external connection (which `scopeDatastore` topology attaches to) already exists.
The step does not need the manifest (it matches on env name against
`legacyOverrides` keys — global plus application names), so keeping it pre-manifest
groups all spec-normalizing mutations together, before any workload env is built.
If a future entry needs the manifest, move the call after `ApplyInfraSizing`
([reconcile_v2.go:184](../../../internal/controller/reconciler/reconcile_v2.go)).

**Algorithm (per pass):**

1. If `len(wandb.Spec.Wandb.LegacyOverrides) == 0`, return no-op.
2. For each registry entry, resolve the env value across sections per `scope`
   (`scopeDatastore`: per-app beats global, disagreement → error).
3. If found, call `entry.apply(...)` with the full `EnvVar`, honoring `onExist`
   and the source shape (see [Source shapes](#source-shapes-literal-vs-valuefrom)):
   - literal `Value` → materialize into the converted Secret (merge — see below).
   - `ValueFrom.SecretKeyRef` → point the selector directly at the user's Secret.
   - other `ValueFrom` → passthrough (`remove=false`, leave the env in place).
   - `keepCR` skips the write when the field is already set; managed ClickHouse
     drops (no write, but still removed).
4. **Remove the env var from every `legacyOverrides` section** when `remove=true`
   (mapped, guard-dropped, or user-field-wins) — the field/manifest is now
   authoritative. Passthrough (`remove=false`) leaves it. Prune emptied sections;
   prune the map when it empties.
5. If anything changed, `client.Update(ctx, wandb)` + return
   `ctrl.Result{RequeueAfter: time.Second}`; else no-op. (Exactly
   `migrateLegacyAnnotations`' persistence shape — there is no shared helper, so
   this is hand-written the same way.)

### Source shapes: literal vs ValueFrom

`legacyOverrides` carries each env var as a full `corev1.EnvVar`, so a mapped var
may be a literal `Value` **or** a `ValueFrom` (`legacyEnvVar` preserves the whole
body). The mapper must handle both — the original harvest read only scalars and
silently dropped `valueFrom`-sourced replication env, a latent bug this fixes.

Both shapes reach the pod through the same consolidation: external ClickHouse
`WriteState` runs every spec selector through `external.ResolveFields` →
`ResolveSecretKey`, which dereferences whatever Secret each selector points at
(operator-owned **or** the user's) and copies the values into the unified
`wandb-clickhouse-connection` Secret
([clickhouse.go:37-60](../../../internal/controller/infra/external/clickhouse/clickhouse.go)).
So:

- **literal `Value`** → materialize into `<cr>-clickhouse-converted`, point the
  selector there (needs the merge below).
- **`ValueFrom.SecretKeyRef`** → set the selector **directly** to the user's
  `{Name, Key}` — no Secret write at all; `ResolveFields` reads it.
- **`ValueFrom.ConfigMapKeyRef` / other** → not representable in a Secret-only
  selector → passthrough (`remove=false`): leave the env in `legacyOverrides`,
  where it still reaches the pod as raw env (the read side skips the manifest
  binding when the field is unpublished, so the raw override supplies it). No loss.

### Secret materialization (merge, don't replace)

For the **literal** case the value must live in a Secret the selector can point
at. **Reuse the `<cr>-clickhouse-converted` Secret, but merge rather than
replace.** `migrateLegacyClickHouse` builds the full data map and
`CreateOrUpdate`s, *replacing* `secret.Data`
([migrate_legacy.go:453](../../../internal/controller/reconciler/migrate_legacy.go);
test `TestMigrateLegacyClickHouse_PreExistingSecretOverwritten`). A naive
`CreateOrUpdate` from `mapLegacyEnvToCR` on the same Secret would clobber the
host/port/user literals migration wrote. Add a small merge helper
(read existing → set the one key → update). The `ValueFrom.SecretKeyRef` case
needs no Secret write.

### Managed-vs-external guard

Replication attaches only when `ClickHouse[default].ExternalClickHouse != nil`
(managed ClickHouse derives its own topology). For managed, `apply` writes nothing
and returns `remove=true`, so the env is still removed — preserving
`TestConvertTo_ClickHouseReplicationDroppedWhenManaged`.

### Round-trip safety

Removing an env var from `legacyOverrides` only preserves pod behavior if a
manifest source re-injects the value from the field we wrote. For replication the
manifest `{type: clickhouse, field: replicated|replicated-cluster}` source does
exactly that and *skips* the env when the selector is unpublished
([pods.go:283-294](../../../internal/controller/reconciler/pods.go)).

**Registry contract:** only register an env var whose value the manifest
re-injects from the target field. This is an author rule, enforced by per-version
fixture tests, not by code (the binding is manifest-version-specific). A generic
`custom-resource` dotted-path source already exists
([pods.go:417-434](../../../internal/controller/reconciler/pods.go)) as the mirror
for scalar `spec.*` targets that have no dedicated source type.

### Precedence summary (after this change)

For a mapped (registered) env var, effective precedence at the pod becomes:

1. Explicit v2 CR field set by a user → wins (`keepCR` entries).
2. v1 env var via `legacyOverrides` → the reconcile mapping
   (`overrideConversionDerived` beats the conversion-derived structured flag).
3. v1 structured flag via the conversion webhook → the field, when no env maps.
4. Manifest default.

Unmapped env vars are unchanged: they stay in `legacyOverrides` and
`applyLegacyOverrideEnv` still applies them **last**, beating manifest env.

## What changes

### Remove from the conversion webhook (`api/v1`)

- **`isClickHouseReplicationEnv` exclusion** in `legacyEnvFromSection`
  ([overrides.go:252-257](../../../api/v1/weightsandbiases_conversion_overrides.go))
  — *the pivotal change*: lets both env vars flow into `legacyOverrides`.
- `mapClickHouseReplication`, `harvestClickHouseReplication`,
  `readClickHouseReplicationEnv`, `resolveClickHouseEnvFinding`, the
  `clickHouseEnvFinding` struct, `readClickHousePendingAnnotation` (now dead), the
  `clickHousePendingReplicatedKey`/`clickHousePendingClusterKey` constants, and the
  `mapClickHouseReplication` call in `applyValueMappings`.
- `envClickHouseReplicated`/`envClickHouseReplicatedCluster` and
  `isClickHouseReplicationEnv` move to (or are re-declared in) the reconciler
  package as the registry's env names.
- **Keep** `mapClickHouse` and `clickHouseFields` (structured connection literals).
  Under the recommended option (A below), add a small **structured-only**
  read of `global.clickhouse.replicated` (+ cluster) into the pending
  annotation's `replicated`/`replicatedCluster` keys, replacing the env-aware
  harvest with a plain typed mapping.

### Add to the reconciler (`internal/controller/reconciler`)

- New `legacy_env_mapping.go`: the registry, `mapLegacyEnvToCR`, target
  constructors (`externalClickHouseSelector`), scope resolution (the datastore
  conflict/precedence check), the Secret-merge helper, the `onExist` guard, and
  env removal + map pruning.
- Wire `mapLegacyEnvToCR` into `Reconcile` after `migrateLegacyAnnotations`.
- `migrateLegacyClickHouse` keeps draining `replicated`/`replicatedCluster` from
  the pending annotation (now sourced only from the **structured flag**); the env
  mapper overrides on a later pass when the env is present.

## Invariants to preserve

These are pinned by existing tests and must survive the move (source: the
behavior audit of `legacy_overrides_test.go`,
`weightsandbiases_conversion_clickhouse_replication_test.go`,
`weightsandbiases_conversion_overrides_test.go`, `migrate_legacy_test.go`,
`clickhouse_replication_test.go`).

Replication mapping (re-expressed against `legacyOverrides` in the reconciler):
- [ ] Env resolved from any section's `env`/`extraEnv`; **env beats extraEnv**.
- [ ] **Per-app beats global**; apps that **disagree → hard error** ("every
      application must agree"); identical values are not a conflict.
- [ ] Non-boolean `WF_CLICKHOUSE_REPLICATED` → hard error ("is not a boolean").
- [ ] Helm-template (`{{ }}`) values ignored (already dropped by
      `legacyEnvVar` before they reach `legacyOverrides`).
- [ ] Managed ClickHouse → replication dropped (no external connection to attach).
- [ ] Values land in `<cr>-clickhouse-converted` under keys
      `replicated`/`replicatedCluster`; selectors point there; **only-fill-zero /
      CR-value-wins** except the `overrideConversionDerived` topology entries.
- [ ] The env vars **do not remain** in `legacyOverrides` after mapping
      (today's `require.NotContains`); the raw v1-values annotation still
      round-trips them untouched.
- [ ] Read side: apps read topology from the connection Secret; the env var is
      **dropped when the selector is unpublished** (unchanged — no code change).

Generic `legacyOverrides` behavior (unchanged, must not regress):
- [ ] `overrideEnvVars` replace-in-place-then-append; empty names skipped; last
      duplicate wins; nil overrides are a no-op.
- [ ] `applyLegacyOverrideEnv` precedence manifest < global < per-app; an app
      with no entry still gets the global layer.
- [ ] `validateLegacyOverrides` remains non-mutating (log-only).

### Test migration

- `api/v1/weightsandbiases_conversion_clickhouse_replication_test.go` — **moves**
  into `internal/controller/reconciler/` and is rewritten to seed
  `spec.wandb.legacyOverrides[...].Env` and assert Secret/selector results plus
  env removal (instead of asserting the pending-annotation payload).
- `api/v1/weightsandbiases_conversion_overrides_test.go` — **stays** as a
  conversion test but loses the "replication vars stripped from overrides" case
  (that behavior moves to the reconciler); the two env vars now pass through
  verbatim, so add/adjust a passthrough assertion.
- `internal/controller/reconciler/migrate_legacy_test.go` — the ClickHouse
  replication-drain cases narrow to the structured-flag path; env-driven
  replication assertions move to the new mapper's test.
- `internal/controller/reconciler/legacy_overrides_test.go` and
  `clickhouse_replication_test.go` — **unchanged.**
- New `internal/controller/reconciler/legacy_env_mapping_test.go`.

## Open decision: the structured `global.clickhouse.replicated` fallback

Today the harvest also honors the **structured** v1 flag
`global.clickhouse.replicated` as a fallback when no env var set replication, with
**env-beats-flag** precedence (`TestConvertTo_ClickHouseReplicationEnvWinsOverFlag`,
`...FromGlobalClickhouseFlag`). That flag is not an env var and is not carried in
`legacyOverrides`, so a purely env-driven reconcile mapper won't see it.

- **(A) Recommended — flag mapped in conversion; env overrides in reconcile.**
  `mapClickHouse` maps the *structured* flag into the field (its normal
  structured job, no env harvesting). The reconcile env mapper uses
  `overrideConversionDerived` for the topology entries, so a present env var
  overrides the flag-derived field (preserves env-beats-flag) and the flag value
  survives when no env maps. Preserves **every** current invariant; keeps the
  reconciler purely `legacyOverrides`-driven (no v1-values parsing). Cost: a v2
  user who both set the typed field *and* left a stale v1 env would be overridden
  — but that combination cannot arise from conversion (the map is
  conversion-owned) and is documented.
- **(B) Simpler — drop the structured-flag fallback (env-only).** Uniform
  `keepCR` everywhere; removes *all* replication logic from conversion. Cost: a
  v1 deployment that set `global.clickhouse.replicated: true` structurally but
  never set the env var loses replication — a real behavior change with an
  existing test. Low risk in practice (the chart typically propagates the flag to
  the env var anyway), but it is a fidelity loss.

Recommend **(A)** for zero behavior change; choose **(B)** if the team accepts the
narrow fidelity loss for a smaller conversion webhook.

## Sequencing

1. API/registry scaffolding in the reconciler (`legacy_env_mapping.go`) + the
   Secret-merge helper; unit-test the mapper in isolation with hand-built
   `legacyOverrides`.
2. Wire `mapLegacyEnvToCR` into `Reconcile`; add the two replication entries.
3. Move the structured-flag mapping into `mapClickHouse` (option A) and delete the
   env harvest + exclusion from the conversion webhook.
4. Migrate/rewrite the affected tests; update
   [docs/infra-connection-settings.md](../../infra-connection-settings.md) to note
   replication env vars are now promoted at reconcile.
5. `make manifests generate sync-crd-embed`, `make lint`, `make test`.

## Non-goals

- Changing the manifest read side (`resolveEnvvars`) — it already re-injects
  mapped values and is the mechanism that makes removal safe.
- Modeling new env vars beyond the two replication seeds. The env-candidate audit
  (ClickHouse connection vars, `GORILLA_OIDC_*`, `GORILLA_SESSION_LENGTH`,
  bucket/`WF_FILE_STORAGE_*`, Kafka) confirms the registry will grow, but each new
  entry is its own change gated on a manifest re-injection binding.
