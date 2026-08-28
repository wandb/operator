# Implementation plan: reconcile-time legacy env-var → CR mapping (Option A)

Companion to [legacy_env_var_mapping.md](legacy_env_var_mapping.md). Implements
**Option A**: the structured `global.clickhouse.replicated` flag is mapped to the
typed field in the conversion webhook (a plain structured mapping); the raw env
vars flow through `legacyOverrides` and are promoted — and allowed to override the
conversion-derived value — by a new reconcile-time registry.

Branch: `danielpanzella/envvar-to-cr-value`.

## End-state behavior (what "done" looks like)

- The conversion webhook no longer knows about `WF_CLICKHOUSE_*` env vars. All env
  vars — including the two replication vars — land in
  `spec.wandb.legacyOverrides[...].Env` verbatim.
- `mapClickHouse` maps the **structured** `global.clickhouse.replicated` flag into
  the clickhouse-pending annotation's `replicated` key (only when a ClickHouse
  connection is present).
- A new reconciler step `mapLegacyEnvToCR` promotes registered env vars into typed
  fields (materializing/merging the `<cr>-clickhouse-converted` Secret), removes
  the mapped env vars from `legacyOverrides`, and persists + requeues.
- Precedence at the pod: explicit user field > v1 env var > v1 structured flag >
  manifest default. Unmapped env vars are untouched.

---

## Two source shapes: literal `Value` vs `ValueFrom`

`legacyOverrides` carries each env var as a full `corev1.EnvVar`, which may hold
either a literal `Value` **or** a `ValueFrom` (e.g. `secretKeyRef`) — `legacyEnvVar`
preserves the whole body during conversion. The mapper must handle both. (The
original harvest only read scalars via `scalarToString` and silently dropped any
`valueFrom`-sourced replication env — a latent bug this design fixes.)

Both shapes reach the pod through the **same** consolidation: external ClickHouse
`WriteState` runs every spec selector — `spec.Replicated`, `spec.ClusterName`, … —
through `external.ResolveFields` → `ResolveSecretKey`, which dereferences whatever
secret each selector points at and copies the values into the unified
`wandb-clickhouse-connection` secret ([clickhouse.go:37-60](../../../internal/controller/infra/external/clickhouse/clickhouse.go)).
So a selector may point at an operator-owned Secret **or a user's Secret** — the
value is resolved either way. That gives three source cases:

| Source env shape | How it maps to `conn.Replicated` (a `SecretKeySelector`) |
|---|---|
| literal `Value` (e.g. `"true"`) | materialize the value into `<cr>-clickhouse-converted`, point the selector there |
| `ValueFrom.SecretKeyRef` | point the selector **directly** at the user's `{Name, Key}` — no copy |
| `ValueFrom.ConfigMapKeyRef` / `FieldRef` / … | not representable in a Secret-only selector → **passthrough**: leave the env in `legacyOverrides` (still injected as raw pod env), don't touch the field |

## The override policy (the other subtle rule)

For the two representable shapes, `onExist` distinguishes conversion-derived from
user-set by **where the current selector points**:

