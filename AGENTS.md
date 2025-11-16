# Agent Guide: Wandb Operator

This document provides essential information for AI agents working in this Kubernetes operator codebase.

## Project Overview

This is a **Kubernetes Operator** for managing Weights & Biases (wandb) deployments, built using:
- **Kubebuilder v4** (Go-based operator framework)
- **controller-runtime v0.21.0**
- **Go 1.25.1**
- **Kubernetes API v0.33.3**
- **Operator Lifecycle Manager (OLM)** support
- **Multi-binary architecture** (controller + canary)

The operator manages:
- `WeightsAndBiases` custom resources (v1 and v2 APIs)
- `Application` custom resources (v2 API)
- Infrastructure components: MySQL, Redis, MinIO, Kafka, ClickHouse
- Helm chart deployments for wandb platform

## Essential Commands

### Development Setup
```bash
# Prerequisites: kind, tilt, kubebuilder, jq

# Create/manage kind cluster
./scripts/setup_kind.sh          # Creates cluster (uses tilt-settings.json)
./scripts/teardown_kind.sh       # Deletes cluster

# Configure Tilt
cp tilt-settings.sample.json tilt-settings.json
# Edit tilt-settings.json as needed

# Start development environment
tilt up                           # Opens Tilt UI on http://localhost:10350
```

### Build & Test
```bash
# Essential workflow
make manifests generate          # Generate CRDs and deepcopy code from API types
make fmt                         # Format code
make vet                         # Run go vet
make lint                        # Run golangci-lint
make test                        # Run tests + generate coverage.html

# Build binaries
make build                       # Build all binaries (controller + canary)
make build-controller            # Build controller only (bin/manager)
make build-canary               # Build canary only (bin/canary)

# Docker images
make docker-build                # Build all images
make docker-build-controller     # Build controller image
make docker-build-canary         # Build canary image
```

### Generate Code
```bash
make manifests                   # Generate CRDs, RBAC, webhooks from kubebuilder markers
make generate                    # Generate DeepCopy methods
go generate ./...                # Generate counterfeiter mocks
```

### Kubernetes Operations
```bash
make install                     # Install CRDs to cluster
make uninstall                   # Remove CRDs from cluster
make deploy                      # Deploy controller to cluster
make undeploy                    # Remove controller from cluster
make apply-local-dev             # Apply local webhook dev config
```

### Local Development
```bash
# Run controller locally (without webhooks)
make run

# Run controller locally WITH webhooks
make setup-local-webhook         # One-time setup
make run-local-webhook           # Run with webhook support
```

### Testing
```bash
make test                        # Unit tests + coverage report
make test-e2e                    # End-to-end tests (requires kind cluster)
```

### Dependency Management
```bash
make safe-bump-deps              # Safe dependency updates (go get -u)
make major-bump-deps             # Major version updates
make list-outdated               # List outdated direct dependencies
make find-deprecated             # Find deprecated dependencies
make check-vulnerabilities       # Run govulncheck
```

### OLM (Operator Lifecycle Manager)
```bash
make bundle                      # Generate OLM bundle
make bundle-build                # Build bundle image
```

## Project Structure

```
operator/
├── api/                         # CRD API definitions
│   ├── v1/                      # v1 API (WeightsAndBiases)
│   └── v2/                      # v2 API (WeightsAndBiases, Application)
├── cmd/                         # Binary entrypoints
│   ├── controller/              # Main operator controller
│   └── canary/                  # Connectivity tester
├── internal/                    # Internal packages (not importable)
│   ├── controller/              # Reconciler implementations
│   │   ├── infra/               # Infrastructure controllers (mysql, redis, etc.)
│   │   └── ctrlqueue/           # Controller queue helpers
│   ├── model/                   # Business logic (merge, defaults)
│   ├── webhook/                 # Webhook implementations
│   ├── vendored/                # Vendored API files
│   └── utils/                   # Internal utilities
├── pkg/                         # Public packages (importable)
│   ├── wandb/                   # Wandb-specific packages
│   ├── helm/                    # Helm integration
│   ├── redact/                  # Sensitive data masking
│   └── utils/                   # Generic utilities
├── config/                      # Kustomize configs
│   ├── crd/                     # CRD base + patches
│   ├── default/                 # Default deployment config
│   ├── local-dev/               # Local webhook development
│   ├── manager/                 # Controller manager
│   ├── rbac/                    # RBAC manifests
│   └── samples/                 # Sample CRs
├── test/                        # Test files
│   ├── e2e/                     # End-to-end tests
│   └── utils/                   # Test utilities
├── hack/                        # Scripts and utilities
│   └── testing-manifests/       # Test manifests (CRs, minio, canary)
├── scripts/                     # Helper scripts
├── docs/                        # Documentation
├── olm/                         # OLM-specific files
└── dist/                        # Generated files (created by Tilt)
```

