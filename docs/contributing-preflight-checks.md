# Contributing a preflight check

Preflight checks live in `pkg/preflight`. Every check is listed in the `Checks` registry in `pkg/preflight/registry.go`. Anything that walks the registry (such as `wsm preflight`) picks up a new check automatically once wsm is bumped to an operator version that includes it. No wsm code change is needed.

Adding a check takes three steps: write the check, register it, and test it.

## 1. Write the check: `pkg/preflight/<name>.go`

Create one file per check, named after what it checks (for example `managed_kafka_storage.go`). A check is a function that takes the typed struct found at its CR field and returns a `Result`. The example used throughout this guide validates `spec.kafka.managedKafka.storageSize`:

```go
package preflight

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/api/resource"
)

// RunManagedKafkaStorageCheck validates spec.kafka.managedKafka.storageSize before the PVC is created with it.
func RunManagedKafkaStorageCheck(_ context.Context, spec *apiv2.ManagedKafkaSpec, _ Env) Result {
	if spec.StorageSize == "" {
		return Result{Check: ManagedKafkaStorageCheck, Outcome: OutcomePass, Message: "storageSize not set; default applies"}
	}
	if _, err := resource.ParseQuantity(spec.StorageSize); err != nil {
		return Result{Check: ManagedKafkaStorageCheck, Outcome: OutcomeFail,
			Message: fmt.Sprintf("storageSize %q is not a valid quantity", spec.StorageSize)}
	}
	return Result{Check: ManagedKafkaStorageCheck, Outcome: OutcomePass}
}
```

Conventions:

- **Resolve every `ValueOrSecret` through `env.Resolve`**, never by reading Secrets directly. Pass resolver errors to `resolveFailure`, which reports `SKIPPED` when Secrets are unavailable (wsm `--offline`) and `UNKNOWN` otherwise.
- **Pick the outcome carefully.**
  - `FAIL`: the config is definitely wrong.
  - `UNKNOWN`: the check couldn't decide (for example, discovery was forbidden or a server gave an unexpected response).
  - `SKIPPED`: the check couldn't run (for example, it had no credentials).
- **Never echo secrets in `Message`.** Messages land in CR status and CLI output; use `redact(...)` on driver errors.
- **Cluster checks** use `env.Mapper` (a `meta.RESTMapper`). Return `UNKNOWN` when it's nil.

## 2. Register it: `pkg/preflight/registry.go`

Add a name constant to the `const` block:

```go
const (
	// ...
	ManagedKafkaStorageCheck = "managedKafkaStorageCheck"
)
```

Add an entry to `Checks`:

```go
var Checks = map[string]Check{
	// ...
	ManagedKafkaStorageCheck: typedCheck(ManagedKafkaStorageCheck, "spec.kafka.managedKafka",
		TypeStatic, []ResourceReq{ReqReadOnly}, RunManagedKafkaStorageCheck),
}
```

The arguments to `typedCheck` are:

| Argument | Meaning |
|---|---|
| name | The constant above. Users also pass it in the `skipPreflights` annotation. |
| CR field | Dotted path that triggers the check when present. Use `*` for instance-map keys (`spec.mysql.*.externalMysql`); `spec.kafka` is not a map, so it needs no `*`. The value at this path is decoded into your function's spec type, so the two must match. |
| type | `TypeStatic` (inspects the CR only), `TypeCluster` (queries the Kubernetes API), or `TypeConnection` (dials a service from wherever the check runs). |
| resource reqs | `ReqReadOnly`, `ReqReadWrite`, or `ReqResourceCreate`. `wsm` asks the user before running any `ReqResourceCreate` check. A check that creates a preflight pod in the cluster must declare it. |
| run func | Your `Run...` function. `typedCheck` adapts it to the registry's untyped signature. |

### Relationships between checks: `withDependsOn` and `withCovers`

Two checks are sometimes related: one only makes sense once another has passed, or one proves what another tried to prove from a better place. To record that, wrap the `typedCheck(...)` call in `registry.go`. Each helper takes the check, then one or more names of other checks:

