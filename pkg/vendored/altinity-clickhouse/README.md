# Altinity ClickHouse Operator API Vendored Code

This directory contains vendored API types from the [Altinity ClickHouse Operator](https://github.com/Altinity/clickhouse-operator) project.

## Source

- **Repository**: https://github.com/Altinity/clickhouse-operator
- **Version**: release-0.26.3
- **Date Vendored**: 2026-04-23

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates. This allows the W&B operator to manage ClickHouse installations while controlling when and how we adopt upstream changes.

Additionally, the upstream operator has several code generation issues that prevent direct use:
- DeepCopy methods contain mutex copying errors that fail go vet
- Many interface types that are incompatible with controller-gen
- Complex dependencies on internal packages

## Changes Made

### No Code Generation or Annotations

**Important**: Unlike our own API types (api/v1, api/v2), we do NOT run controller-gen on ANY vendored code. The vendored types and their DeepCopy methods are used as-is from upstream. This means:

- No code generation annotations needed (`+kubebuilder:*`, `+k8s:*`, `+genclient`)
- DeepCopy methods already exist in `zz_generated.deepcopy.go` from upstream
- We only fix compilation errors, not regenerate code

All code generation annotations have been removed:
- `+k8s:*` annotations (e.g., `+k8s:deepcopy-gen:*`, `+k8s:openapi-gen=true`)

Files affected:
- `clickhouse.altinity.com/v1/doc.go`
- `clickhouse.altinity.com/v1/types.go`
- `clickhouse.altinity.com/v1/api_resources.go`

The Makefile's `generate` target only processes `./api/v1` and `./api/v2`, explicitly excluding all vendored APIs:
- `pkg/vendored/altinity-clickhouse/...`
- `pkg/vendored/percona-operator/...`
- `pkg/vendored/minio-operator/...`
- `pkg/vendored/redis-operator/...`
- `pkg/vendored/strimzi-kafka/...`

### DeepCopy Mutex Fixes

The following DeepCopyInto methods in `clickhouse.altinity.com/v1/zz_generated.deepcopy.go` were fixed to avoid copying sync.Mutex and sync.RWMutex:

- **ClickHouseInstallation** (lines 571-589): Removed `*out = *in` shallow copy, commented out mutex field copies
- **ClickHouseInstallationRuntime** (lines 643-660): Removed `*out = *in` shallow copy, commented out `commonConfigMutex` copy
- **ClickHouseInstallationTemplate** (lines 673-691): Removed `*out = *in` shallow copy, commented out mutex field copies
- **ClickHouseOperatorConfiguration** (lines 745-751): Removed `*out = *in` shallow copy
- **OperatorConfig** (lines 1456-1502): Removed `*out = *in` shallow copy
- **OperatorConfigCHI** (lines 1585-1589): Removed `*out = *in` shallow copy
- **OperatorConfigCHIRuntime** (lines 1602-1624): Removed `*out = *in` shallow copy, commented out `mutex` field copy
- **OperatorConfigTemplate** (lines 2034-2038): Removed `*out = *in` shallow copy
- **Status** (lines 2656-2747): Removed `*out = *in` shallow copy, explicitly copied all non-mutex fields, commented out `mu` field copy

The same class of fixes was applied to
`clickhouse-keeper.altinity.com/v1/zz_generated.deepcopy.go` (commented out the
`*out = *in` shallow copy and the mutex field copies, explicitly copying the
non-mutex scalar fields for `Status`):

- **ClickHouseKeeperInstallation** — commented out the shallow copy and the
  `statusCreatorMutex` / `runtimeCreatorMutex` copies
- **ClickHouseKeeperInstallationRuntime** — commented out the shallow copy and
  the `commonConfigMutex` copy
- **Status** — replaced the shallow copy with explicit non-mutex field copies and
  commented out the `mu` (sync.RWMutex) copy

### Mutex Copy in MergeFrom

- **clickhouse.altinity.com/v1/type_configuration_chop.go** (line 816): Changed `mergo.Merge(c, *from, ...)` to `mergo.Merge(c, from, ...)` to pass pointer instead of dereferencing (which would copy mutexes)

### Struct Tag Fixes

- **clickhouse.altinity.com/v1/type_configuration_chop.go** (lines 194-195): Fixed missing closing quotes in struct tags for `OperatorConfigWatchNamespaces` fields

### Generic Type Format String Fix

- **util/map.go** (line 248): Format specifier is `%v` to support any comparable type in generic map printing function (already present in upstream 0.26.3)

### Unexported Field JSON Tag Fixes

- **clickhouse.altinity.com/v1/type_template_indexes.go**: Removed json/yaml tags from unexported `templates` fields in:
  - `HostTemplatesIndex` (line 20)
  - `PodTemplatesIndex` (line 74)
  - `VolumeClaimTemplatesIndex` (line 128)
  - `ServiceTemplatesIndex` (line 182)

### Import Path Updates

All import paths have been updated from `github.com/altinity/clickhouse-operator/pkg/` to the vendored paths:
- `github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1`
- `github.com/wandb/operator/pkg/vendored/altinity-clickhouse/common/`
- `github.com/wandb/operator/pkg/vendored/altinity-clickhouse/deployment/`
- `github.com/wandb/operator/pkg/vendored/altinity-clickhouse/metrics/`
- `github.com/wandb/operator/pkg/vendored/altinity-clickhouse/swversion/`
- `github.com/wandb/operator/pkg/vendored/altinity-clickhouse/util/`
- `github.com/wandb/operator/pkg/vendored/altinity-clickhouse/version/`
- `github.com/wandb/operator/pkg/vendored/altinity-clickhouse/xml/`

## What Was Vendored

We vendored the API type definitions and utility packages needed for our operator:

### CRDs

CRD files are located in `pkg/vendored/altinity-clickhouse/crds/`:
- `clickhouse-operator-install-bundle.yaml`
- `clickhouse.altinity.com_clickhouseinstallations.yaml` (individual CRD)

**Purpose**: Integration testing with real Kubernetes API server (envtest)

### API Types
- `clickhouse.altinity.com/v1/` - Main ClickHouseInstallation CRD types
  - All type definition files (`type_*.go`)
  - Generated DeepCopy methods (`zz_generated.deepcopy.go`)
  - API registration and scheme builder
  - Configuration helpers
- `clickhouse-keeper.altinity.com/v1/` - ClickHouseKeeperInstallation (CHK) CRD
  types, used to provision the ClickHouse Keeper ensemble that backs
  ReplicatedMergeTree replication. Same upstream version (release-0.26.3); reuses
  many shared types from `clickhouse.altinity.com/v1`.
  - `api_group.go` parent package holding `APIGroupName`
    (`clickhouse-keeper.altinity.com`)
  - Same import-path rewrites as the CHI package (see "Import Path Updates")
  - Same DeepCopy mutex fixes applied (see below)

### Supporting Packages
- `common/` - Common types and constants shared across the operator
- `deployment/` - Deployment-related types and utilities
- `metrics/` - Metrics collection types and helpers
- `swversion/` - Software version handling and parsing
- `util/` - Complete utility package with helper functions
- `version/` - Version information and utilities
- `xml/` - XML configuration handling

### Removed Content
- All test files (`*_test.go`)
- Benchmark files (`*_bench_test.go`)
- Test data directories

## Known Issues

None. All go vet errors and warnings from the vendored code have been fixed.

## License

The vendored code maintains its original Apache 2.0 license from the Altinity ClickHouse Operator project.

## Removal

This vendored copy should be reviewed when upgrading to newer versions of the ClickHouse operator. Note that any new version will likely require similar mutex copying fixes in the DeepCopy methods, as this is a pattern throughout the upstream codebase.
