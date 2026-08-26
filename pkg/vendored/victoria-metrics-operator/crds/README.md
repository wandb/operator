# VictoriaMetrics Operator CRDs (Vendored)

This directory contains the CRD manifests vendored from the
[VictoriaMetrics/operator](https://github.com/VictoriaMetrics/operator) project.

## Source

- **Repository**: https://github.com/VictoriaMetrics/operator
- **Operator version (appVersion)**: v0.67.0
- **Fetched via**: victoria-metrics-operator Helm chart 0.58.1
  (https://victoriametrics.github.io/helm-charts/, pinned in `deploy/operator/Chart.yaml`)
- **Date Vendored**: 2026-08-03

## Reason for Vendoring

The crd-installer Job installs these CRDs via server-side apply on both install and upgrade,
so managed telemetry can be enabled on a live cluster (`off` → `forward`/`full`) without Helm
having to add CRDs during an upgrade — which Helm does not do.

## What Was Vendored

CRD manifests only, applied **verbatim** — extracted from the pinned upstream Helm chart by
`make generate-vendored` and embedded into the crd-installer by `make sync-crd-embed`. No W&B
modifications are made (the installer only mutates the operator's own CRDs).

Unlike `redis-operator` / `altinity-clickhouse`, we do **not** vendor Go API types or run any
code generation for these: the operator never constructs VictoriaMetrics CRs in Go (the
`deploy/telemetry` chart does), and the installer parses the CRD YAML generically.

## License

The manifests originate from the upstream VictoriaMetrics operator project and retain its
Apache 2.0 license.

## Updating

Re-run `make generate-vendored` after bumping the victoria-metrics-operator chart version in
`deploy/operator/Chart.yaml`, then `make sync-crd-embed`. Update the versions/date above.
