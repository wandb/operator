# Monitoring and Telemetry Guide

This repo now ships one managed telemetry stack for W&B.

## Behavior

- `telemetry.enabled: false`  
  No telemetry stack resources are rendered and the operator does not wire OTEL endpoints.

- `telemetry.enabled: true`  
  The managed Victoria stack is installed and the operator writes OTEL connection settings for apps.

Installed resources in managed mode:
- `VMSingle`, `VMAgent`, `VLSingle`, `VTSingle`
- OTLP gateway collector (`victoria-otlp-gateway`)
- Scrapes (`VMNodeScrape`, `VMPodScrape`, `VMServiceScrape`) when `telemetry.scrape.enabled=true`
- Alerting (`VMRule`, `VMAlert`) when `telemetry.alerting.enabled=true`
- Grafana + Victoria datasources

Not installed:
- Perses
- vmui

## Helm Values

For the full operator install, set these under top-level `telemetry`:

```yaml
telemetry:
  enabled: true
  namespace: wandb
  retentionPeriod: 1d
  scrape:
    enabled: true
  alerting:
    enabled: false
```

Notes:
- Retention defaults to `1d`.
- There is no customer-facing external endpoint mode in this phase.
- `helm install wandb-operator ./deploy/operator --set telemetry.enabled=true` installs the operator plus the managed telemetry stack.
- `helm install telemetry ./deploy/telemetry --set enabled=true --set namespace=<ns>` installs just the telemetry resources and expects the VictoriaMetrics/Grafana operators and CRDs to already exist.

## Operator Runtime Values

Set OTEL secret defaults under `wandb-operator.telemetry`:

```yaml
wandb-operator:
  telemetry:
    otel:
      secretName: wandb-otel-connection
      protocol: http/protobuf
      serviceName: wandb-service
      resourceAttributes: deployment.environment=prod
```

## Tilt Usage

Set `"installTelemetry": True` in `tilt-settings.star`.

Tilt installs the telemetry stack through Helm and exposes endpoints for:
- Grafana
- VictoriaMetrics
- VictoriaLogs
- VictoriaTraces

## Manifest `source.type=telemetry`

You can source env vars from the operator-managed telemetry secret:

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

## Validation Checklist

1. Render stack:
   - `helm template ./deploy/operator --set telemetry.enabled=true`
   - or `helm template ./deploy/telemetry --set enabled=true --set namespace=<telemetry-namespace>`
2. Verify Grafana resources:
   - `kubectl get grafana,grafanadatasource -n <telemetry-namespace>`
3. Verify telemetry secret:
   - `kubectl get secret wandb-otel-connection -n <wandb-namespace> -o yaml`
4. Verify pod env:
   - `kubectl exec -n <wandb-namespace> deploy/<app> -- env | grep OTEL_`