## Code Organization

### API Packages (`api/v1`, `api/v2`)
- Define CRD types with kubebuilder markers
- Include `*_types.go` (API structs) and `zz_generated.deepcopy.go` (generated)
- Changes here require `make manifests generate`

### Internal Packages (`internal/`)
- `controller/`: Reconciler implementations
- `model/`: Business logic (defaults, merging, validation)
- `webhook/`: Admission webhook handlers
- `vendored/`: Vendored external API types

### Public Packages (`pkg/`)
- Reusable code that could be imported by other projects
- Domain-specific utilities (`wandb/`, `helm/`)
- Generic utilities (`utils/`, `redact/`)

### Config (`config/`)
- Kustomize-based configuration
- `crd/bases/`: Generated CRD YAML (from `make manifests`)
- `crd/patches/`: Manual patches applied via kustomize
- `default/`: Standard deployment
- `local-dev/`: Local webhook development setup

## Code Patterns & Conventions

### Controller Patterns

**Reconcile Structure:**
```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := ctrllog.FromContext(ctx)
    
    // Fetch resource
    resource := &appsv2.WeightsAndBiases{}
    if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
        return ctrlqueue.DoNotRequeue(), client.IgnoreNotFound(err)
    }
    
    // Handle finalizers
    if !resource.ObjectMeta.DeletionTimestamp.IsZero() {
        return r.handleDeletion(ctx, resource)
    }
    
    // Add finalizer if needed
    if !ctrlqueue.ContainsString(resource.Finalizers, resFinalizer) {
        resource.Finalizers = append(resource.Finalizers, resFinalizer)
        return ctrlqueue.Requeue(), r.Update(ctx, resource)
    }
    
    // Reconcile logic
    if err := r.reconcileComponents(ctx, resource); err != nil {
        return ctrlqueue.RequeueWithError(err)
    }
    
    return ctrlqueue.DoNotRequeue(), nil
}
```

**Return Helpers (from `ctrlqueue` package):**
- `ctrlqueue.DoNotRequeue()` - Success, no requeue
- `ctrlqueue.Requeue()` - Requeue immediately
- `ctrlqueue.RequeueWithError(err)` - Requeue with error
- `ctrlqueue.RequeueAfter(duration)` - Requeue after delay

**Logging:**
```go
log := ctrllog.FromContext(ctx)
log.Info("message", "key", value, "key2", value2)
log.Error(err, "operation failed", "resource", name)
```

**Event Recording:**
```go
r.Recorder.Event(resource, corev1.EventTypeNormal, "Reason", "Message")
```

**Event Filtering:**
```go
// Use predicates to filter reconciliation triggers
func filterWBEvents() predicate.Predicate {
    return predicate.GenerationChangedPredicate{}  // Only trigger on spec changes
}
```

### API Type Conventions

**Kubebuilder Markers:**
```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=wandb
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=.status.state
// +kubebuilder:validation:Required
// +kubebuilder:validation:Enum=small;medium;large
// +kubebuilder:default="small"
type WeightsAndBiases struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   WeightsAndBiasesSpec   `json:"spec,omitempty"`
    Status WeightsAndBiasesStatus `json:"status,omitempty"`
}
```

**JSON Tags:**
- All fields use `json:"fieldName,omitempty"`
- Embedded structs use `json:",inline"`

**Custom Types:**
```go
// Use custom types for enums with validation methods
type WBSize string
const (
    WBSizeSmall  WBSize = "small"
    WBSizeMedium WBSize = "medium"
    WBSizeLarge  WBSize = "large"
)
```

**State Management:**
```go
// Implement comparison methods for state precedence
func (s WBStateType) WorseThan(other WBStateType) bool {
    // Custom precedence logic
}
```

### Testing Patterns

**Framework: Ginkgo + Gomega**
```go
var _ = Describe("Component", func() {
    var (
        ctx    context.Context
        cancel context.CancelFunc
    )
    
    BeforeEach(func() {
        ctx, cancel = context.WithCancel(context.Background())
    })
    
    AfterEach(func() {
        cancel()
    })
    
    Context("when condition", func() {
        It("should do something", func() {
            Expect(result).To(Equal(expected))
            Expect(err).NotTo(HaveOccurred())
        })
    })
})
```

