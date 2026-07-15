# CLAUDE.md

Guidance for AI agents working in this repository. Read this before making changes.

## What this is

The **Weights & Biases Kubernetes operator** (`github.com/wandb/operator`) — a
Kubebuilder/controller-runtime operator that installs and reconciles a W&B
deployment plus its backing infrastructure (MySQL, Redis, ClickHouse, Kafka,
object storage) on a cluster. Built with Go and the Operator SDK; deployed via
Helm/Kustomize; developed locally with Kind + Tilt.

## Architecture map

| Path | What lives here |
|------|-----------------|
| `api/v1/`, `api/v2/` | CRD Go types (`*_types.go`). **`v2` is the storage/hub version; `v1` is a conversion spoke.** |
| `cmd/manager/` | Operator manager entrypoint (`main.go`). |
| `cmd/crd-installer/` | Standalone job that installs/updates CRDs (embeds CRDs from `internal/crdinstaller/crds`). |
| `internal/controller/` | Reconcilers. `weightsandbiases_controller.go` and `application_controller.go` are the top-level controllers. |
| `internal/controller/reconciler/` | The v2 reconcile logic, split by concern (`mysql.go`, `redis.go`, `clickhouse.go`, `kafka.go`, `objectstore.go`, `ingress.go`, `gateway.go`, `telemetry_*.go`, `reconcile_v2.go`, …). |
| `internal/controller/infra/managed/` vs `infra/external/` | Each backing service can be **managed** (operator provisions it) or **external** (user supplies a connection). This managed/external split is a core concept. |
| `internal/controller/common/` | Shared reconcile helpers — conditions, labels, retention, detach, state. |
| `internal/webhook/v2/` | Defaulting + validation + conversion webhooks for the CRDs. |
| `pkg/wandb/` | W&B server spec/manifest/status handling (manifest source resolution, channels). |
| `pkg/vendored/` | **Generated/vendored** third-party operator types & CRDs (Moco, Strimzi-Kafka, SeaweedFS, Altinity-ClickHouse, Argo-Rollouts, Redis, Gateway API). Do not hand-edit. |
| `config/` | Kustomize bases, CRD bases/patches, RBAC, manager config. |
| `hack/testing-manifests/` | Local-dev/test fixtures: sample `WeightsAndBiases` CRs (`wandb/`), a Helm chart for external infra (`test-infra/`), cert-manager/seaweedfs/telemetry manifests, and versioned local server manifests (`server-manifest/`). Used by Tilt and tests, not shipped. |
| `docs/design/wandb_v2/` | Design docs for the v2 architecture and infra reconciliation. |

## The server manifest

A **server manifest** is the W&B-published description of a given W&B server
version: which applications to run, their infra requirements (bucket,
clickhouse, kafka, mysql, redis), generated secrets, DB migrations, feature
flags, and shared env-var/volume groups (see `Manifest` in
[pkg/wandb/manifest/manifest.go](pkg/wandb/manifest/manifest.go)).

- The CR points at one via `spec.wandb.manifestRepository` + `spec.wandb.version`.
  The reconciler resolves it through `manifest.GetServerManifest(...)` — an OCI
  artifact pulled with ORAS, or a `file://` path for local manifests.
- **It drives reconciliation.** [reconcile_v2.go](internal/controller/reconciler/reconcile_v2.go)
  loads the manifest, then uses it to apply infra sizing, generate secrets,
  create Kafka topics, run the MySQL init job and migrations, and reconcile each
  application. The CRD describes *desired W&B/infra shape*; the manifest supplies
  the *version-specific contents* the reconciler fills that shape with.
- **Published manifests are generated upstream** from
  [wandb/core `onprem/server-manifest`](https://github.com/wandb/core/tree/master/onprem/server-manifest)
  and published as OCI artifacts — they are **not** authored in this repo.
- For local work, `hack/testing-manifests/server-manifest/<version>/` holds
  checked-in copies (e.g. `0.79.0`). Set `manifestSource="local"` in
  `tilt-settings.star` (with a matching `wandbVersion`) to reconcile against them
  instead of the published repository.

## Critical workflow rules

- **API types are the source of truth, but generated artifacts must be kept in
  sync by hand.** After editing any `api/**/*_types.go`, run:
  ```bash
  make manifests generate sync-crd-embed
  ```
  This regenerates deepcopy methods, the CRD YAML in `config/crd/bases/`, and the
  embedded CRDs used by the crd-installer. CI fails if these are out of date.
- **Tilt does NOT regenerate CRDs automatically** when API types change — it only
  recompiles the controller binary. See [DEVELOPMENT.md](DEVELOPMENT.md) for the
  full "what to run when X changes" matrix. When in doubt, restart Tilt.
- **Don't edit generated files**: `zz_generated.deepcopy.go`, `config/crd/bases/*`,
  `internal/crdinstaller/crds/*`, and anything under `pkg/vendored/`. Edit the
  source and regenerate.
- **Counterfeiter fakes**: regenerate with `go generate ./...` after changing a
  mocked interface.

## Build / test / lint

- `make test` — runs `manifests generate sync-crd-embed vet` + unit tests via
  envtest (downloads binaries on first run); produces `coverage.html`.
- `make lint` — golangci-lint. `make lint-fix` to autofix.
- `make build` — regenerates, vets, and builds the manager + crd-installer binaries.
- **Always run `make lint` and `make test` before considering a task complete.**
- Tests use **Ginkgo/Gomega** (`suite_test.go` files set up envtest). CI runs
  `make test-coverage` and `make build` on Go 1.25.

## Commit & PR conventions

- PR titles (and the squash commit) **must follow Conventional Commits** — a CI
  check enforces it. Allowed types: `fix`, `feat`, `docs`, `style`, `refactor`,
  `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- The subject **must start with an uppercase letter** (e.g.
  `feat: Add retention policy validation`).
- Releases are automated via **semantic-release** off `main` (`feat` → minor,
  `fix` → patch, `!`/`BREAKING CHANGE` → major). `CHANGELOG.md` is generated — do
  not edit it by hand.

## Code style

### Comments
- **Do NOT add inline comments that simply restate what the code does.**
- Only comment to explain WHY, not WHAT — business logic, non-obvious behavior,
  important context.
- **Keep comments concise.** Prefer a short one-liner; only expand to multiple
  lines when the context genuinely can't be conveyed briefly.

```go
// Bad
// increment the counter
count++

// Good — explains a non-obvious constraint
// apiserver bounces v2 → v1 → v2 on admission; stash raw values so the round-trip is lossless
```

### Dependencies
- Scripts should explicitly check for required tools (e.g. `jq`) and fail with a
  clear message if missing. Don't add fallbacks for missing dependencies unless
  asked.

## Where to learn more

- [README.md](README.md) — local dev setup (Kind, Tilt, Kubebuilder, Kustomize).
- [DEVELOPMENT.md](DEVELOPMENT.md) — the regenerate-on-change flow and common issues.
- [docs/config-api.md](docs/config-api.md), [docs/infra-connection-settings.md](docs/infra-connection-settings.md),
  [docs/monitoring.md](docs/monitoring.md) — CR config, external infra, telemetry.
