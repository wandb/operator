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