**Test Files:**
- Unit tests: `*_test.go` alongside source files
- Test suites: `*_suite_test.go` with `RunSpecs()`
- E2E tests: `test/e2e/`

**Mocking:**
```go
// Generate mocks with counterfeiter
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Interface
type Interface interface {
    Method() error
}

// Use in tests
fake := &fakes.FakeInterface{}
fake.MethodReturns(errors.New("test error"))
```

### Error Handling

**Named Errors:**
```go
var (
    ErrTypeNotMatch = errors.New("type does not match")
    ErrNotFound     = errors.New("resource not found")
)
```

**Error Checking:**
```go
if err := operation(); err != nil {
    if errors.IsNotFound(err) {
        // Handle not found
        return nil
    }
    return fmt.Errorf("operation failed: %w", err)
}
```

**Early Returns:**
- Return immediately on errors
- Use `client.IgnoreNotFound(err)` for optional resources

### Logging & Debugging

**Structured Logging:**
```go
log.Info("reconciling resource",
    "name", resource.Name,
    "namespace", resource.Namespace,
    "generation", resource.Generation)
```

**Sensitive Data:**
```go
// Use redaction for sensitive values
log.Info("config", "values", resource.Spec.SensitiveValuesMasked())
```

**Log Levels:**
- `.Info()` - Normal operations, state changes
- `.Error()` - Failures requiring attention
- Avoid excessive logging in hot paths

### Naming Conventions

**Files:**
- Controllers: `*_controller.go`
- Tests: `*_test.go`
- Test suites: `*_suite_test.go`
- Generated: `zz_generated.*.go`
- Fakes: `*fakes/fake_*.go`

**Variables:**
- Use descriptive names (avoid single-letter except in short scopes)
- Context: `ctx`
- Logger: `log`
- Errors: `err`
- Clients: `client`, `k8sClient`

**Constants:**
- Finalizers: `*Finalizer` (e.g., `resFinalizer = "finalizer.app.wandb.com"`)
- Event reasons: PascalCase (e.g., `"ComponentCreated"`)

## Critical Workflows

### When API Types Change

**The Problem:** Tilt does NOT automatically regenerate CRDs when `api/` types change.

**Manual Steps (Required):**
```bash
# 1. Regenerate CRDs and deepcopy
make manifests generate

# 2. Either:
#    a) Restart Tilt (recommended)
tilt down
tilt up

#    b) OR manually trigger in Tilt UI
#       Click "Regenerate-CRDs" trigger button

#    c) OR manually apply CRDs
kubectl apply -f config/crd/bases/apps.wandb.com_weightsandbiases.yaml
```

**What Gets Regenerated:**
- `config/crd/bases/*.yaml` - Base CRD YAML files
- `api/*/zz_generated.deepcopy.go` - DeepCopy methods
- `config/rbac/role.yaml` - RBAC from controller annotations

### Development Flow Reference

| What Changed | Auto-Detected? | Action Required |
|--------------|----------------|-----------------|
| `api/**/*_types.go` | ❌ No | `make manifests generate`, then restart Tilt or trigger `Regenerate-CRDs` |
| `internal/controller/**/*.go` | ✅ Yes | Tilt auto-rebuilds and restarts |
| `pkg/**/*.go` | ✅ Yes | Tilt auto-rebuilds and restarts |
| `cmd/**/*.go` | ✅ Yes | Tilt auto-rebuilds and restarts |
| `config/crd/patches/*.yaml` | ❌ No | Restart Tilt |
| `config/default/**/*.yaml` | ❌ No | Restart Tilt |
| `hack/testing-manifests/wandb/*.yaml` | ✅ Yes | Tilt auto-applies (via `Install dev CR` resource) |

### Tilt Resources

Tilt organizes work into labeled resources:

**"wandb" label:**
- `Watch&Rebuild` - Watches code, runs `make` + compiles binary
- `Regenerate-CRDs` - Manual trigger to regenerate CRDs from API types
- `Regenerate-RBAC` - Manual trigger to regenerate RBAC from annotations
- `Install dev CR` - Manual trigger to apply test CR
- `operator-controller-manager` - Controller deployment

**"infra" label:**
- `minio` - Local S3 storage
- `percona-mysql-op-*` - MySQL operator
- `redis-op-*` - Redis operator
- `kafka-op-*` - Kafka operator
- `minio-op-*` - MinIO operator
- `clickhouse-op-*` - ClickHouse operator

