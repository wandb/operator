# Operator ↔ Server-Manifest Version Compatibility

**Status:** Draft / proposed
**Area:** `pkg/wandb/manifest`, `internal/webhook/v2`, operator version embedding

## Summary

The operator and the W&B server manifest evolve independently and are released
on different cadences. Certain operator changes require a newer manifest, and
certain manifest changes require a newer operator. Today nothing enforces this:
the manifest carries a `requiredOperatorVersion` field and a `manifestVersion`
field, but the operator parses neither (`manifestVersion` isn't even on the
struct) and validates neither.

This document proposes a **two-sided compatibility contract** enforced at
**admission time** by the validating webhook, **fail-closed**, using both
existing fields:

- `requiredOperatorVersion` — the manifest declares which operator versions can
  process it (semver constraint).
- `manifestVersion` — a coarse, monotonic integer schema-contract version; the
  operator embeds the **explicit set** of manifest versions it supports.

## Problem statement

Compatibility breaks in **both directions**:

1. **Operator needs a newer manifest.** We ship an operator change that depends
   on manifest content/shape that older manifests don't have. Users must not be
   able to pin an older manifest against the new operator.
2. **Manifest needs a newer operator.** Upstream makes a backwards-incompatible
   change to the manifest. Older operators must refuse to install it rather than
   mis-reconcile.

A single one-directional field cannot express both — each side must be able to
state a requirement about the other.

### Three version axes (don't conflate them)

| Axis | Example | Owned by | Changes |
|------|---------|----------|---------|
| **Operator version** | `2.1.0` | this repo (git tag / chart `appVersion`) | per operator release |
| **W&B server version** (`spec.wandb.version`) | `0.79.0` | upstream W&B releases | frequently |
| **Manifest schema version** (`manifestVersion`) | `3` | upstream manifest generator | rarely (only on structural breaks) |

Compatibility is gated on **operator version ↔ manifest schema version**, *not*
on the W&B server version. Server versions churn constantly; we deliberately do
**not** want to maintain a per-server-version compatibility table. The manifest
schema version is the slow-moving structural contract — that is the correct
thing to gate on.

## Goals

- Express and enforce compatibility in both directions.
- Fail fast and legibly at `kubectl apply` time (admission rejection with a
  human-readable reason), so a bad version pin never reaches reconciliation.
- Keep the common case zero-friction: once upstream cuts compatible manifests,
  nothing special is required of users.
- Avoid maintaining a per-server-version matrix.

## Non-goals

