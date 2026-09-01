# KEDA API Vendored Code

This directory contains vendored API types from the [KEDA](https://github.com/kedacore/keda) project.

## Source

- **Repository**: https://github.com/kedacore/keda
- **Version**: v2.18.3
- **Date Vendored**: 2026-09-01

## Reason for Vendoring

`Application.spec.scaledObjectTemplate` only needs `ScaledObjectSpec`, but importing it pulled the
whole KEDA module in as a direct dependency. That put KEDA scaler CVEs on this repo's alert list for
code the operator never executes (GHSA-6w3m-4hhp-775q), and the patched releases require
`k8s.io/api v0.36` + `controller-runtime v0.23`, forcing an unrelated k8s bump on the whole operator.

## Changes Made

### Kept

Only the types reachable from `ScaledObjectSpec`, with their upstream validation markers intact so the
generated CRD schema is unchanged:

- `scaledobject_types.go`: `ScaledObjectSpec`, `Fallback`, `AdvancedConfig`, `ScalingModifiers`,
  `HorizontalPodAutoscalerConfig`, `ScaleTarget`
- `scaletriggers_types.go`: `ScaleTriggers`, `AuthenticationRef`

### Deleted

- `ScaledObject`, `ScaledObjectStatus`, `ScaledObjectList` and the rest of the group's kinds
  (`ScaledJob`, `TriggerAuthentication`, …) — the operator never reads or writes KEDA objects
- Webhooks, `SchemeBuilder` registration, helper methods and their tests
- Everything gated on KEDA-internal packages

## Regenerating deepcopy

`make generate` only covers `./api/v1` and `./api/v2`, so run this directly after editing the types:

```bash
./bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./pkg/vendored/keda/..."
```