**"canary" label:**
- Connectivity tester resources (if `displayCanary: true`)

**"webhook" label:**
- `Setup-Local-Webhook-Certs` - Generate local webhook certificates

**"OLM" label:**
- OLM installation and management

### Testing Workflow

**Before Committing:**
```bash
# MUST run both (per CLAUDE.md)
make lint
make test

# Optional: view coverage
open coverage.html
```

**Test Types:**
- Unit tests: Fast, no cluster required
- Integration tests: Use envtest (fake API server)
- E2E tests: Require real cluster (kind)

**Running Specific Tests:**
```bash
# Unit tests only (excludes e2e)
make test

# E2E tests only (requires kind cluster)
make test-e2e
```

## Important Gotchas

### 1. CRD Generation is NOT Automatic
- Changing `api/*_types.go` requires manual `make manifests generate`
- Tilt will NOT detect these changes automatically
- See "When API Types Change" section above

### 2. Kustomize Patches
- CRDs in `config/crd/bases/` are generated and can be overwritten
- Customizations go in `config/crd/patches/`
- Example: `config/crd/patches/use_v2.yaml` controls API version
- Changes to patches require Tilt restart

### 3. Multi-Version API
- Both v1 and v2 APIs exist
- v2 is the storage version (`+kubebuilder:storageversion`)
- Test CRs specify version via `wandbCrName` in `tilt-settings.json`
  - `wandb-default-v1` - Uses v1 API
  - `wandb-dev-v2` - Uses v2 API

### 4. Finalizers
- Always use helper: `ctrlqueue.ContainsString(resource.Finalizers, finalizer)`
- Remove with: `controllerutil.RemoveFinalizer(resource, finalizer)`
- Don't forget to update resource after adding/removing

### 5. Owner References
- Set with: `controllerutil.SetOwnerReference(owner, dependent, r.Scheme)`
- Required for garbage collection
- Must be in same namespace

### 6. Tilt Settings
- `tilt-settings.json` is NOT checked into git
- Copy from `tilt-settings.sample.json`
- Required for Tilt to run
- Controls: cluster name, operator installation, infra components

### 7. Local Webhook Development
- Set `"localWebhookDev": true` in `tilt-settings.json`
- Run `make setup-local-webhook` once
- Controller runs locally, not in cluster
- Use `make run-local-webhook` to start controller

### 8. Test Coverage
- Always generates `coverage.html` when running `make test`
- Expected as part of testing workflow (per CLAUDE.md)

### 9. Dependencies
- Scripts check for required dependencies (jq, kind, etc.)
- Fail with clear error messages if missing
- No fallbacks unless explicitly requested (per CLAUDE.md)

### 10. Sensitive Data
- Never log passwords, tokens, or secrets
- Use `.SensitiveValuesMasked()` methods for safe logging
- Redaction utilities in `pkg/redact/`

## Code Style (from CLAUDE.md)

### Comments
- **DO NOT** add comments that restate what code does
- **ONLY** add comments explaining WHY, not WHAT
- Good: Explaining business logic, non-obvious behavior
- Bad: `# Check if cluster exists` before obvious conditional

**Examples:**

❌ **Bad:**
```go
// Check if resource exists
if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
```

✅ **Good:**
```go
if err := r.Get(ctx, req.NamespacedName, resource); err != nil {
```

✅ **Acceptable (when context needed):**
```go
// Use empty string fallback to handle missing kindClusterName key
KIND_CLUSTER_NAME=$(jq -r '.kindClusterName // empty' tilt-settings.json)
```

### Testing & Linting
- **ALWAYS** run `make lint` and `make test` before completing tasks
- Generate coverage reports with `make test` (produces `coverage.html`)
- No exceptions

### Dependencies
- Scripts should explicitly check for required dependencies
- Fail with clear error messages if missing
- Do NOT provide fallbacks unless specifically requested

## Common Tasks

### Add a New CRD Field
```bash
# 1. Edit API type
vim api/v2/weightsandbiases_types.go

# 2. Add field with kubebuilder markers
# 3. Regenerate
make manifests generate

# 4. Restart Tilt
tilt down && tilt up

# 5. Test
make test
```

### Add a New Controller
```bash
# 1. Create controller file
vim internal/controller/myresource_controller.go

# 2. Implement Reconcile() and SetupWithManager()

# 3. Register in main.go
vim cmd/controller/main.go

# 4. Add RBAC markers:
// +kubebuilder:rbac:groups=apps.wandb.com,resources=myresources,verbs=get;list;watch

# 5. Regenerate RBAC
make manifests

# 6. Test
make test
```

