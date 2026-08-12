# Grafana Operator CRDs (Vendored)

This directory contains the CRD manifests vendored from the
[grafana/grafana-operator](https://github.com/grafana/grafana-operator) project.

## Source

- **Repository**: https://github.com/grafana/grafana-operator
- **Operator version (appVersion)**: v5.21.4
- **Fetched via**: grafana-operator Helm chart 5.21.4 from the Grafana Helm repo
  (`https://grafana.github.io/helm-charts`), the source pinned for this dependency in
  `deploy/operator/Chart.yaml`. We vendor from that pinned channel — not a separate OCI
  registry — so the CRDs track the controller version the operator chart actually deploys.
- **Date Vendored**: 2026-08-03

## Reason for Vendoring

The crd-installer Job installs these CRDs via server-side apply on both install and upgrade,
so managed telemetry can be enabled on a live cluster (`off` → `full`) without Helm having to
add CRDs during an upgrade — which Helm does not do.

## What Was Vendored

CRD manifests only, applied **verbatim** — extracted from the pinned upstream Helm chart by
`make generate-vendored` and embedded into the crd-installer by `make sync-crd-embed`. No W&B
modifications are made (the installer only mutates the operator's own CRDs).

Unlike `redis-operator` / `altinity-clickhouse`, we do **not** vendor Go API types or run any
code generation for these: the operator never constructs Grafana CRs in Go (the
`deploy/telemetry` chart does), and the installer parses the CRD YAML generically.

## License

The manifests originate from the upstream Grafana operator project and retain its Apache 2.0
license.

## Updating

Re-run `make generate-vendored` after bumping the grafana-operator chart version in
`deploy/operator/Chart.yaml`, then `make sync-crd-embed`. Update the versions/date above.
