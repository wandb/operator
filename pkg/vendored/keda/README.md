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

`zz_generated.deepcopy.go` is generated, never hand-edited. This package is included in the
`generate` target's paths, so after editing the types run the same command as for `api/`:

```bash
make manifests generate sync-crd-embed
```

The other vendored packages are **not** in those paths on purpose: `controller-gen` fails on the
Altinity ClickHouse types (interface fields it can't model) and rewrites the Argo Rollouts output,
which was copied verbatim from upstream rather than generated here.
