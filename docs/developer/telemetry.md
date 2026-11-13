# Telemetry internals

For deployment settings, use [monitoring](../user/monitoring.md). The Operator
chart writes runtime telemetry settings into the `wandb-operator-telemetry-config`
ConfigMap in its release namespace, rendered only when `telemetry.mode` is not
`off`. The controller reads it and reconciles W&B connection Secrets and status;
a missing ConfigMap means telemetry is disabled. The controller watches it and
re-enqueues every W&B resource on change. `--telemetry-config-name` and
`--telemetry-config-namespace` override its location ([main.go](../../cmd/manager/main.go)).
The charts live in [deploy/operator](../../deploy/operator) and
[deploy/telemetry](../../deploy/telemetry).

## Tilt Usage

Telemetry is off by default in Tilt. Set `"observabilityMode": "full"` in
`tilt-settings.star` for the local full stack.

Tilt renders the operator chart with `telemetry.mode=full`, enables the
VictoriaMetrics and Grafana operator dependencies, and lets the controller read
the chart-rendered telemetry ConfigMap.

Tilt exposes endpoints for:

- Grafana
- VictoriaMetrics
- VictoriaLogs
- VictoriaTraces

## Manifest `source.type=telemetry`

The resolved operator-managed telemetry status is published on
`WeightsAndBiases.status.telemetryStatus`, including `ready`, `state`, `mode`,
and nested `connection` details such as the effective protocol, endpoints,
Secret name, gorilla tracer connection, DogStatsD address, and local
Datadog-agent compatibility endpoint. These Datadog-compatible values point at
the in-cluster telemetry gateway; they do not add a Datadog SaaS exporter.

For a development resource named `wandb` in namespace `wandb`:

```bash
kubectl get weightsandbiases wandb -n wandb -o jsonpath='{.status.telemetryStatus}'
```

```bash
kubectl get weightsandbiases wandb -n wandb -o jsonpath='{.status.telemetryStatus.connection.connectionSecret}'
```

You can source env vars from the operator-managed telemetry secret:

```yaml
env:
  - name: GORILLA_TRACER
    sources:
      - type: telemetry
        field: gorillaTracer
  - name: GORILLA_STATSD_ADDRESS
    sources:
      - type: telemetry
        field: statsdAddress
  - name: DD_TRACE_AGENT_URL
    sources:
      - type: telemetry
        field: datadogTraceAgentURL
  - name: DD_AGENT_HOST
    sources:
      - type: telemetry
        field: datadogTraceAgentHost
  - name: DD_TRACE_AGENT_PORT
    sources:
      - type: telemetry
        field: datadogTraceAgentPort
  - name: OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
    sources:
      - type: telemetry
        field: metricsEndpoint
  - name: OTEL_EXPORTER_OTLP_LOGS_ENDPOINT
    sources:
      - type: telemetry
        field: logsEndpoint
  - name: OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
    sources:
      - type: telemetry
        field: tracesEndpoint
```

## Chart development

Render the Operator chart with `--include-crds` and each telemetry profile when
changing templates. The forward profile requires a destination endpoint. See
[testing](testing.md) for chart checks. The standalone telemetry chart expects
its controllers and CRDs to exist already.

## Image packaging and pod-log collection

`make download-grafana-plugins` downloads and verifies the pinned VictoriaLogs
plugin before `make docker-build`. The Dockerfile copies it into
`/opt/wandb/grafana-plugins`; Grafana's init container copies that directory into
its shared plugin volume. Update the pinned version and checksum together.
The parent chart selects the installed Operator image by default, while the
standalone telemetry chart requires `grafana.pluginImage` in full mode.

The [telemetry helpers](../../deploy/telemetry/templates/_helpers.tpl) centralize
registry prefixes, pull Secrets, and the Grafana resource. Keep explicit Victoria
and config-reloader image versions aligned with the bundled controller. Validate
both the parent Operator chart and standalone telemetry chart when changing this
contract. User-facing settings are in [restricted environments](../user/restricted-environments.md#telemetry-images-and-plugins).

The log-agent DaemonSet tails node Pod logs through a filelog receiver, persists
offsets on the host, and exports through the OTLP gateway. Its namespace filters
include the telemetry and release namespaces plus `logAgent.extraNamespaces`.
It excludes its own logs to avoid a collection loop. The agent needs root and
hostPath access; keep the OpenShift profile's log-agent opt-out intact.

The full-stack Grafana configuration serves `/console/grafana/` and includes
managed MySQL and deployment-availability panels. Console also has direct Pod
metrics read permissions; that path depends on metrics-server rather than the
Victoria scrape pipeline.
