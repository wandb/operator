# Telemetry internals

For deployment settings, use [monitoring](../user/monitoring.md). The Operator
chart writes runtime telemetry settings into its release-namespace ConfigMap;
the controller reads it and reconciles W&B connection Secrets and status.
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