- Gating on the W&B **server** version itself.
- Auto-selecting a compatible version for the user (we reject; we don't rewrite).
- Runtime/reconcile-time enforcement as the primary gate (see "Enforcement").

## Design

### The two-sided contract

Two complementary fields, with a clear division of labor.

#### `manifestVersion` — monotonic integer schema-contract version

- Type: **integer** (`1`, `2`, `3`, …). Replaces the current stringly-typed
  `v1alpha1` value seen in test manifests.
- Bumped **only** on a backwards-incompatible change to the manifest *structure*
  — a new required field the operator must understand, a renamed/removed field,
  a changed meaning. These are expected to be **rare**.
- The operator embeds an **explicit set** of supported versions, e.g.
  `CompatibleManifestVersions = {2, 3}` — *not* a floor/ceiling range. A discrete
  set is simple, exact, and avoids implying support for versions we never tested.
  It also lets us drop support for an old version (remove `2` from the set)
  independently of adding a new one.

This single field handles **both directions at the structural-break granularity**:

- Manifest `manifestVersion` ∉ operator's set, and it's **higher** than anything
  we know → manifest is too new, operator too old → reject (direction 2).
- Manifest `manifestVersion` ∉ operator's set, and it's **lower** than our
  minimum → operator dropped support for that shape → reject (direction 1).

#### `requiredOperatorVersion` — semver constraint on the operator

- Type: **semver constraint string** (e.g. `>=2.3.0 <3.0.0`), parsed with
  `github.com/Masterminds/semver/v3` (already in `go.mod`).
- The manifest declares the operator versions that can correctly process it; the
  operator checks its **own embedded version** against the constraint.
- This is the **fine-grained** lever. Not every "you need a newer operator"
  requirement is a structural manifest break. Example: the operator fixes how it
  provisions Kafka and a manifest relies on the new behavior — the manifest
  *structure* is unchanged (`manifestVersion` stays the same) but we still want
  to require operator `>=2.4.0`. `manifestVersion` can't express that;
  `requiredOperatorVersion` can.

#### Why both

| Concern | Field | Granularity | Owned by |
|---------|-------|-------------|----------|
| Structural contract of the manifest document | `manifestVersion` | coarse, integer set | operator (the set) + manifest (its value) |
| Minimum/range operator *behavior* a manifest depends on | `requiredOperatorVersion` | fine, semver | manifest |

Both checks must pass (logical AND).

### Operator self-version

The binary currently embeds no version. Introduce a small package, e.g.
`internal/version` (or `pkg/version`), exposing:

```go
package version

// Version is the operator's semantic version, injected at build time.
// Falls back to a sentinel for `go run` / tests.
var Version = "0.0.0-dev"
```

Injected via linker flag in the Dockerfile/Makefile, driven by the chart
`appVersion` or git tag:

```
go build -ldflags "-X github.com/wandb/operator/internal/version.Version=${VERSION}" ./cmd/manager
```

Both the webhook and (later, if desired) the reconciler read
`version.Version`. The same value should be surfaced in logs at startup.

### Prerelease handling (important)

`github.com/Masterminds/semver/v3` does **not** match prereleases against range
constraints by default: `2.0.0-alpha.2` does **not** satisfy `^2.0.0` or
`>=2.0.0`. The operator chart is currently `2.0.0-alpha.2`, so a naive check
would reject every manifest today.

**Policy:** compare on the **release version with prerelease/build metadata
stripped**. `2.0.0-alpha.2` is treated as `2.0.0` for the purpose of the
`requiredOperatorVersion` check. Implementation: parse the operator version,
take `Major/Minor/Patch`, and check that finalized version against the
constraint (or use a constraint built with prerelease-inclusive options). This
keeps alpha/rc builds usable during development while preserving range
semantics. Document this clearly so upstream constraint authors know prereleases
are floored to their release version.

### Strict / fail-closed semantics

Both fields are **required and must be valid**. The manifest is rejected if:

- `manifestVersion` is absent, ≤ 0, or non-integer.
- `requiredOperatorVersion` is absent or not a parseable semver constraint.
- `manifestVersion` ∉ `CompatibleManifestVersions`.
- the operator's (prerelease-floored) version does not satisfy
  `requiredOperatorVersion`.

This means **every currently-published manifest must be re-cut** to carry both
fields before it will admit. Given v2 is still in alpha, this is acceptable and
is called out in Migration below.

## Enforcement: validating webhook (admission)

Enforcement lives in the existing validating webhook
(`internal/webhook/v2/weightsandbiases_webhook.go`), invoked on **create and
update** — which is exactly when a user pins or changes `spec.wandb.version` /
`spec.wandb.manifestRepository`. A new `validateManifestCompatibility(ctx, wandb)`
is added to the `validateSpec` orchestrator. Returning an `error` rejects
admission and the message is shown directly to the user at `kubectl apply`.

### Why webhook-only is feasible here

The obvious objection to admission-time enforcement is that it must **fetch the
OCI manifest** to read the two fields. That is acceptable because:

- The webhook runs **in-process** in the operator pod — same network egress and
  registry credentials as the reconciler.
- `manifest.GetServerManifest` already **caches** pulled artifacts into a local
  OCI store (`/tmp/server-manifest`). The first admission of a given
  `(repository, version)` pulls; subsequent admissions hit the cache.
- Reconciliation pulls the same manifest moments later, so the webhook fetch
  warms the cache it will use anyway.

### Failure modes

Because we are fail-closed:

| Situation | Result | Message |
|-----------|--------|---------|
| Operator version not in `requiredOperatorVersion` | **reject** | "manifest \<v\> requires operator \<constraint\>; this operator is \<version\>. Upgrade the operator or select a compatible manifest version." |
| `manifestVersion` not in operator's set | **reject** | "manifest \<v\> uses schema version N; this operator supports {…}. …" |
| Field missing / unparseable | **reject** | field-path error via `field.Invalid` |
| **Registry unreachable / pull fails** | **reject** | "could not fetch manifest \<repo\>:\<version\> to validate compatibility: \<err\>" |

The last row is the operationally significant one: a registry outage will
**block CR applies/updates** (including unrelated spec edits, since the webhook
fires on every update). Mitigations:

- The local OCI cache means previously-validated versions still admit offline.
- Apply a bounded timeout to the webhook fetch so admission fails fast with a
  clear message rather than hanging.
- Existing CRs continue reconciling regardless — admission only gates *new
  writes*, not the running deployment.

This trade-off is inherent to the "webhook-only" decision and is accepted; it is
documented here so the on-call behavior is not surprising.

## Implementation plan

1. **`pkg/wandb/manifest/manifest.go`** — add `ManifestVersion int` to the
   `Manifest` struct (key `manifestVersion`; matched case-insensitively by
   `sigs.k8s.io/yaml`). Handle it in `mergeSimple` (take the first non-zero, and
   detect conflicting values across merged files).
2. **`internal/version/version.go`** — new package with the build-injected
   `Version` var.
3. **Build wiring** — `-ldflags -X` in `Dockerfile`/`Dockerfile.cross` and the
   relevant `Makefile` build targets, sourced from chart `appVersion` or git tag.
4. **`pkg/wandb/manifest/compat.go`** (new) — the compatibility logic:
   - `CompatibleManifestVersions` set (the operator-owned source of truth).
   - `CheckCompatibility(operatorVersion string, m Manifest) error` doing both
     checks with prerelease flooring. Pure and unit-testable, no I/O.
5. **`internal/webhook/v2/weightsandbiases_webhook.go`** — add
   `validateManifestCompatibility`: fetch via `GetServerManifest`, then call
   `manifest.CheckCompatibility(version.Version, m)`; wire into `validateSpec`.
6. **Re-cut local fixtures** under `hack/testing-manifests/server-manifest/*`
   with integer `manifestVersion` and a valid `requiredOperatorVersion`.
7. **Tests** — table-driven unit tests for `CheckCompatibility` (in-set,
   out-of-set high/low, satisfied/unsatisfied constraint, prerelease operator,
   missing/garbage fields); webhook tests for accept/reject and the
   registry-unreachable path.
8. **Docs** — `docs/config-api.md` and an upstream note in wandb/core's
   `onprem/server-manifest` generator describing the contract and the integer
   `manifestVersion` bump policy.

## Worked examples

- **Operator `2.4.0`, manifest `manifestVersion: 3`, `requiredOperatorVersion: ">=2.4.0 <3.0.0"`**, operator set `{2,3}` → admit.
- **Operator `2.3.0`** with the same manifest → reject: operator below
  `requiredOperatorVersion` floor (direction 2, fine-grained).
- **Operator `2.4.0` supporting `{3,4}`, manifest `manifestVersion: 2`** → reject:
  schema too old, support dropped (direction 1).
- **Operator `2.4.0` supporting `{2,3}`, manifest `manifestVersion: 4`** → reject:
  schema too new for this operator (direction 2, structural).
- **Manifest missing `manifestVersion`** → reject (fail-closed).

## Open questions

1. **Source of the build version** — git tag vs chart `appVersion`. They should
   not drift; pick one as canonical (recommend git tag, with CI asserting the
   chart matches).
2. **`manifestVersion` set maintenance** — where does the canonical set live and
   how is dropping an old version reviewed? Proposal: a single constant in
   `compat.go` with a comment block documenting each version's meaning.
3. **Conversion / older OCI tags** — do we backfill the two fields into already
   published manifests, or only enforce for versions cut after this lands? With
   fail-closed, un-backfilled old versions become unschedulable; confirm that's
   intended.
4. **Webhook timeout value** — what bound on the OCI fetch keeps admission
   responsive without flaking on a cold cache + slow registry?
