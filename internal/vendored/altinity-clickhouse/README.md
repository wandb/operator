# Altinity ClickHouse Operator API Vendored Code

This directory contains vendored API types from the [Altinity ClickHouse Operator](https://github.com/Altinity/clickhouse-operator) project.

## Source

- **Repository**: https://github.com/Altinity/clickhouse-operator
- **Version**: v0.0.0-20251007061817-0cf33cf23815 (pseudo-version from Oct 7, 2024)
- **Date Vendored**: 2025-10-28

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates. This allows the W&B operator to manage ClickHouse installations while controlling when and how we adopt upstream changes.

Additionally, the upstream operator has several code generation issues that prevent direct use:
- DeepCopy methods contain mutex copying errors that fail go vet
- Many interface types that are incompatible with controller-gen
- Complex dependencies on internal packages

## Changes Made

### No Code Generation or Annotations

**Important**: Unlike our own API types (api/v1, api/v2), we do NOT run controller-gen on ANY vendored code. The vendored types and their DeepCopy methods are used as-is from upstream. This means:

- No code generation annotations needed (e.g., `+kubebuilder:*`, `+k8s:*`)
- DeepCopy methods already exist in `zz_generated.deepcopy.go` from upstream
- We only fix compilation errors, not regenerate code

All code generation annotations have been removed:
- `+k8s:*` annotations (e.g., `+k8s:deepcopy-gen:*`, `+k8s:openapi-gen=true`)

Files affected:
- `clickhouse.altinity.com/v1/doc.go`
- `clickhouse.altinity.com/v1/types.go`
- `clickhouse.altinity.com/v1/api_resources.go`

The Makefile's `generate` target only processes `./api/v1` and `./api/v2`, explicitly excluding all vendored APIs:
- `internal/vendored/altinity-clickhouse/...`
- `internal/vendored/percona-operator/...`
- `internal/vendored/minio-operator/...`
- `internal/vendored/redis-operator/...`
- `internal/vendored/strimzi-kafka/...`

### DeepCopy Mutex Fixes

The following DeepCopyInto methods in `clickhouse.altinity.com/v1/zz_generated.deepcopy.go` were fixed to avoid copying sync.Mutex and sync.RWMutex:

- **ClickHouseInstallation** (lines 518-537): Removed `*out = *in` shallow copy, commented out mutex field copies
- **ClickHouseInstallationRuntime** (lines 591-608): Removed `*out = *in` shallow copy, commented out `commonConfigMutex` copy
- **ClickHouseInstallationTemplate** (lines 621-639): Removed `*out = *in` shallow copy, commented out mutex field copies
- **ClickHouseOperatorConfiguration** (lines 695-701): Removed `*out = *in` shallow copy
- **OperatorConfig** (lines 1402-1447): Removed `*out = *in` shallow copy
- **OperatorConfigCHI** (lines 1531-1535): Removed `*out = *in` shallow copy
- **OperatorConfigCHIRuntime** (lines 1548-1570): Removed `*out = *in` shallow copy, commented out `mutex` field copy
- **OperatorConfigTemplate** (lines 1981-1985): Removed `*out = *in` shallow copy
- **Status** (lines 2603-2700): Removed `*out = *in` shallow copy, explicitly copied all non-mutex fields, commented out `mu` field copy

### Mutex Copy in MergeFrom

- **clickhouse.altinity.com/v1/type_configuration_chop.go** (line 792): Changed `mergo.Merge(c, *from, ...)` to `mergo.Merge(c, from, ...)` to pass pointer instead of dereferencing (which would copy mutexes)

### Struct Tag Fixes

- **clickhouse.altinity.com/v1/type_configuration_chop.go** (lines 176-177): Fixed missing closing quotes in struct tags for `OperatorConfigWatchNamespaces` fields

### Generic Type Format String Fix

- **util/map.go** (line 248): Changed format specifier from `%s` to `%v` to support any comparable type in generic map printing function

### Unexported Field JSON Tag Fixes

- **clickhouse.altinity.com/v1/type_template_indexes.go**: Removed json/yaml tags from unexported `templates` fields in:
  - `HostTemplatesIndex` (line 21)
  - `PodTemplatesIndex` (line 76)
  - `VolumeClaimTemplatesIndex` (line 131)
  - `ServiceTemplatesIndex` (line 186)

### Import Path Updates

All import paths have been updated from `github.com/altinity/clickhouse-operator/pkg/` to the vendored paths:
- `github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1`
- `github.com/wandb/operator/internal/vendored/altinity-clickhouse/common/`
- `github.com/wandb/operator/internal/vendored/altinity-clickhouse/deployment/`
- `github.com/wandb/operator/internal/vendored/altinity-clickhouse/metrics/`
- `github.com/wandb/operator/internal/vendored/altinity-clickhouse/swversion/`
- `github.com/wandb/operator/internal/vendored/altinity-clickhouse/util/`
- `github.com/wandb/operator/internal/vendored/altinity-clickhouse/version/`
- `github.com/wandb/operator/internal/vendored/altinity-clickhouse/xml/`

## What Was Vendored

We vendored the API type definitions and utility packages needed for our operator:

### API Types
- `clickhouse.altinity.com/v1/` - Main ClickHouseInstallation CRD types
  - All type definition files (`type_*.go`)
  - Generated DeepCopy methods (`zz_generated.deepcopy.go`)
  - API registration and scheme builder
  - Configuration helpers

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