| Current `conn.Replicated` selector | Action (representable source) |
|---|---|
| unset (`.Name == ""`) | write (materialize or repoint); set selector |
| points at `<cr>-clickhouse-converted` (conversion-derived: migrate_legacy's flag drain, or a prior env-map pass) | override — env beats the structured flag |
| points at a **user-supplied** Secret (and not the converted one) | leave field untouched (CR/user wins) |

`removed from legacyOverrides` is gated on representability, **not** on whether a
write happened:

- literal / `SecretKeyRef` source (mapped, or skipped because user-owned) → **removed**
  (the field is authoritative; the manifest re-injects from it).
- managed / unconfigured ClickHouse → **removed** and not written (managed derives
  its own topology — matches the original drop).
- unrepresentable `valueFrom` → **kept** (passthrough), so no data is lost.

Cluster (`WF_CLICKHOUSE_REPLICATED_CLUSTER`) has no structured source, so it is
`keepCR` (only-fill-if-unset), then removed under the same rules.

---

## Steps

Each step is independently compilable/testable. Steps 1–5 add the new path
(dormant except for the two seed entries); step 6 removes the old path; steps 7–9
clean up, migrate tests, and regenerate.

### Step 1 — Registry scaffolding (new file, no wiring)

**File:** `internal/controller/reconciler/legacy_env_mapping.go` (new)

Add the types and the seed registry. No behavior yet beyond pure helpers.

```go
package reconciler

const (
	envClickHouseReplicated        = "WF_CLICKHOUSE_REPLICATED"
	envClickHouseReplicatedCluster = "WF_CLICKHOUSE_REPLICATED_CLUSTER"

	// Data keys inside the operator-owned <cr>-clickhouse-converted Secret.
	convertedClickHouseReplicatedKey = "replicated"
	convertedClickHouseClusterKey    = "replicatedCluster"
)

type mappingScope int

const (
	// scopeDatastore: one value for the whole datastore. Collected across all
	// legacyOverrides sections; per-app beats global; disagreement is an error.
	scopeDatastore mappingScope = iota
)

type conflictPolicy int

const (
	keepCR conflictPolicy = iota // don't overwrite a field already set
	overrideConversionDerived     // overwrite when unset or operator-owned (see policy table)
)

// applyFn owns instance resolution, the managed-vs-external guard, Secret
// materialization / selector repointing, and setting the field. It receives the
// full scope-resolved EnvVar (Value or ValueFrom). It returns remove=true when
// the env var should be stripped from legacyOverrides (mapped, or intentionally
// dropped); remove=false leaves it in place as a raw-env passthrough (a valueFrom
// shape the target can't represent).
type applyFn func(ctx context.Context, c ctrlClient.Client, w *apiv2.WeightsAndBiases, env corev1.EnvVar) (remove bool, err error)

type legacyEnvMapping struct {
	env     string
	scope   mappingScope
	onExist conflictPolicy
	apply   applyFn
}

var legacyEnvMappings = []legacyEnvMapping{
	{env: envClickHouseReplicated, scope: scopeDatastore, onExist: overrideConversionDerived,
		apply: externalClickHouseSelector(convertedClickHouseReplicatedKey, overrideConversionDerived)},
	{env: envClickHouseReplicatedCluster, scope: scopeDatastore, onExist: keepCR,
		apply: externalClickHouseSelector(convertedClickHouseClusterKey, keepCR)},
}
```

**Verify:** `go build ./...` (unused symbols are fine at this stage if referenced
by the test added in later steps; otherwise add the resolver from Step 3 in the
same commit).

### Step 2 — Secret upsert (merge) helper

**File:** `internal/controller/reconciler/migrate_legacy.go` (add near
`materializeConvertedSecret`)

`materializeConvertedSecret` **replaces** `secret.Data`
([migrate_legacy.go:453](../../../internal/controller/reconciler/migrate_legacy.go)),
which would clobber the host/port keys migration wrote. Add a merge variant that
preserves existing keys:

```go
// upsertConvertedSecretKeys merges data into an existing (or new) opaque Secret
// without dropping keys other writers set. Used by the env mapper, which adds
// keys to the same <cr>-*-converted Secret migrateLegacy* already populated.
func upsertConvertedSecretKeys(ctx context.Context, c ctrlClient.Client, w *apiv2.WeightsAndBiases, name string, data map[string][]byte) error {
	if len(data) == 0 {
		return nil
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: w.Namespace}}
	if _, err := ctrl.CreateOrUpdate(ctx, c, secret, func() error {
		secret.Type = corev1.SecretTypeOpaque
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		for k, v := range data {
			secret.Data[k] = v // overwrite our keys, keep the rest
		}
		return nil
	}); err != nil {
		return fmt.Errorf("upsert %s: %w", name, err)
	}
	return nil
}
```

(`CreateOrUpdate` populates `secret` with live state before the mutate runs, so
`secret.Data` already holds migration's keys.)

**Verify:** a unit test that seeds a Secret with `{host}`, upserts `{replicated}`,
asserts both keys survive.

### Step 3 — The ClickHouse apply function + scope resolver

**File:** `internal/controller/reconciler/legacy_env_mapping.go`

```go
const clickHouseConvertedSecretSuffix = "-clickhouse-converted"

func externalClickHouseSelector(dataKey string, policy conflictPolicy) applyFn {
	return func(ctx context.Context, c ctrlClient.Client, w *apiv2.WeightsAndBiases, env corev1.EnvVar) (bool, error) {
		spec, ok := w.Spec.ClickHouse[apiv2.DefaultInstanceName]
		if !ok || spec.ExternalClickHouse == nil { // managed / unconfigured → drop (remove, no write)
			return true, nil
		}
		conn := spec.ExternalClickHouse
		target := &conn.Replicated
		if dataKey == convertedClickHouseClusterKey {
			target = &conn.ClusterName
		}
		secretName := w.Name + clickHouseConvertedSecretSuffix

		// Only literal Value and ValueFrom.SecretKeyRef are representable in a
		// Secret-only selector. Other valueFrom shapes → passthrough (keep env).
		isLiteral := env.ValueFrom == nil && env.Value != ""
		hasSecretRef := env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil
		if !isLiteral && !hasSecretRef {
			logx.GetSlog(ctx).Warn("legacy env has an unrepresentable valueFrom; leaving it as a raw override",
				"env", env.Name)
			return false, nil // remove=false: passthrough
		}

		// onExist: leave a user-supplied selector alone; overwrite when unset or
		// operator-owned. keepCR only fills when unset. Either way the env is removed.
		userOwned := target.Name != "" && target.Name != secretName
		if (policy == keepCR && target.Name != "") || (policy == overrideConversionDerived && userOwned) {
			return true, nil
		}

		if hasSecretRef {
			*target = *env.ValueFrom.SecretKeyRef // point directly at the user's Secret
			return true, nil
		}
		if err := upsertConvertedSecretKeys(ctx, c, w, secretName,
			map[string][]byte{dataKey: []byte(env.Value)}); err != nil {
			return false, err
		}
		*target = secretSelector(secretName, dataKey)
		return true, nil
	}
}
```

`secretSelector` already exists
([migrate_legacy.go:482](../../../internal/controller/reconciler/migrate_legacy.go)).
No map write-back is needed: `w.Spec.ClickHouse[default].ExternalClickHouse` is a
`*ClickHouseConnection`, and the `spec` copy shares that pointer, so `conn :=
spec.ExternalClickHouse` and `*target = …` mutate the connection the map entry
already points at. (Only a *newly created* connection would need the
`setExternalInstance` write-back idiom from `migrateLegacyClickHouse`; the guard
above returns early when `ExternalClickHouse == nil`, so we never create one here.)

Scope resolver (datastore precedence + conflict), re-expressing
`resolveClickHouseEnvFinding` against `legacyOverrides`:

```go
// resolveDatastoreEnv finds one env var across legacyOverrides sections and
// returns the winning EnvVar (Value or ValueFrom): per-app beats "global"; apps
// that disagree are an error. Agreement compares the whole body, so a literal and
// a secretKeyRef for the same var are a genuine conflict.
func resolveDatastoreEnv(overrides map[string]apiv2.LegacyOverrides, name string) (env corev1.EnvVar, found bool, err error) {
	var globalEnv corev1.EnvVar
	var globalSet bool
	var appEnv corev1.EnvVar
	var appSet bool
	var appSrc string
	for _, key := range sortedKeys(overrides) {
		for _, e := range overrides[key].Env {
			if e.Name != name || (e.Value == "" && e.ValueFrom == nil) {
				continue
			}
			if key == apiv2.LegacyOverridesGlobalKey {
				if !globalSet {
					globalEnv, globalSet = e, true
				}
				continue
			}
			if appSet && !sameEnvBody(e, appEnv) {
				return corev1.EnvVar{}, false, fmt.Errorf(
					"legacyOverrides: %s at %s conflicts with %s; ClickHouse replication "+
						"is a property of the datastore, so every application must agree",
					name, appSrc, key)
			}
			appEnv, appSet, appSrc = e, true, key
		}
	}
	switch {
	case appSet:
		return appEnv, true, nil // per-app beats global
	case globalSet:
		return globalEnv, true, nil
	default:
		return corev1.EnvVar{}, false, nil
	}
}

// sameEnvBody compares the payload (not the name) of two EnvVars.
func sameEnvBody(a, b corev1.EnvVar) bool {
	return a.Value == b.Value && apiequality.Semantic.DeepEqual(a.ValueFrom, b.ValueFrom)
}
```

Both literal `Value` and `ValueFrom` entries are considered — a v1 user who
sourced the var from a Secret keeps it (the original harvest dropped these).
Helm-template `Value`s never reach `legacyOverrides` (`legacyEnvVar` drops them
during conversion), so no template handling is needed here.

**Verify:** table tests for `resolveDatastoreEnv` (global-only, per-app-wins,
agreeing apps, conflicting apps → error) and for `externalClickHouseSelector`
(unset → writes; operator-owned → overwrites value; user-owned → skipped; managed
→ not applied).

### Step 4 — Orchestration + env removal + persist/requeue

**File:** `internal/controller/reconciler/legacy_env_mapping.go`

```go
// mapLegacyEnvToCR promotes registered legacy env vars into typed spec fields,
// then removes them from legacyOverrides (the field is now authoritative).
// Mirrors migrateLegacyAnnotations: mutate spec → Update → requeue → short-circuit.
func mapLegacyEnvToCR(ctx context.Context, c ctrlClient.Client, w *apiv2.WeightsAndBiases) (ctrl.Result, error) {
	if len(w.Spec.Wandb.LegacyOverrides) == 0 {
		return ctrl.Result{}, nil
	}
	changed := false
	for _, m := range legacyEnvMappings {
		env, found, err := resolveDatastoreEnv(w.Spec.Wandb.LegacyOverrides, m.env) // scopeDatastore only for now
		if err != nil {
			return ctrl.Result{}, err
		}
		if !found {
			continue
		}
		remove, err := m.apply(ctx, c, w, env)
		if err != nil {
			return ctrl.Result{}, err
		}
		if remove && removeLegacyEnv(w, m.env) { // strip from every section; prune empties
			changed = true
		}
	}
	if !changed {
		return ctrl.Result{}, nil
	}
	if err := c.Update(ctx, w); err != nil {
		return ctrl.Result{}, fmt.Errorf("update CR after legacy env mapping: %w", err)
	}
	return ctrl.Result{RequeueAfter: time.Second}, nil
}
```

`removeLegacyEnv` filters the named var out of every section's `Env`, drops a
section whose `Env` empties **and** has nil `Resources`, and deletes the map when
it empties (so `legacyOverrides` disappears cleanly, matching the "absent, not
empty map" invariant). Returns whether anything was removed.

**Note on `changed`:** gate the requeue on actual env **removal**, not on whether
a field was written. A guard-dropped var (managed ClickHouse) or a
user-field-wins skip still returns `remove=true` and must persist. An
unrepresentable `valueFrom` returns `remove=false` (passthrough) and persists
nothing for that entry. The `apply` Secret write is idempotent, so re-running
before the removal persists is safe.

### Step 5 — Wire into `Reconcile`

**File:** `internal/controller/reconciler/reconcile_v2.go` (after line 169)

```go
if res, migErr := migrateLegacyAnnotations(ctx, client, wandb); migErr != nil || res.RequeueAfter > 0 {
	return res, migErr
}

// Promote known legacy env vars into typed fields, then drop them from overrides.
if res, mapErr := mapLegacyEnvToCR(ctx, client, wandb); mapErr != nil || res.RequeueAfter > 0 {
	return res, mapErr
}
```

Runs on passes where `migrateLegacyAnnotations` is a no-op, so the external
ClickHouse connection already exists.

### Step 6 — Remove the env harvest from the conversion webhook, add the structured mapping

**File:** `api/v1/weightsandbiases_conversion_mapping.go`

- Delete the `mapClickHouseReplication` call in `applyValueMappings` (line ~104).
- Delete `mapClickHouseReplication`, `harvestClickHouseReplication`,
  `readClickHouseReplicationEnv`, `resolveClickHouseEnvFinding`,
  `clickHouseEnvFinding`, `readClickHousePendingAnnotation`, the
  `envClickHouseReplicated`/`envClickHouseReplicatedCluster`/`isClickHouseReplicationEnv`
  symbols, and `clickHousePendingClusterKey` (cluster is now env-only).
- **Keep** `nestedBoolLenient` and `clickHousePendingReplicatedKey`.
- In `mapClickHouse`, after the `clickHouseFields` loop and `passwordSecret` block,
  before the `if !sawField` gate — map the structured flag **only when a
  connection field was already seen** (preserves "replication needs an external
  connection; managed drops it"):

```go
if sawField {
	if flag, ok, err := nestedBoolLenient(chMap, "replicated"); err != nil {
		return fmt.Errorf("spec.values.global.clickhouse.replicated: %w", err)
	} else if ok {
		remaining[clickHousePendingReplicatedKey] = strconv.FormatBool(flag)
	}
}
```

`remaining` is the clickhouse-pending payload, and `migrateLegacyClickHouse`
already decodes a `replicated` key into `conn.Replicated` — so no reconciler
change is needed for the flag path.

**File:** `api/v1/weightsandbiases_conversion_overrides.go`

- Delete the `isClickHouseReplicationEnv(k)` exclusion (lines 252-257) — now the
  two env vars pass through into `legacyOverrides`. This is the pivotal change.

### Step 7 — migrate_legacy cluster cleanup (small)

**File:** `internal/controller/reconciler/migrate_legacy.go`

Cluster no longer travels through the pending annotation. Remove
`ReplicatedCluster` from `legacyClickHousePayload` and its
`fill(&conn.ClusterName, "replicatedCluster", …)` line. Keep the `Replicated`
field + its `fill` (fed by the structured flag). The env mapper now owns
`conn.ClusterName` and overrides of `conn.Replicated`.

### Step 8 — Tests

- **Move + rewrite** `api/v1/weightsandbiases_conversion_clickhouse_replication_test.go`
  → `internal/controller/reconciler/legacy_env_mapping_test.go`. Re-express its
  cases against `spec.wandb.legacyOverrides` input and Secret/selector output:
  env from app/global env & extraEnv, env-beats-extraEnv (already resolved by
  conversion, so assert passthrough ordering upstream), per-app-beats-global,
  conflicting-apps → error, non-boolean tolerated as a Secret string (the "is not
  a boolean" check was a webhook concern — decide whether to keep it; see Open
  items), managed → dropped, literal values → converted Secret keys, **env removed
  from overrides**, override-of-flag, user-owned-selector-respected. **New cases:**
  a `valueFrom.secretKeyRef` source → `conn.Replicated` points at the user's
  Secret and the env is removed; an unrepresentable `valueFrom` (e.g.
  `configMapKeyRef`) → field untouched and env **left** in `legacyOverrides`.
- **Trim** `api/v1/weightsandbiases_conversion_overrides_test.go`: drop the
  "replication vars stripped" expectation; add a passthrough assertion that the
  two vars now appear in `legacyOverrides`.
- **Adjust** `internal/controller/reconciler/migrate_legacy_test.go`: the
  ClickHouse replication-drain cases now cover only the structured-flag path
  (`replicated`, no `replicatedCluster`).
- **Add** a conversion test: `global.clickhouse{host, replicated:true}` →
  pending `replicated:"true"`.
- **Unchanged:** `legacy_overrides_test.go`, `clickhouse_replication_test.go`.

### Step 9 — Regenerate, doc, lint, test

- No `*_types.go` change, so codegen is a no-op — but run
  `make manifests generate sync-crd-embed` to be safe (CI gates on it).
- Update [docs/infra-connection-settings.md](../../infra-connection-settings.md):
  note the two replication env vars are now promoted to typed fields at reconcile.
- `make lint && make test`.

---

## Invariant → test coverage map

| Invariant (from the audit) | Where enforced after the change |
|---|---|
| env beats extraEnv | conversion `legacyEnvFromSection` (unchanged) + passthrough test |
| per-app beats global; apps disagree → error | `resolveDatastoreEnv` (Step 3) + table test |
| helm-template values dropped | conversion `legacyEnvVar` (unchanged) |
| managed ClickHouse → replication dropped | `externalClickHouseSelector` guard (Step 3) |
| literal values → `<cr>-clickhouse-converted` keys `replicated`/`replicatedCluster` | `externalClickHouseSelector` + `upsertConvertedSecretKeys` |
| `valueFrom.secretKeyRef` source preserved (field points at user Secret) | `externalClickHouseSelector` (Step 3) + new test |
| unrepresentable `valueFrom` kept as raw override (no data loss) | `externalClickHouseSelector` passthrough (Step 3) + new test |
| env vars removed from `legacyOverrides` | `removeLegacyEnv` (Step 4) |
| structured flag fallback (env beats flag) | flag → pending in `mapClickHouse`; env override in Step 3 |
| read side skips unpublished topology | `resolveEnvvars` `clickhouse` source (unchanged) |
| generic override/precedence/validate behavior | `legacy_overrides_test.go` (unchanged) |

## Open items to confirm during implementation

1. **Non-boolean `WF_CLICKHOUSE_REPLICATED`.** Today conversion errors ("is not a
   boolean"). At reconcile the value is just a Secret string; the app parses it —
   and for a `valueFrom.secretKeyRef` source the value isn't even in hand
   (it lives in the user's Secret), so a strict check couldn't run uniformly
   anyway. Recommend **dropping the hard error** (store the string, let the app
   decide). Flag if you want to keep it for the literal case only.
2. **Two-pass latency.** Flag drain (migrate_legacy) and env override land on
   separate reconcile passes. Both are ~1s requeues; acceptable and consistent
   with existing migration behavior. No change needed.
3. **`envClickHouseReplicated` constant location.** Moves from `api/v1` to
   `reconciler`. If any other package imported the v1 constants (grep says no),
   update accordingly.

## Suggested PR breakdown

- **PR 1** (Steps 1–5): add `mapLegacyEnvToCR` + registry + helpers + wiring, with
  new reconciler tests. The old conversion path still runs, so the two vars are
  still excluded from `legacyOverrides` — the new step is a no-op in prod but fully
  unit-tested. *(Optional: land the machinery before flipping the source.)*
- **PR 2** (Steps 6–9): remove the conversion harvest + exclusion, add the
  structured-flag mapping, migrate tests, docs, codegen. This is the behavior flip.

Landing as one PR is also fine given the shared test migration; split only if you
want the machinery reviewed before the flip.
