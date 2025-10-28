# Percona XtraDB Cluster Operator API Vendored Code

This directory contains vendored API types from the [Percona XtraDB Cluster Operator](https://github.com/percona/percona-xtradb-cluster-operator) project.

## Source

- **Repository**: https://github.com/percona/percona-xtradb-cluster-operator
- **Version**: v1.18.0
- **Date Vendored**: 2025-10-28

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates. This allows the W&B operator to manage Percona XtraDB clusters while controlling when and how we adopt upstream changes.

## Changes Made

### api/percona-operator-vendored/pxc/v1/vendored_helpers.go
- **Created new file**: Added vendored helper types and functions to replace dependencies on internal Percona packages:
  - `Platform` type and constants (from `pkg/version`)
  - PMM user constants: `PMMServer`, `PMMServerKey`, `PMMServerToken` (from `pkg/pxc/users`)
  - `MergeEnvLists` function (simplified version from `pkg/util`)

### api/percona-operator-vendored/pxc/v1/pxc_types.go
- **Lines 25-28**: Commented out imports to internal Percona packages:
  - `github.com/percona/percona-xtradb-cluster-operator/pkg/pxc/users`
  - `github.com/percona/percona-xtradb-cluster-operator/pkg/util`
  - `github.com/percona/percona-xtradb-cluster-operator/pkg/version`
- **Line 33**: Changed `version.Platform` to `Platform`
- **Lines 911-1384**: Commented out `CheckNSetDefaults` method that depends on `version.ServerVersion` (not used by W&B operator)
- **Line 19-20**: Removed unused `resource` import (only used in commented method)
- Replaced all references:
  - `users.PMMServer` → `PMMServer`
  - `users.PMMServerKey` → `PMMServerKey`
  - `util.MergeEnvLists` → `MergeEnvLists`
  - `version.PlatformOpenshift` → `PlatformOpenshift`
  - `version.PlatformKubernetes` → `PlatformKubernetes`

## What Was Vendored

We vendored the API type definitions needed for our operator:

- `pxc/v1/` - PerconaXtraDBCluster CRD types, backup types, and restore types
  - `register.go` - API group registration and scheme builder
  - `doc.go` - Package documentation
  - `pxc_types.go` - Main PerconaXtraDBCluster type definitions
  - `pxc_backup_types.go` - Backup-related types
  - `pxc_prestore_types.go` - Restore-related types
  - `zz_generated.deepcopy.go` - Generated DeepCopy methods
  - `vendored_helpers.go` - Helper types and functions (new file)

## License

The vendored code maintains its original Apache 2.0 license from the Percona XtraDB Cluster Operator project.

## Removal

This vendored copy should be reviewed when upgrading to newer versions of the Percona operator to determine if any important changes or fixes need to be incorporated.