```go
withDependsOn(typedCheck(...), PrerequisiteCheck, ...)
withCovers(typedCheck(...), CoveredCheck, ...)
```

Both helpers copy the existing slice before appending, so they can be nested:

```go
withCovers(withDependsOn(typedCheck(...), A), B)
```

Always refer to other checks by their name constants, never by string literals.

The operator only records these relationships on `Check.DependsOn` and `Check.Covers`. They're applied by whatever runs the checks: `ExecuteChecks` in wsm's `pkg/preflight`, which Watchtower also uses. The operator's own reconcile gate (`gatePreflight`) ignores them.

#### `withDependsOn`: run only after a prerequisite passes

Use this when your check's answer is meaningless, or would fail misleadingly, unless another check has passed. For example, `clusterCapacityCheck` sizes the cluster from the server manifest, so it depends on `serverManifestCheck`:

```go
ClusterCapacityCheck: withDependsOn(typedCheck(ClusterCapacityCheck, "spec",
	TypeCluster, []ResourceReq{ReqReadOnly}, RunClusterCapacityCheck), ServerManifestCheck),
```

What the runner does:

- **Order.** Prerequisites run before the checks that depend on them. Otherwise the registry order is kept.
- **Skip when a prerequisite didn't pass.** If a prerequisite ran and its outcome was anything other than `PASS` (`FAIL`, `UNKNOWN` or `SKIPPED`), the dependent check doesn't run. It reports `SKIPPED` with the message `requires <prerequisite> to pass`.
- **Every instance must pass.** If the prerequisite runs more than once (for example, once per `spec.mysql.*` instance), all of those runs must pass.
- **Unmatched prerequisites don't block.** A prerequisite that never matched the CR (its field isn't set, or the `skipPreflights` annotation removed it) doesn't stop the dependent check. Nothing ran, so there's nothing to block on.

Rules, enforced by `TestRegistryRelations` in `relations_test.go`:

- Every name must be a registered check.
- A check can't depend on itself.
- There must be no cycles (`A → B → A`).

#### `withCovers`: stand in for a check that couldn't reach its target

Use this when your check verifies the same thing as another check, but from a vantage point that can actually reach the target. Typically the covering check runs from inside the cluster, and the covered check dials from wherever wsm runs. For example, `externalMysqlInClusterCheck` dials MySQL from a probe pod, so it covers the local `externalMysqlCheck`:

```go
ExternalMysqlInClusterCheck: withCovers(typedCheck(ExternalMysqlInClusterCheck, "spec.mysql.*.externalMysql",
	TypeCluster, []ResourceReq{ReqResourceCreate}, RunExternalMysqlInClusterCheck), ExternalMysqlCheck),
```

After all checks have run, the runner turns a covered `FAIL` into `SKIPPED` only when all of these are true:

1. The covering check reported `PASS`.
2. Both results are for the same `FieldPath`, meaning the same instance (`spec.mysql.ext.externalMysql`). A pass on one instance never covers another instance.
3. The covered check reported `FAIL` with `Failure: FailureUnreachable`.

When that happens, the covered result's `CoveredBy` is set to the covering check's name, and its message gets the prefix `not reachable from this machine: `. wsm prints `covered by <check>` under it, and a footer saying the credentials weren't verified. Watchtower shows the same as "Covered by …; credentials were not verified."

Nothing else is collapsed. A `FailureRejected` result stays `FAIL`; that's a wrong password or a missing database, and it's wrong from anywhere. An `UNKNOWN` result stays `UNKNOWN`. If the covering check itself fails, comes back `UNKNOWN` (for example, the probe pod timed out), or is declined by the user, nothing is collapsed and the covered `FAIL` stands.

**What the covered check must do.** Covering only works if the covered check classifies its failures. Every `FAIL` it returns must set `Result.Failure`:

