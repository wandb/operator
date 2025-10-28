# Altinity ClickHouse Operator - Vendored API

This directory contains vendored CRD API types from the Altinity ClickHouse Operator.

## Source

- **Repository**: https://github.com/Altinity/clickhouse-operator
- **Version**: master (commit from 2025-10-28)
- **Date Vendored**: 2025-10-28

## What Was Vendored

The following directories from `pkg/apis/` were copied:
- `clickhouse.altinity.com/v1` - ClickHouse Installation CRD types
- `common/types` - Common type definitions
- `deployment` - Deployment-related types
- `metrics` - Metrics types
- `swversion` - Software version types

The `clickhouse-keeper.altinity.com` API was excluded as it is not used by this project.

## Changes Made

### Import Path 
All imports were updated from:
```go
github.com/altinity/clickhouse-operator/pkg/apis/...
```
to:
```go
github.com/wandb/operator/api/altinity-clickhouse-vendored/...
```

### Mutex Handling in DeepCopy Methods
The generated `zz_generated.deepcopy.go` file had multiple issues with copying sync.Mutex and sync.RWMutex fields, which Go does not allow. The following changes were made:

1. **Removed shallow copies** that would copy mutexes:
   - Removed `*out = *in` lines from DeepCopyInto methods for types containing mutexes
   - Added explicit field-by-field copying for value types
   - Left mutex fields uncopied (they remain zero-valued)

2. **Types modified**:
   - `ClickHouseInstallation`
   - `ClickHouseInstallationRuntime`
   - `ClickHouseInstallationTemplate`
   - `ClickHouseOperatorConfiguration`
   - `OperatorConfig`
   - `OperatorConfigCHI`
   - `OperatorConfigCHIRuntime`
   - `OperatorConfigTemplate`
   - `Status`

### Struct Tag Fixes
Fixed malformed struct tags in `type_configuration_chop.go`:
- `OperatorConfigWatchNamespaces.Include` - Added missing closing backtick
- `OperatorConfigWatchNamespaces.Exclude` - Added missing closing backtick

### Unexported Field Fixes
Removed json/yaml tags from unexported fields in `type_template_indexes.go`:
- `HostTemplatesIndex.templates`
- `PodTemplatesIndex.templates`
- `VolumeClaimTemplatesIndex.templates`
- `ServiceTemplatesIndex.templates`

Go does not allow json tags on unexported fields.

### Commented Out Unused Code
Using the `/** UNUSED CODE ... */` pattern:

1. **MergeFrom method** in `type_configuration_chop.go`:
   - Function uses `mergo.Merge()` which attempts to copy mutex fields
   - Not used by wandb/operator controller code
   - Commented out the function and the mergo import

2. **Test files**: All `*_test.go` files were removed as they are not needed in production.

## Types Used by wandb/operator

The wandb/operator project uses only a subset of the vendored types:

### Core Types
- `ClickHouseInstallation` - Main CRD for ClickHouse installations
- `ChiSpec` - Specification for ClickHouseInstallation
- `Configuration` - Configuration section with clusters and users
- `Cluster` - Cluster definition
- `ChiClusterLayout` - Layout with shard and replica counts
- `Defaults` - Default settings
- `TemplatesList` - List of template references
- `Templates` - Templates for pods, volumes, etc.
- `VolumeClaimTemplate` - Volume claim template definition

### Settings Types
- `Settings` - User settings container
- `SettingScalar` - Scalar setting value
- Functions: `NewSettings()`, `NewSettingScalar()`

### Status
- `Status` - Installation status (used via `ClickHouseInstallation.Status`)
- Constant: `StatusCompleted` - Status value indicating completed installation

### Unused Types
The following types are defined but not used by wandb/operator:
- `ClickHouseInstallationTemplate`
- `ClickHouseOperatorConfiguration`
- All `OperatorConfig*` types
- ClickHouse Keeper types (entire `clickhouse-keeper.altinity.com` API)

## Why Vendored

The ClickHouse operator API was vendored for the following reasons:

1. **Dependency Management**: The upstream operator has many internal package dependencies (pkg/util, pkg/version, etc.) that are tightly coupled to the operator's runtime behavior. Vendoring allows us to use just the CRD type definitions without pulling in the entire operator codebase.

2. **Mutex DeepCopy Issues**: The upstream generated code has issues with mutex copying in DeepCopy methods that cause compilation errors. These needed to be fixed locally.

3. **Code Quality Issues**: The upstream code has several quality issues (malformed struct tags, unexported fields with json tags) that needed correction.

4. **Version Stability**: Vendoring ensures we have a stable set of types that won't change unexpectedly with upstream updates.

5. **Minimal Footprint**: By vendoring, we can remove test files and unused code, reducing the codebase size.

## License

The vendored code is licensed under the Apache License 2.0, as indicated in the source file headers. The original license is maintained in all vendored files.

## Updating

If this vendored code needs to be updated in the future:

1. Download the latest `pkg/apis/` directory from the ClickHouse operator repository
2. Update import paths from `github.com/altinity/clickhouse-operator/pkg/apis/` to the vendored path
3. Remove test files: `find . -name "*_test.go" -delete`
4. Fix mutex DeepCopy issues in `zz_generated.deepcopy.go` (comment out `*out = *in` lines and handle fields individually)
5. Fix any malformed struct tags or unexported field issues
6. Comment out any functions that cause mutex copying errors (like `MergeFrom`)
7. Run `make build` to verify compilation
8. Update this README with the new version and any additional changes
