# Monitoring and Telemetry Guide

This repo now supports a consolidated telemetry stack with VictoriaMetrics in both Helm and Tilt workflows.

## Modes

### Managed mode
Use this when the chart should install telemetry components in-cluster.

Installed (optional/toggle-driven):
- `VMSingle`
- `VMAgent`
- `VLSingle`
- `VTSingle`
- Scrapes (`VMNodeScrape`, `VMPodScrape`, `VMServiceScrape`)
- Alerting (`VMRule`, `VMAlert`)
- Optional UI (`Grafana`, `vmui`)
- Optional `Perses`

### External mode
Use this when metrics/logs/traces should be exported to existing external endpoints.

In external mode, managed VM resources are not rendered.

## Helm Configuration

### 1. Telemetry stack values (chart resources)
Set these under top-level `telemetry`.

Managed example:

```yaml
telemetry:
  enabled: true
  mode: managed
  namespace: wandb
  ui:
    grafana:
      enabled: false
    vmui:
      enabled: false
  alerting:
    enabled: true
    notifier:
      enabled: true
      target: http://alertmanager-operated.monitoring.svc:9093
```

External example:

```yaml
telemetry:
  enabled: true
  mode: external
  external:
    metricsEndpoint: https://metrics.example.com/v1/metrics
    logsEndpoint: https://logs.example.com/v1/logs
    tracesEndpoint: https://traces.example.com/v1/traces
```

### 2. Operator runtime telemetry values (secret + env source resolution)
Set these under `wandb-operator.telemetry`.

```yaml
wandb-operator:
  telemetry:
    enabled: true
    mode: managed
    managed:
      vmsingleName: victoria-instance
      vlsingleName: victoria-logs
      vtsingleName: victoria-traces
    otel:
      secretName: wandb-otel-connection
      protocol: http/protobuf
      serviceName: wandb-service
      resourceAttributes: deployment.environment=prod
```

For external runtime:

```yaml
wandb-operator:
  telemetry:
    enabled: true
    mode: external
    metricsEndpoint: https://metrics.example.com/v1/metrics
    logsEndpoint: https://logs.example.com/v1/logs
    tracesEndpoint: https://traces.example.com/v1/traces
```

## Tilt Usage

Set `installTelemetry: true` in `tilt-settings.json`.

Tilt now uses Helm-driven telemetry resources (no static telemetry YAML apply path), and passes telemetry runtime flags to the locally built operator manager.

## Using `source.type=telemetry` in manifests

You can source app env vars from the operator-managed telemetry secret:

```yaml
env:
  - name: GORILLA_TRACER
    sources:
      - type: telemetry
        field: gorillaTracer
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

Supported telemetry `field` values:
- `metricsEndpoint`
- `logsEndpoint`
- `tracesEndpoint`
- `metricsExporter`
- `logsExporter`
- `tracesExporter`
- `protocol`
- `serviceName`
- `resourceAttributes`
- `gorillaTracer`

## Validation Checklist

1. Render checks:
   - `helm template ... --set telemetry.enabled=true --set telemetry.mode=managed`
   - `helm template ... --set telemetry.enabled=true --set telemetry.mode=external --set telemetry.external.metricsEndpoint=... --set telemetry.external.logsEndpoint=... --set telemetry.external.tracesEndpoint=...`
2. Secret check:
   - `kubectl get secret wandb-otel-connection -n <wandb-namespace> -o yaml`
3. Pod env check:
   - `kubectl exec -n <wandb-namespace> deploy/<app> -- env | grep OTEL_`
4. Alerting check:
   - `kubectl get vmalert,vmrule -n <telemetry-namespace>`
5. Optional UI check:
   - `kubectl get grafana,grafanadatasource -n <telemetry-namespace>`
   - `kubectl get svc vmui -n <telemetry-namespace>`

## Notes

- Per-infrastructure telemetry toggles in the W&B CR (`mysql/redis/kafka/minio/clickhouse.telemetry.enabled`) remain in place for exporter behavior.
- Managed Grafana and vmui remain disabled by default.
