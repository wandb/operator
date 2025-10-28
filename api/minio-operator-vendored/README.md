# MinIO Operator API Vendored Code

This directory contains vendored API types from the [MinIO Operator](https://github.com/minio/operator) project.

## Source

- **Repository**: https://github.com/minio/operator
- **Version**: v0.0.0-20251009055539-34dc39801903 (pseudo-version from Oct 9, 2024)
- **Date Vendored**: 2025-10-28

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates. This allows the W&B operator to manage MinIO tenants while controlling when and how we adopt upstream changes.

## Changes Made

### api/minio-operator-vendored/minio.min.io/v2/vendored_helpers.go
- **Created new file**: Added vendored helper constants to replace dependencies on internal MinIO packages:
  - Certificate file name constants: `PublicCertFile`, `PrivateKeyFile`, `CAPublicCertFile` (from `pkg/certs`)
  - `GroupName` constant (from parent `pkg/apis/minio.min.io`)

### api/minio-operator-vendored/minio.min.io/v2/helper.go
- **Line 40**: Commented out import to internal MinIO package:
  - `github.com/minio/operator/pkg/certs`
- Replaced all references:
  - `certs.CAPublicCertFile` → `CAPublicCertFile`
  - `certs.PublicCertFile` → `PublicCertFile`
  - `certs.PrivateKeyFile` → `PrivateKeyFile`

### api/minio-operator-vendored/minio.min.io/v2/register.go
- **Line 18**: Commented out import to internal MinIO package:
  - `github.com/minio/operator/pkg/apis/minio.min.io`
- **Line 29**: Changed `operator.GroupName` to `GroupName`

## What Was Vendored

We vendored the API type definitions needed for our operator:

- `minio.min.io/v2/` - MinIO Tenant CRD types and helpers
  - `register.go` - API group registration and scheme builder
  - `doc.go` - Package documentation
  - `types.go` - Main Tenant type definitions
  - `constants.go` - Constants used by the API
  - `helper.go` - Helper functions
  - `labels.go` - Label constants and functions
  - `names.go` - Naming utilities
  - `utils.go` - Utility functions
  - `conversion.go` - Conversion utilities
  - `zz_generated.deepcopy.go` - Generated DeepCopy methods
  - `zz_generated.defaults.go` - Generated default values
  - `vendored_helpers.go` - Helper constants (new file)

## License

The vendored code maintains its original GNU Affero General Public License v3.0 from the MinIO Operator project.

## Removal

This vendored copy should be reviewed when upgrading to newer versions of the MinIO operator to determine if any important changes or fixes need to be incorporated.
