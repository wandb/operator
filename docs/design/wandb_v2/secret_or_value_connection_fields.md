# String-or-secret connection fields

**Status:** Implemented (Option 2) — validated by `make lint`/`make test` and westest
**Scope:** `api/v2` external-connection + OIDC fields
**Target release:** during v2 beta (`2.0.0-beta.3` today), before v2 GA

## Implementation status

All connection types (`MysqlConnection`, `RedisConnection`, `KafkaConnection`,
`ClickHouseConnection`, `ObjectStoreConnection`) and `OidcSpec` now use the
`ValueOrSecret` envelope (Option 2), and `ProxyValue` folded onto the shared type.
Delivered and green:

- `make build` / `make lint` (0 issues) / `make test` (exit 0).
- westest **`local-kind-ingress`** (managed SeaweedFS, status-side) and
  **`local-kind-external`** (external MySQL/Redis/ClickHouse/MinIO via the legacy
  `{name, key}` shape → normalized → `wandb-verify` presigned bucket round-trip) —
  both pass against a locally-built operator.
- The manifest `custom-resource` env resolver is union-aware (fixes the OIDC
  path); masq log-redaction is wired in `internal/logx`.

Still open: whether to **reject** a literal on strictly-secret fields (deferred,
see [Open decisions](#open-decisions)).

## Problem

Every field on the external-connection structs — and on `OidcSpec` — is typed
`corev1.SecretKeySelector`, so the CRD forces the user to create a Kubernetes
Secret and a per-field `{name, key}` selector *even for values that are plainly
not secret* (hostnames, ports, database names, bucket names, regions, TLS
flags, the OIDC issuer URL, …).

The external object-store fixture is the poster child
([external-objectstore/patch.yaml](../../../hack/testing-manifests/wandb/kustomize/overlays/external-objectstore/patch.yaml)):

```yaml
externalObjectStore:
  provider: { name: external-objectstore-connection, key: Provider }   # not secret
  endpoint: { name: external-objectstore-connection, key: Host }       # not secret
  port:     { name: external-objectstore-connection, key: Port }       # not secret
  bucket:   { name: external-objectstore-connection, key: Bucket }     # not secret
  region:   { name: external-objectstore-connection, key: Region }     # not secret
  accessKey:{ name: external-objectstore-connection, key: AccessKey }  # sensitive
  secretKey:{ name: external-objectstore-connection, key: SecretKey }  # SECRET
```

A user who just wants `bucket: my-bucket` and `region: us-west-2` has to stuff
those into a Secret first. We want every such field to accept **either a literal
string or a secret reference**, ideally without breaking any existing CR.

## Goals / non-goals

**Goals**

- Let each connection/OIDC field be supplied as a **literal value** or a
  **secret reference**.
- Apply the union type **uniformly to all fields** (per product direction),
  including ones that are secret today — the union does not by itself force a
  literal.
- Preserve existing CRs: an already-applied CR using the `{name, key}` shape
  must keep working, ideally with **no user-visible action**.
- Converge on a single, reusable field type and consumption path (today the same
  idea is expressed three different ways — see [Background](#background)).
- **Redact sensitive values from operator logs** so a secret placed in a field
  never reaches the log, independent of the reject-literal policy — see
  [Log redaction](#log-redaction-masq).

**Non-goals / explicitly deferred**

- **Whether to *reject* a literal on genuinely-secret fields** (password,
  secretKey, sslKey, clientSecret) is a **follow-up decision**, tracked in
  [Open decisions](#open-decisions). This doc makes every field *capable* of a
  literal; the policy of forbidding one on secret fields is out of scope here.
- Managed-infra connection generation is unchanged in behavior; only the Go type
  its status is written through changes.
- No change to how the operator materializes its own connection secret.

## Background

Three facts about the current design shape the solution.

### 1. There is already a value-or-secret precedent in v2

`spec.global.proxy.httpProxy` / `httpsProxy` are `*ProxyValue`
([weightsandbiases_types.go:197-217](../../../api/v2/weightsandbiases_types.go)):

```go
type ProxyValue struct {
    Value     string            `json:"value,omitempty"`     // literal
    ValueFrom *ProxyValueSource `json:"valueFrom,omitempty"` // secret
}
type ProxyValueSource struct {
    SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}
```

This mirrors `corev1.EnvVar`/`EnvVarSource`. The "exactly one of value/valueFrom"
invariant is enforced **imperatively in the validating webhook**
(`validateProxySpec`, [weightsandbiases_webhook.go:861](../../../internal/webhook/v2/weightsandbiases_webhook.go)),
not in the schema — the repo uses **zero** CEL `x-kubernetes-validations` rules
today. Consumption turns the union into an env var in `proxyValueEnvVars`
([proxy_env.go:61](../../../internal/controller/reconciler/proxy_env.go)):
literal `Value` → literal env var; `ValueFrom.SecretKeyRef` → a live
`SecretKeyRef` env source, so the credential never lands in the pod spec.

**The connection fields lack exactly this pattern.** The design is largely
"generalize `ProxyValue` to the connection/OIDC fields."

### 2. Consumption already collapses every field to a string

For each external infra type, `WriteState` builds a
`map[string]corev1.SecretKeySelector` and hands it to `ResolveFields`
([common.go:114](../../../internal/controller/infra/external/common.go)), which
dereferences each selector into a `map[string]string` and writes them all into
an **operator-owned connection secret** (`wandb-mysql-connection`, etc.). Apps
consume from *that* secret, never from the user's original
(e.g. [mysql.go:42](../../../internal/controller/infra/external/mysql/mysql.go),
[objectstore.go:43](../../../internal/controller/infra/external/objectstore/objectstore.go)).

Consequence: a **literal value slots straight into the `map[string]string`
with no secret read**. The resolution layer is the single chokepoint.

There are actually **three** places that decode "field → string or secret", and
they should converge on one helper:

| Path | Where | Today |
|------|-------|-------|
| External infra resolve | `ResolveFields` / `ResolveSecretKey` ([common.go:16](../../../internal/controller/infra/external/common.go)) | reads `SecretKeySelector` only |
| Manifest `custom-resource` env | `resolveCRFieldSecretSelector` + `resolveCRFieldEnvValue` ([reconcile_v2.go:1332-1367](../../../internal/controller/reconciler/reconcile_v2.go)) | tries `SecretKeySelector`, else literal scalar |
| Proxy env | `proxyValueEnvVars` ([proxy_env.go:61](../../../internal/controller/reconciler/proxy_env.go)) | already union-aware |

The **manifest `custom-resource` path is a live breakage risk**: the server
manifest can source an env var from a dotted CR path (e.g. an OIDC field). Today
`resolveCRFieldSecretSelector` unmarshals the terminal node as
`{name, key}`. If that node becomes a `{value, valueFrom}` union, **both**
resolvers fail to match it and the env var silently resolves to nothing. This
path must be made union-aware as part of the change.

### 3. The connection structs are dual-purpose (spec **and** status)

`MysqlConnection`, `RedisConnection`, `ObjectStoreConnection`, and
`ClickHouseConnection` are used **both** as user spec
(`spec.<infra>[].external*`) **and** as operator-written status
(`status.<infra>Status[].connection`, [weightsandbiases_types.go:840-863](../../../api/v2/weightsandbiases_types.go)).
`KafkaConnection` is **status-only** (Kafka is managed-only; there is no
`externalKafka`). Every `URL` field is operator-generated output.

Status writers construct these structs with `SecretKeySelector` literals
pointing at the operator connection secret — in both the external readers
([mysql.go:108](../../../internal/controller/infra/external/mysql/mysql.go))
and the managed writers (`moco/conn.go:95`, `opstree/conn.go:97`,
`bufstream/conn.go:93`, `altinity/conn.go:108`, `objectstore/secret.go:64`).
So **changing the struct type ripples into every status writer**, not just spec
input. This is fine — status always uses the *secret* arm of the union — but it
must be handled (see [Consumption changes](#consumption-changes)).

## The core constraint: no scalar-or-object in a structural schema

The nicest UX would be one field that accepts **either** a bare string **or** an
object on the same path:

```yaml
bucket: my-bucket                 # string
bucket: { name: s, key: Bucket }  # object
```

This is **not expressible in a structural CRD schema**. Structural schemas
require a single `type` per node. The only polymorphism escape hatches are:

- `x-kubernetes-int-or-string` — int-vs-string scalar only; cannot model
  scalar-vs-object.
- `x-kubernetes-preserve-unknown-fields` — allows anything but **disables
  validation and pruning** for that subtree. This CRD is served at v1+v2 with a
  conversion webhook and must stay structural; going schemaless on a typed field
  is a regression (it is confined today to the legacy v1 `spec.values` blob).

Therefore the field must be a **wrapper object** with mutually-exclusive
sub-fields — the `value` / `valueFrom` envelope, exactly like `ProxyValue`.
This is what forces the backward-compat discussion below: today the field *is*
the bare `{name, key}` object, and a wrapper changes that shape.

### Pruning happens before webhooks (why this matters)

For CRDs, unknown fields are **pruned at decode time, before mutating admission
webhooks run**. So a field that is not in the schema cannot be recovered by a
webhook. Any scheme that wants a webhook to *migrate* the legacy `{name, key}`
shape must **keep `name`/`key` in the schema** so they survive pruning long
enough for the webhook to move them. This is the linchpin of Option 2 below.

## Proposed type

Introduce one shared type (generalizing `ProxyValueSource`):

```go
// ValueOrSecret supplies a configuration value either as a literal or from a
// Secret key. Exactly one of Value or ValueFrom is set; the webhook enforces it.
type ValueOrSecret struct {
    // Value is a literal value.
    // +optional
    Value string `json:"value,omitempty"`

    // ValueFrom sources the value from a Secret key.
    // +optional
    ValueFrom *SecretValueSource `json:"valueFrom,omitempty"`
}

// SecretValueSource reads a value from a Secret key in the W&B namespace.
type SecretValueSource struct {
    // +optional
    SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}
```

**`ProxyValue` folds onto this shared type** (decided). `ProxyValue` /
`ProxyValueSource` are removed; `SecretValueSource` replaces `ProxyValueSource`;
`ProxySpec.HTTPProxy` / `HTTPSProxy` become `*ValueOrSecret`. Proxy already uses
the `value`/`valueFrom` envelope, so existing proxy CRs are unaffected; its
URL-specific checks (http/https scheme, userinfo rejection) stay in the webhook
as a layer on top of the shared exactly-one-of validator (see
[Validation](#validation)). Proxy fields inherit the deprecated legacy
`name`/`key` fields too — vestigial for proxy, removed at GA with everyone else.

Connection/OIDC fields change from `corev1.SecretKeySelector` to `ValueOrSecret`,
and `status.<infra>Status[].connection` uses the **same** `ValueOrSecret` object
as the spec (decided) — status always populates the secret arm. The definitive
shipped shape carries the deprecated legacy selector fields for backward
compatibility; see [Option 2](#option-2--valuevaluefrom-envelope--auto-normalizing-defaulter-recommended).

Shared consumption + construction helpers (one implementation, used by all three
paths in the table above):

```go
// SecretKeyRef returns the effective selector (ValueFrom.SecretKeyRef, else the
// legacy name/key), or nil for a literal/unset value.
func (v *ValueOrSecret) SecretKeyRef() *corev1.SecretKeySelector

// IsZero reports "neither literal nor secret ref set".
func (v *ValueOrSecret) IsZero() bool

// AsEnvVar produces a literal or SecretKeyRef-backed EnvVar (à la proxyValueEnvVars).
func (v *ValueOrSecret) AsEnvVar(name string) corev1.EnvVar

// Normalize rewrites the legacy {name,key} shape into ValueFrom. Each connection
// type + OidcSpec has its own Normalize() that calls this on every field; the
// defaulter invokes them (see Log-redaction / Validation).
func (v *ValueOrSecret) Normalize()

// LiteralValue / ValueFromSecret / ValueFromSelector are the constructors.
func LiteralValue(s string) ValueOrSecret
func ValueFromSecret(name, key string, optional bool) ValueOrSecret // status writers
func ValueFromSelector(sel corev1.SecretKeySelector) ValueOrSecret  // v1→v2 conversion

// Resolution (needs a client) lives in internal/controller/infra/external:
//   ResolveValue / ResolveValueFields; and utils.ConnSecretResolver.ValueOrSecret.
```

> **Naming note (implementation):** the status-writer constructor is
> `ValueFromSecret`, **not** `SecretRef` — `SecretRef` collides with the existing
> `type SecretRef struct` used by `ListenerTLSConfig.CertificateRef`.
>
> **Deprecation-lint note:** all access to the deprecated legacy `name`/`key`
> fields lives in `api/v2` methods (`SecretKeyRef`, `Normalize`). staticcheck
> `SA1019` exempts use within the declaring package, so consumers never touch the
> deprecated fields directly.

Secret-bearing fields on the connection/OIDC structs additionally carry a
`masq:"secret"` struct tag for log redaction — see
[Log redaction](#log-redaction-masq).

## Backward-compatibility options (the key decision)

The requirement is "existing CRs keep working, ideally with no user-visible
action." Two viable options; they differ in **API cleanliness now vs. lifecycle
cost**. A third (clean break) is listed as considered-and-rejected given the
no-break requirement.

### Option 1 — Flat superset (permanent), lowest cost

Make the type a superset that keeps the historical selector fields at the top
level and *adds* `value`:

```go
type ValueOrSecret struct {
    Value    string `json:"value,omitempty"`    // literal (new)
    Name     string `json:"name,omitempty"`     // secret ref (legacy shape, kept)
    Key      string `json:"key,omitempty"`
    Optional *bool  `json:"optional,omitempty"`
}
```

- Existing `host: {name, key}` **stays valid and populated** — zero migration,
  zero risk, nothing rewrites the user's object.
- New users write `host: {value: "db.example"}`.
- **Downside:** permanently diverges from the `value`/`valueFrom` idiom that
  `ProxyValue` established in the *same* CRD; `name`/`key` sitting alongside
  `value` is a little ad-hoc, and there is no `valueFrom` grouping.

Complexity: **low.** New type + deepcopy, union-aware resolve helpers, mechanical
constructor swaps in status writers and conversion, and per-field validation. No
mutating normalization, no deprecation lifecycle.

### Option 2 — `value`/`valueFrom` envelope + auto-normalizing defaulter (recommended)

Adopt the idiomatic envelope as **canonical**, but during the beta bridge keep
the legacy selector fields in the schema so they survive pruning, and have the
**mutating defaulter normalize them into `valueFrom` on admission**:

```go
type ValueOrSecret struct {
    Value     string             `json:"value,omitempty"`
    ValueFrom *SecretValueSource `json:"valueFrom,omitempty"`

    // Deprecated: legacy inline secret-ref shape, retained for backward compat
    // through v2 beta. The defaulter rewrites these into ValueFrom.SecretKeyRef.
    // Removed at v2 GA.
    Name     string `json:"name,omitempty"`
    Key      string `json:"key,omitempty"`
    Optional *bool  `json:"optional,omitempty"`
}
```

Normalization slots into the existing `WeightsAndBiasesCustomDefaulter.Default`
alongside `applyMySQLDefaults` etc.
([weightsandbiases_webhook.go:71](../../../internal/webhook/v2/weightsandbiases_webhook.go)):
for every connection/OIDC field, if `Name != ""` and `ValueFrom == nil`, set
`ValueFrom.SecretKeyRef = {Name, Key, Optional}` and clear the legacy fields.

- Existing `host: {name, key}` keeps working; on next apply the stored object is
  silently rewritten to `host: {valueFrom: {secretKeyRef: {name, key}}}` — the
  **user never takes an action**, matching the "automate it" ask.
- New users write the clean `value` / `valueFrom` shape.
- At **v2 GA**, drop `name`/`key`/`optional` from the type; everything stored has
  been normalized by then, and the API is fully idiomatic.
- The normalization logic mirrors the existing v1→v2 `classifyValueFromOrLiteral`
  helper ([weightsandbiases_conversion_mapping.go:784](../../../api/v1/weightsandbiases_conversion_mapping.go)),
  so the pattern is not new to the codebase.

Complexity: **Option 1 + a normalizing defaulter + a GA-removal task.** Schema is
a strict superset of Option 1 during the bridge (both carry `value` + legacy
`name`/`key`); Option 2 additionally carries `valueFrom` and the normalizer.

### Option 3 — Clean break (envelope only) — rejected

Envelope only, no legacy fields, document manual migration. Cleanest type and
least code, but an existing beta CR with `host: {name, key}` would be **pruned to
`host: {}` on upgrade — silent data loss**. Rejected given the no-break
requirement; listed for completeness (viable only if we accept editing every
existing v2 CR).

### Recommendation

**Decided: Option 2.** Rationale: v2 is still beta (`2.0.0-beta.3`), which is the cheap
moment to establish the clean, `ProxyValue`-consistent shape and schedule legacy
removal at GA; the normalizing defaulter delivers the "user never sees it"
guarantee; and doing it now avoids a *second* API migration later. If minimizing
scope is paramount and the team is comfortable with a permanently-flat union,
**Option 1 delivers the identical user-facing capability for materially less
work** — it is a legitimate fallback, not a wrong answer.

| | Option 1 (flat) | Option 2 (envelope + normalize) | Option 3 (clean break) |
|---|---|---|---|
| Existing CRs keep working | ✅ verbatim | ✅ auto-normalized | ❌ pruned/broken |
| User action required | none | none | edit every CR |
| End-state API shape | flat, non-idiomatic | idiomatic, matches ProxyValue | idiomatic |
| Mutating webhook work | none | normalizer (mechanical) | none |
| Lifecycle cost | none | deprecate + remove at GA | none |
| Second migration later? | maybe (if we later want envelope) | no | no |

## Field inventory & classification

From a repo-wide audit of `corev1.SecretKeySelector` in `api/v2`. All live in
[weightsandbiases_types.go](../../../api/v2/weightsandbiases_types.go);
`application_types.go` has none.

| Struct | Field(s) | Class | Notes |
|---|---|---|---|
| `MysqlConnection` | host, port, database, tls, sslCa, sslCert | non-secret | |
| | username | arguable | credential-ish, often not secret |
| | password, sslKey | **secret** | |
| | url | output | operator-generated DSN |
| `RedisConnection` | host, port, tls, sslCa | non-secret | |
| | password | **secret** | |
| | url | output | |
| `ClickHouseConnection` | host, tcpPort, httpPort, database | non-secret | |
| | username | arguable | |
| | password | **secret** | |
| | url | output | |
| `ObjectStoreConnection` | provider, endpoint, port, bucket, path, region, tlsEnabled, forcePathStyle | non-secret | |
| | accessKey | arguable | key *id* |
| | secretKey | **secret** | |
| | url | output | |
| `KafkaConnection` | host, port, brokerEndpoint, clusterID, url | output | status-only (managed-only), no `externalKafka` |
| `OidcSpec` | issuerUrl, authMethod | non-secret | |
| | clientId | arguable | |
| | clientSecret | **secret** | |

Per product direction, **all** of these become `ValueOrSecret`. `url` and the
`KafkaConnection` fields are operator-generated output; they simply never use the
literal arm. The genuinely-secret set (password, sslKey, secretKey,
clientSecret) is what the deferred "reject literal" policy would target.

## Consumption changes (as implemented)

1. **External resolve** — added `ResolveValue` / `ResolveValueFields`
   ([common.go](../../../internal/controller/infra/external/common.go)) alongside
   the (now unused for connections) `ResolveFields`; a literal is used as-is, a
   secret arm is read via `ResolveSecretKey`. All external `WriteState`s
   (mysql/redis/clickhouse/objectstore) feed `ResolveValueFields`. The
   objectstore read path (`utils.ConnSecretResolver`) gained a `ValueOrSecret`
   method.

2. **Manifest `custom-resource` resolver** (done) —
   `resolveCRFieldSecretSelector` / `resolveCRFieldEnvValue`
   ([reconcile_v2.go](../../../internal/controller/reconciler/reconcile_v2.go))
   are union-aware: the terminal CR node is unmarshalled into `ValueOrSecret`;
   `SecretKeyRef()` → a `SecretKeyRef` env source, a literal → an env value. This
   fixed the OIDC breakage noted in [Background](#background).

3. **Status writers** — external `ReadState` and managed writers
   (`moco/conn.go`, `opstree/conn.go`, `bufstream/conn.go`, `altinity/conn.go`,
   `objectstore/secret.go`) construct the connection struct via
   `apiv2.ValueFromSecret(name, key, optional)` instead of `SecretKeySelector{…}`.
   The internal `objectstore.ConnInfo.*Ref` fields stay `corev1.SecretKeySelector`
   (status is always secret-backed), consumed by ClickHouse/Bufstream.

4. **Other consumers updated to `SecretKeyRef()`**: `custom_ca.go` (external
   MySQL/Redis TLS CA volume mounts + checksum), `kafka.go` (bootstrap host from
   the status connection), and every `pods.go` env-source case
   (mysql/redis/clickhouse/kafka/bucket).
   Mechanical.

4. **Env construction** — `proxyValueEnvVars`
   ([proxy_env.go:61](../../../internal/controller/reconciler/proxy_env.go)) is
   retyped to `*ValueOrSecret` (or replaced by the shared `AsEnvVar`), and OIDC /
   any directly-surfaced connection field uses the same helper — preserving the
   "secret stays a `SecretKeyRef`, never a literal in the pod spec" property.

## Validation

Follow the `ProxyValue` precedent — **imperative Go in the validating webhook**,
consistent with the repo (no CEL today):

- Generalize a `validateValueOrSecret(v, path)` helper enforcing **exactly one of
  `value` / `valueFrom`** (and, during the Option 2 bridge, treating legacy
  `name`/`key` as the secret arm). Replaces the selector-only
  `validateRequiredSecretSelector`
  ([weightsandbiases_webhook.go:552](../../../internal/webhook/v2/weightsandbiases_webhook.go))
  and is called from each per-infra validator (`validateMySQLSpec`,
  `validateObjectStoreSpec`, …). `validateProxySpec` reuses the same helper for
  the exactly-one-of check and keeps its URL scheme / userinfo checks as
  proxy-specific additions on top.
- Keep the credential-redaction discipline: when rejecting a literal that might
  contain a secret, pass a constant `"[redacted]"` as the `field.Invalid` value
  (as `validateProxySpec` does), never the offending string.
- **Deferred:** whether to reject a literal on the secret-classified fields (see
  [Open decisions](#open-decisions)). If adopted, it is one extra check in the
  same helper, keyed by a per-field "isSecret" flag.
- **CEL: skipped for now** (decided). A single CEL rule
  `has(self.value) != has(self.valueFrom)` per field would enforce the exclusion
  even when the webhook is bypassed (GitOps dry-run), and remains available
  (k8s 1.35, controller-gen v0.19.0) as a later defense-in-depth add — but it
  would be the repo's first CEL rule, and the Go webhook is authoritative
  regardless. Not in scope for this change.

## Log redaction (masq)

**Why now.** The union lets a user place a real secret in a field's `value`, and
until the [deferred reject-literal policy](#open-decisions) forbids that, the
operator can log connection data during reconcile. Concretely, the manifest
`custom-resource` resolver already logs a *resolved literal* value at debug level
(`logger.Debug("field found in CR", …, "value", val)`,
[pods.go:418](../../../internal/controller/reconciler/pods.go)), and the
resolved-fields path (see below) holds plaintext credentials in memory. We want
defense-in-depth: a sensitive value should never reach the operator log even if
it reaches the CR.

**Library.** [`github.com/m-mizutani/masq`](https://github.com/m-mizutani/masq)
(Apache-2.0 — matches the operator; min Go 1.24, fine on the repo's 1.26).
`masq.New(opts...)` returns a `ReplaceAttr` function for `slog.HandlerOptions`.
The operator **already logs through `slog`** ([main.go:165](../../../cmd/manager/main.go),
[internal/logx](../../../internal/logx/handler.go)), so masq drops in with no
change to the logging stack.

**Wiring — one central place, composed with the existing `ReplaceAttr`.**
`internal/logx` owns handler construction ([handler.go:10](../../../internal/logx/handler.go)):
JSON/Text handlers read `ReplaceAttr` off `opts.HandlerOptions` (unset today),
while the Pretty/tint handler sets its **own** `ReplaceAttr`
([pretty.go:14](../../../internal/logx/pretty.go)). There is only one
`ReplaceAttr` slot per handler, so masq must be **composed**, not assigned
blindly:

```go
// internal/logx — build once, apply to every format.
var redact = masq.New(masq.WithTag("secret")) // default tag key is "masq"

func chainReplaceAttr(fns ...func([]string, slog.Attr) slog.Attr) func([]string, slog.Attr) slog.Attr {
    return func(groups []string, a slog.Attr) slog.Attr {
        for _, fn := range fns {
            if fn != nil {
                a = fn(groups, a)
            }
        }
        return a
    }
}
```

- **JSON/Text:** in `withDefaults`/`NewHandler`, set
  `opts.HandlerOptions.ReplaceAttr = chainReplaceAttr(opts.HandlerOptions.ReplaceAttr, redact)`.
- **Pretty:** `BuildPrettyHandler` composes `redact` with its existing LoggerKey
  rename (they touch different attrs, so order is safe; run masq last so
  redaction is final).

Centralizing in `logx` covers every logger — controller-runtime via
`NewLogrLogger` and the direct `NewSlogLogger`.

**Tagging.** With `masq.WithTag("secret")`, any struct field tagged
`masq:"secret"` is replaced wholesale with masq's redaction placeholder when that
struct is logged as an attribute:

```go
type MysqlConnection struct {
    // ...
    Password ValueOrSecret `json:"password,omitempty" masq:"secret"`
    SslKey   ValueOrSecret `json:"sslKey,omitempty"   masq:"secret"`
    URL      ValueOrSecret `json:"url,omitempty"      masq:"secret"` // assembled DSN embeds the password
}
```

Fields to tag `masq:"secret"`:

| Struct | Fields |
|---|---|
| `MysqlConnection` | password, sslKey, url |
| `RedisConnection` | password, url |
| `ClickHouseConnection` | password, url |
| `ObjectStoreConnection` | secretKey, accessKey, url |
| `OidcSpec` | clientSecret |
| `KafkaConnection` | url |

`accessKey` and every `url` are tagged because the assembled S3/GCS/Azure and DSN
URLs embed credentials as userinfo (see the URL builders in
[objectstore.go](../../../internal/controller/infra/external/objectstore/objectstore.go)
and [mysql.go](../../../internal/controller/infra/external/mysql/mysql.go)). The
`masq` tag is inert to controller-gen and deepcopy-gen — **no CRD/schema impact**.

**Honest limitation — tagging is necessary but not sufficient.** Tag-based
masking fires only when a value is logged **as part of the tagged struct**. Two
leak paths it does *not* cover:

1. The resolved `map[string]string` in `ResolveFields`/`WriteState`
   ([common.go:114](../../../internal/controller/infra/external/common.go),
   [mysql.go:88](../../../internal/controller/infra/external/mysql/mysql.go))
   holds the plaintext password and the full DSN — a bare map, not a tagged
   struct.
2. Ad-hoc string logging like the `pods.go:418` debug line above (the literal
   arm of the `custom-resource` resolver).

Mitigations, most robust first:

- **Discipline:** never log the resolved connection map or a resolved value;
  audit and remove/guard existing sites (the `pods.go:418` debug line is the
  known offender).
- **Typed secret + `WithType`:** give resolved secret strings a dedicated type
  (`type Secret string`) and add `masq.WithType[Secret]()`, so a secret is masked
  wherever it is logged, independent of struct context. Strongest option for the
  resolved-values path; recommended alongside tagging.
- **Coarse censors:** `masq.WithFieldName("password"|"secretKey"|…)` /
  `masq.WithContain(...)` as belt-and-suspenders. Lower precision; optional.

**Residual risk:** a secret typed into a *non-secret* field's `value` is not
masked (that field isn't tagged). Closing that residual is exactly the deferred
reject-literal policy; masking the known-secret fields is the interim mitigation.

## Conversion & legacy-migration impact

`ConvertFrom` (v2→v1) round-trips through annotations and never reads these Go
types — **unaffected**. The v1→v2 direction and the reconciler's legacy
migration construct `SecretKeySelector` literals and must be updated to build the
`ValueOrSecret` secret arm instead:

- `api/v1/weightsandbiases_conversion_mapping.go` — the `setRef` field-tables
  (`mysqlFields`, `redisFields`, `clickHouseFields`, `oidcFields`), the
  `mapBucket` access/secret-key literals, and the legacy password/clientSecret
  ref blocks. `classifyValueFromOrLiteral`
  ([:784](../../../api/v1/weightsandbiases_conversion_mapping.go)) already
  distinguishes literal vs secret — it maps cleanly onto `ValueOrSecret`
  (literal → `Value`; valueFrom → the secret arm), arguably *simplifying* this
  code.
- `internal/controller/reconciler/migrate_legacy.go` — the `fill` closures now
  target `*apiv2.ValueOrSecret`, guard on `!target.IsZero()`, and assign
  `apiv2.ValueFromSecret(secretName, dataKey, false)`. The old `secretSelector`
  helper was removed.

Done as implemented; the compiler enumerated every site. `ConvertFrom` (annotation
round-trip) was untouched, as predicted.

## Worked example (object store)

Before (today, all-secret-ref):

```yaml
externalObjectStore:
  endpoint: { name: os-conn, key: Host }
  bucket:   { name: os-conn, key: Bucket }
  region:   { name: os-conn, key: Region }
  accessKey:{ name: os-conn, key: AccessKey }
  secretKey:{ name: os-conn, key: SecretKey }
```

After (Option 2 — literals for non-secrets, secret arm for credentials):

```yaml
externalObjectStore:
  endpoint: { value: s3.us-west-2.amazonaws.com }
  bucket:   { value: my-wandb-bucket }
  region:   { value: us-west-2 }
  accessKey:{ valueFrom: { secretKeyRef: { name: os-conn, key: AccessKey } } }
  secretKey:{ valueFrom: { secretKeyRef: { name: os-conn, key: SecretKey } } }
```

The existing all-`{name, key}` form continues to apply unchanged; under Option 2
the defaulter rewrites each entry to the `valueFrom` form on next admission.

## Rollout (completed)

1. ✅ Added `ValueOrSecret` / `SecretValueSource` + helpers; switched all
   connection/OIDC fields to the Option 2 envelope (deprecated legacy
   `name`/`key`/`optional` retained through beta). Folded
   `ProxyValue`/`ProxyValueSource` onto `ValueOrSecret`/`SecretValueSource`
   (retyped `ProxySpec.HTTPProxy`/`HTTPSProxy`; updated `proxy_env.go`). Added
   `masq:"secret"` tags to the secret-bearing fields (password/sslKey/secretKey/
   clientSecret + credential-bearing URLs).
2. ✅ `make manifests generate sync-crd-embed`.
3. ✅ Updated external resolve, the `custom-resource` resolver, status writers,
   conversion (`mapMySQL/Redis/ClickHouse/OIDC/Bucket`), `migrate_legacy`,
   `custom_ca.go`, `kafka.go`, `pods.go`, and per-type webhook validation.
4. ✅ Added the normalizing defaulter (`normalizeConnections`, all external
   conns + OIDC); deprecation markers on the legacy fields. **Follow-up: drop
   legacy `name`/`key`/`optional` at v2 GA.**
5. ✅ Wired masq into `internal/logx` (`masq.New(masq.WithTag("secret"))` composed
   via a `chainReplaceAttr` helper into the JSON/Text `HandlerOptions` and the
   Pretty/tint handler); added `redact_test.go`.
6. ⏳ Fixture refresh under
   [hack/testing-manifests/](../../../hack/testing-manifests/) to showcase
   literals is optional (the defaulter normalizes the existing all-secret-ref
   fixtures); westest `local-kind-external` already exercises the legacy shape.
7. ✅ `make lint` (0 issues) && `make test` (exit 0); westest `local-kind-ingress`
   + `local-kind-external` green.

## Open decisions

1. **Backward-compat approach:** ✅ **Decided — Option 2** (envelope +
   normalizing defaulter + deprecate legacy at GA).
2. **Reject literals on secret fields?** ⏳ **Still open** (deferred, per
   product) — allow everywhere / reject on {password, sslKey, secretKey,
   clientSecret} / warn. Log redaction is the interim mitigation until this
   lands, and the typed-`Secret` question (below) rides along with it.
3. **Log redaction — typed `Secret` + `masq.WithType`?** ✅ **Decided — not
   now.** Rely on struct-field tags + logging discipline; revisit together with
   decision #2.
4. **Mask the *arguable* fields (`username`/`clientId`)?** ✅ **Decided — no.**
   Only the strictly-secret set is tagged, plus `accessKey` and every `url`
   (URLs embed credentials as userinfo). `username`/`clientId` stay untagged.
5. **Fold `ProxyValue` onto the shared `ValueOrSecret`?** ✅ **Decided — yes**,
   as part of this change.
6. **CEL exclusion rule?** ✅ **Decided — no** (Go-only for now; CEL noted as a
   possible later add).
7. **Type used for status?** ✅ **Decided — shared `ValueOrSecret`**, same
   object as the spec (status always uses the secret arm).