### Add Infrastructure Component
```bash
# 1. Create package under internal/controller/infra/
mkdir -p internal/controller/infra/mydb

# 2. Implement interface (typically Create, Update, Delete, GetActual)

# 3. Add to reconciler in internal/controller/

# 4. Add Tilt resource (if needed for dev)
vim Tiltfile

# 5. Test
make test
```

### Debug in Cluster
```bash
# 1. Start Tilt
tilt up

# 2. View logs in Tilt UI (http://localhost:10350)
# OR
kubectl logs -n operator-system deployment/operator-controller-manager -f

# 3. Check CR status
kubectl get wandb -o yaml

# 4. Check events
kubectl describe wandb <name>
```

### Debug Locally
```bash
# Without webhooks
make run

# With webhooks
make setup-local-webhook
make run-local-webhook

# In another terminal
kubectl apply -f hack/testing-manifests/wandb/wandb-dev-v2.yaml
kubectl get wandb -w
```

## CI/CD

### GitHub Actions
- `.github/workflows/run-tests.yaml` - Tests and build on PR/push to main
- `.github/workflows/release.yaml` - Release automation
- `.github/workflows/pr-title.yaml` - PR title validation
- `.github/workflows/docker-build-scan.yml` - Docker image scanning

### Release Process
- Uses semantic-release (`.releaserc.json`)
- Automated via GitHub Actions
- CHANGELOG.md is auto-generated

## Resources & Documentation

### Internal Documentation
- `README.md` - Basic setup and prerequisites
- `DEVELOPMENT.md` - Detailed development workflow and Tilt issues
- `MULTI_BINARY.md` - Multi-binary project structure
- `CLAUDE.md` - Code style guidelines for AI agents (THIS FILE IS LAW)
- `docs/config-api.md` - Configuration API documentation

### External Resources
- [Kubebuilder Book](https://book.kubebuilder.io/) - Operator patterns
- [controller-runtime](https://pkg.go.dev/sigs.k8s.io/controller-runtime) - Framework docs
- [Tilt Documentation](https://docs.tilt.dev/) - Local development

### Key Concepts
- **CRD** - Custom Resource Definition
- **CR** - Custom Resource (instance of CRD)
- **Reconciler** - Controller logic that brings actual state to desired state
- **Operator** - Kubernetes controller that manages custom resources
- **Kubebuilder** - Framework for building operators
- **Kustomize** - Kubernetes YAML templating tool
- **OLM** - Operator Lifecycle Manager (for operator distribution)
- **envtest** - Fake Kubernetes API server for testing

## Quick Reference

```bash
# Daily workflow
make manifests generate    # After API changes
make fmt                  # Format code
make lint                 # Check linting
make test                 # Run tests
tilt up                   # Start dev environment

# Troubleshooting
tilt down                 # Stop Tilt
rm -rf dist/ tilt_bin/    # Clean generated files
make manifests generate   # Regenerate everything
tilt up                   # Restart

# Testing
make test                 # Unit tests
open coverage.html        # View coverage
make test-e2e             # E2E tests

# Release
git commit -m "feat: new feature"  # Conventional commits
git push                           # CI runs tests
# Merge to main triggers release
```

## Agent-Specific Notes

### Before Making Changes
1. Check if `api/` types need updates → requires `make manifests generate`
2. Check if controller RBAC markers changed → requires `make manifests`
3. Always run `make lint` and `make test` before completing

### When Testing Changes
1. If API changed: Restart Tilt (CRDs won't auto-update)
2. If controller changed: Tilt auto-rebuilds (no restart needed)
3. Always verify with `kubectl get <resource> -o yaml`

### Common Mistakes to Avoid
- ❌ Forgetting to run `make manifests generate` after API changes
- ❌ Not restarting Tilt after CRD patches change
- ❌ Adding obvious comments (violates CLAUDE.md)
- ❌ Not running `make lint` and `make test` before finishing
- ❌ Logging sensitive data without redaction
- ❌ Using single-letter variable names unnecessarily

### Success Checklist
- ✅ Code formatted (`make fmt`)
- ✅ Linting passes (`make lint`)
- ✅ Tests pass (`make test`)
- ✅ If API changed: `make manifests generate` run
- ✅ If testing in cluster: Tilt shows green
- ✅ No obvious/redundant comments added
- ✅ Error handling follows patterns
- ✅ Logging is structured and masks sensitive data
