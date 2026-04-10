# Monitoring and Telemetry Guide

This repo now ships a configurable telemetry stack for W&B.

## Behavior

- `telemetry.mode: off`  
  No telemetry stack resources are rendered and the operator does not wire OTEL endpoints.

- `telemetry.mode: forward`  
  The Victoria stack is installed, workloads send OTEL data to the in-cluster gateway, and that gateway forwards OTLP data to a customer-managed endpoint.

- `telemetry.mode: full`  
  The Victoria stack is installed and exposed through in-cluster Grafana dashboards and datasources.

Installed resources in `forward` and `full` modes:
- `VMSingle`, `VMAgent`, `VLSingle`, `VTSingle`
- OTLP gateway collector (`victoria-otlp-gateway`)
- Scrapes (`VMNodeScrape`, `VMPodScrape`, `VMServiceScrape`) when `telemetry.scrape.enabled=true`
- Alerting (`VMRule`, `VMAlert`) when `telemetry.alerting.enabled=true`

Installed resources only in `full` mode:
- Grafana + Victoria datasources

Not installed:
- Perses
- vmui

## Helm Values

For the operator chart, set these under top-level `telemetry`:

```yaml
telemetry:
  mode: forward
  namespace: wandb
  otel:
    secretName: wandb-otel-connection
    protocol: http/protobuf
    serviceName: wandb-service
    resourceAttributes: deployment.environment=prod
  forwarding:
    otlp:
      endpoint: https://otel.example.com
      protocol: http/protobuf
      headers:
        Authorization: Bearer <token>
```

Notes:
- Retention defaults to `1d`.
- `telemetry.mode=forward/full` requires `telemetry.otel.secretName`.
- `telemetry.mode=forward` requires `telemetry.forwarding.otlp.endpoint`.
- The telemetry mode controls the stack behavior, but Helm still needs dependency booleans for the VictoriaMetrics and Grafana operator subcharts.
- When telemetry is enabled, the operator reads settings from the in-namespace `wandb-operator-telemetry-config` ConfigMap through the Kubernetes API and re-reconciles W&B instances when that ConfigMap changes.
- When telemetry is off, that ConfigMap is not rendered and the operator falls back to disabled telemetry behavior.
- Use the preset files in `deploy/operator/profiles/` to avoid remembering those extra flags and to switch modes cleanly on an existing release.
- `helm upgrade --install wandb-operator ./deploy/operator --reset-values -f ./deploy/operator/profiles/telemetry-full.yaml` installs the operator plus the full local telemetry stack.
- `helm upgrade --install wandb-operator ./deploy/operator --reset-values -f ./deploy/operator/profiles/telemetry-off.yaml` disables the telemetry stack and its dependent operators.
- `helm install telemetry ./deploy/telemetry --set mode=full --set namespace=<ns>` installs just the telemetry resources and expects the VictoriaMetrics/Grafana operators and CRDs to already exist.
- When rendering YAML manually, use `helm template <release> ./deploy/operator --include-crds ...` so the VictoriaMetrics and Grafana CRDs are present before apply.

## Operator Runtime Values

Set OTEL connection settings under `telemetry.otel`:

```yaml
telemetry:
  otel:
    secretName: wandb-otel-connection
    protocol: http/protobuf
    serviceName: wandb-service
    resourceAttributes: deployment.environment=prod
```

## Tilt Usage

Set `"installTelemetry": True` in `tilt-settings.star`.

Tilt treats `installTelemetry=True` as the local `full` mode. It installs the
VictoriaMetrics and Grafana operators, then installs the standalone telemetry
chart with `mode=full`, and it writes `wandb-operator-telemetry-config` in
`operator-system` so the operator picks up the same configmap-backed telemetry
settings as the Helm flow.

Tilt exposes endpoints for:
- Grafana
- VictoriaMetrics
- VictoriaLogs
- VictoriaTraces

## Manifest `source.type=telemetry`

The resolved operator-managed telemetry status is also published on
`WeightsAndBiases.status.telemetryStatus`, including `ready`, `state`, `mode`,
and nested `connection` details such as the effective protocol, endpoints,
secret name, and gorilla tracer connection.

```bash
kubectl get weightsandbiases <name> -n <namespace> -o jsonpath='{.status.telemetryStatus}'
```

```bash
kubectl get weightsandbiases <name> -n <namespace> -o jsonpath='{.status.telemetryStatus.connection.connectionSecret}'
```

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
   - `helm template wandb-operator ./deploy/operator --include-crds -f ./deploy/operator/profiles/telemetry-full.yaml`
   - or `helm template ./deploy/telemetry --set mode=full --set namespace=<telemetry-namespace>`
2. Verify Grafana resources:
   - `kubectl get grafana,grafanadatasource -n <telemetry-namespace>`
3. Verify telemetry secret:
   - `kubectl get secret <configured-secret-name> -n <wandb-namespace> -o yaml`
4. Verify pod env:
   - `kubectl exec -n <wandb-namespace> deploy/<app> -- env | grep OTEL_`