```go
if err := db.PingContext(ctx); err != nil {
	kind := FailureUnreachable // DNS, dial, timeout, untrusted cert: depends on where the check runs
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		kind = FailureRejected // the server answered and refused: wrong from anywhere
	}
	return Result{Check: ExternalMysqlCheck, Outcome: OutcomeFail, Failure: kind,
		Message: fmt.Sprintf("cannot connect to %s: %s", target, redact(err.Error(), password))}
}
```

Classify the error the server sends back. For example, use `*mysql.MySQLError` for go-sql-driver, or the `redis.Error` interface for go-redis (it's an interface, so declare `var rErr redis.Error`, not a pointer). Any other error is `FailureUnreachable`. Classify the error from the step that actually contacts the server (`Ping`, a handshake), not from a later optional step. If `Failure` is left empty, the result can never be covered.

**What the covering check must do.** It must use the same `CR field` as the covered check, so they produce matching `FieldPath`s. Its `PASS` message should say where it ran (for example, `reachable from a pod in namespace "wandb"`).

Rules, enforced by `TestRegistryRelations`:

- Every name must be a registered check.
- A check can't cover itself.
- Both checks must have the same `CRField`.
- Two checks can't cover each other.

Optional: to also gate reconcile on the check, call `gatePreflight` with the check's name from the matching file in `internal/controller/reconciler/` (see `redis.go` for an example).

## 3. Test it: `pkg/preflight/<name>_test.go`

Put the tests next to the check (for example `managed_kafka_storage_test.go`) in package `preflight`, using table-driven tests. Helpers already in the package:

- `literalResolver`: resolves literal values only.
- `secretRef(name, key)`: builds a Secret-backed `ValueOrSecret`.

```go
func TestRunManagedKafkaStorageCheck(t *testing.T) {
	tests := []struct {
		name string
		spec apiv2.ManagedKafkaSpec
		want Outcome
	}{
		{name: "unset uses default", spec: apiv2.ManagedKafkaSpec{}, want: OutcomePass},
		{name: "valid quantity", spec: apiv2.ManagedKafkaSpec{StorageSize: "10Gi"}, want: OutcomePass},
		{name: "invalid quantity", spec: apiv2.ManagedKafkaSpec{StorageSize: "ten gigs"}, want: OutcomeFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RunManagedKafkaStorageCheck(context.Background(), &tt.spec, Env{Resolve: literalResolver})
			if got.Outcome != tt.want {
				t.Errorf("got %s %q, want %s", got.Outcome, got.Message, tt.want)
			}
		})
	}
}
```

Cover at least the pass path and each fail path. For checks that resolve `ValueOrSecret` fields, also cover these cases:

- a resolver returning `ErrSecretUnavailable`, which should give `SKIPPED` (see `TestRunProxyCheck_SecretUnavailable`)
- no secret in `Message`

If your check declares relationships, or is the target of one:

- Add its expected `DependsOn` or `Covers` to `TestRegistryRelationEntries` in `relations_test.go`, so that dropping the wrapper by accident fails the test.
- If it can be covered, test that it sets `Failure` correctly. There should be one case for `FailureUnreachable` (a closed port from `closedPort(t)`) and one for `FailureRejected` (a fake server that answers with an error). See `TestRunExternalMysqlCheck_FailureKind` and `TestRunExternalRedisCheck_FailureKind`.
- How the runner orders, skips and collapses checks is tested in wsm (`pkg/preflight/relations_test.go`), not here.

For connection checks, use a local fake (`httptest.NewServer`, or a `net.Listen` on `127.0.0.1:0`) rather than real services. `TestRegistryConsistency` in `preflight_test.go` automatically validates the new registry entry's name, type and resource reqs.

Run the tests:

```bash
go test ./pkg/preflight/...
```

## Try it with wsm

To try the check from wsm, point a wsm checkout at your local operator clone, build it, and run preflight against a CR that sets the check's field (see [preflight-checks.md](preflight-checks.md)):

```bash
go mod edit -replace github.com/wandb/operator=../operator && make build && ./wsm preflight wandb.yaml
```

Drop the `replace` before committing in wsm: `go mod edit -dropreplace github.com/wandb/operator`.
