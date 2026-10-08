# Monitor a deployment

Telemetry is configured in the Operator Helm release. W&B CRs expose the resolved
connection and readiness in `status.telemetryStatus`.

| Mode | Behavior |
| --- | --- |
| `off` | No telemetry stack and no Operator-provided OTEL endpoints |
| `forward` | In-cluster Victoria stack and OTLP gateway forwarding to your endpoint |
| `full` | In-cluster Victoria stack with Grafana dashboards and datasources |

Both enabled modes include metrics, logs, traces, and an OTLP gateway. Scrapes and
alerting are controlled independently; Grafana is installed only in `full` mode.
Retention defaults to `1d`.

## Enable the full stack

Merge these settings into your saved `operator-values.yaml`. Match the telemetry
namespace to your deployment:

```yaml
telemetry:
  mode: full
  namespace: wandb
  crds:
    victoriaMetrics: true
    grafana: true
  otel:
    secretName: wandb-otel-connection
    protocol: http/protobuf
    serviceName: wandb-service
  retentionPeriod: 1d
victoria-metrics-operator:
  enabled: true
grafana-operator:
  enabled: true
```

For an existing release that has telemetry off, install the required CRDs first:

```bash
helm upgrade wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version "$OPERATOR_VERSION" --namespace wandb-operators \
  --reuse-values --set telemetry.crds.victoriaMetrics=true \
  --set telemetry.crds.grafana=true --wait --timeout 10m
```

Then apply the complete values file. Use this command directly for a new
installation with the settings above:

```bash
helm upgrade --install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version "$OPERATOR_VERSION" --namespace wandb-operators --create-namespace \
  -f operator-values.yaml --wait --timeout 10m
```

Preserve your other chart settings when updating the values file. Both the mode
and dependency booleans are required.

## Forward telemetry externally

Use `mode: forward`, keep the VictoriaMetrics dependency and its CRDs enabled,
and disable the Grafana dependency. Configure the destination:

```yaml
telemetry:
  mode: forward
  crds:
    victoriaMetrics: true
    grafana: false
  forwarding:
    otlp:
      endpoint: https://otel.example.com
      protocol: http/protobuf
      headers: {}
victoria-metrics-operator:
  enabled: true
grafana-operator:
  enabled: false
```

Keep `telemetry.otel.secretName` set. Supply any required authentication headers
through your protected Helm values workflow. For an existing `off` installation,
perform the CRD preparation upgrade above with only VictoriaMetrics enabled.

## Verify and access telemetry

```bash
kubectl get weightsandbiases wandb -n wandb \
  -o jsonpath='{.status.telemetryStatus}'
kubectl get vmsingle,vmagent,vlsingle,vtsingle -n wandb
kubectl get grafana,grafanadatasource -n wandb
kubectl get services -n wandb
```

Run the Grafana query only for `full` mode. Use the listed Grafana Service and
its port to access the dashboard through your approved cluster access method,
such as `kubectl port-forward`. Verify data arrives from an application; resource
existence alone does not prove successful collection. In `forward` mode, also
check the receiving endpoint.

The connection includes Datadog-compatible local gateway endpoints for W&B
workloads; it does not configure a Datadog SaaS exporter.

To disable collection, set mode to `off`, both dependency booleans to `false`,
and both telemetry CRD flags to `false` in your complete values file, then upgrade.
Review telemetry data retention and provider cleanup behavior before removing a
stack whose stored data you need to retain.
