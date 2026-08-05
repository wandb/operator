# SeaweedFS Operator API Vendored Code

This directory contains vendored API types from the [SeaweedFS Operator](https://github.com/seaweedfs/seaweedfs-operator) project.

## Source

- **Repository**: https://github.com/seaweedfs/seaweedfs-operator
- **Operator Version**: 1.0.32
- **Helm Chart Version**: 0.1.35
- **Date Vendored**: 2026-07-17

## Reason for Vendoring

To have full control over the CRD API types and avoid unexpected breaking changes when the upstream operator updates. This allows the W&B operator to manage SeaweedFS clusters while controlling when and how we adopt upstream changes. Vendoring also avoids pulling in the full SeaweedFS server dependency tree via go.mod.

## Changes Made

### Removed Code Generation Annotations

All `+kubebuilder:*` annotations have been removed from vendored types. These annotations are only needed when running code generators, which we do not do for vendored APIs.

### Removed Files

- `component_accessor.go` - Runtime helpers not needed for CR creation/status reading
- `seaweed_webhook.go` - Webhook logic not needed
- `seaweed_webhook_test.go` - Tests

### Added Files

- `vendored_helpers.go` - GroupName constant

## What Was Vendored

### CRDs

- `crds/seaweed.seaweedfs.com_seaweeds.yaml`
  - **Source**: https://github.com/seaweedfs/seaweedfs-operator/config/crd/bases/seaweed.seaweedfs.com_seaweeds.yaml

### API Types

- `seaweed.seaweedfs.com/v1/` - SeaweedFS CRD types
  - `types.go` - Main Seaweed type definitions
  - `groupversion_info.go` - API group registration and scheme builder
  - `vendored_helpers.go` - Helper constants
  - `zz_generated.deepcopy.go` - Generated DeepCopy methods

## License

The vendored code is licensed under the Apache License 2.0 from the SeaweedFS Operator project.
