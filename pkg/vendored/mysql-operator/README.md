# MySQL Operator API Types (v2)

This directory contains Go type definitions for interacting with Oracle MySQL Operator v2 Custom Resources.

## Generation Process

These types were generated to enable the W&B operator to manage MySQL InnoDB clusters via the Oracle MySQL Operator.

### Source

- **MySQL Operator Version**: 8.0.44-2.0.20
- **API Group**: `mysql.oracle.com`
- **API Version**: `v2`
- **Date Vendored**: 2026-01-20

### Source CRDs

- **MySQL Operator CRDs**: `https://github.com/mysql/mysql-operator/blob/8.0.44-2.0.20/deploy/charts/mysql-operator/templates/deploy-crds.yaml`

### Generation Steps

1. **Downloaded CRD YAML files** from MySQL Operator 8.0.44-2.0.20
2. **Created type definitions** based on OpenAPI v3 schemas in the CRDs
3. **Generated DeepCopy methods** via `controller-gen`
4. **Registered** with operator scheme in `cmd/main.go`

## What Was Vendored

We vendored the API type definitions needed for our operator:

- `v2/` - InnoDBCluster and MySQLBackup CRD types
  - `groupversion_info.go` - API group registration and scheme builder
  - `innodbcluster_types.go` - InnoDBCluster CRD type definitions
  - `mysqlbackup_types.go` - MySQLBackup CRD type definitions
  - `zz_generated.deepcopy.go` - Generated DeepCopy methods

### Removed Content
- All test files
- Internal operator logic and controllers

## License

The vendored code maintains its original Universal Permissive License (UPL) 1.0 from the Oracle MySQL Operator project.

## Updates

This vendored copy should be reviewed when upgrading to newer versions of the MySQL operator to determine if any important changes or fixes need to be incorporated.
