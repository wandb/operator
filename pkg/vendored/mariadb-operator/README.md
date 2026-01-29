# MariaDB Operator API Vendored Code

This directory contains vendored API types from the [MariaDB Operator](https://github.com/mariadb-operator/mariadb-operator) project.

## Source

- **Repository**: https://github.com/mariadb-operator/mariadb-operator
- **Version**: v25.10.4
- **Date Vendored**: 2026-01-26

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates.
This allows the W&B operator to manage MariaDB Operator while controlling when and how we adopt upstream changes.

## Changes Made

### Added

- `GaleraState` and `Bootstrap` types from `mariadb-operator/pkg/galera/recovery/recovery.go` added to `non-api-types.go`
- Updated `v1alpha1/mariadb_galera_types.go` to use those types and avoid the upstream dependancy

### Removed Content

- All test files (`*_test.go`)
- All functions and methods from API types to minimize dependencies and logic in vendored code.
- All unused imports.

Run `make generate` after changes applied