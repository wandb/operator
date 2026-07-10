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
- When telemetry is enabled, Helm renders `wandb-operator-telemetry-config` in the operator release namespace. The operator reads that ConfigMap through the Kubernetes API and re-reconciles W&B instances when it changes.
- When telemetry is off, that ConfigMap is not rendered and the operator treats telemetry as disabled.
- Use the preset files in `deploy/operator/profiles/` to avoid remembering those extra flags and to switch modes cleanly on an existing release.
- `helm upgrade --install wandb-operator ./deploy/operator --reset-values -f ./deploy/operator/profiles/telemetry-full.yaml` installs the operator plus the full local telemetry stack.
- `helm upgrade --install wandb-operator ./deploy/operator --reset-values -f ./deploy/operator/profiles/telemetry-off.yaml` disables the telemetry stack and its dependent operators.
- `helm install telemetry ./deploy/telemetry --set mode=full --set namespace=<ns>` installs just the telemetry resources and expects the VictoriaMetrics/Grafana operators and CRDs to already exist.
- When rendering YAML manually, use `helm template <release> ./deploy/operator --include-crds ...` so the VictoriaMetrics and Grafana CRDs are present before apply.

## Operator Runtime Values

Set OTEL secret defaults under `telemetry.otel`:

```yaml
telemetry:
  otel:
    secretName: wandb-otel-connection
    protocol: http/protobuf
    serviceName: wandb-service
    resourceAttributes: deployment.environment=prod
```

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

## Dashboards (mode `full`)

Grafana dashboards live in `deploy/telemetry/dashboards/*.json` and are provisioned by
`deploy/telemetry/templates/telemetry-ui.yaml` (one `GrafanaDashboard` per file). They are
organized for **support triage**: start at Field Investigation, then follow the symptom to a
deep-dive. Every chart has an adjacent `About:` notecard explaining what it shows, what "bad"
looks like, and what to do next.

| Dashboard (uid) | Use when the customer says… |
|---|---|
| `wandb-field-investigation` | **Start here.** Golden signals + a symptom→dashboard index. |
| `wandb-telemetry-overview` | At-a-glance health across app + infra; includes telemetry self-health. |
| `wandb-managed-install-performance` | Infra host health (MySQL/Redis/Kafka/ClickHouse/object store, CPU/mem/restarts). |
| `wandb-api` | "The whole app is slow or erroring." HTTP/GraphQL RED, latency, rate-limiting. |
| `wandb-graphql-errors` | "Specific queries fail." GraphQL error attribution by service/operation. |
| `wandb-ingest` | "My run isn't logging / data is delayed." Filestream + Kafka ingest lag. |
| `wandb-run-data` | "Charts won't load / show wrong data." History read path, sampling, dual-read. |
| `wandb-artifacts` | "Artifact won't upload/download." Artifact ops, manifests, GC, TTL. |
| `wandb-jobs` | "Old data slow / storage growing." Parquet export lag, cron, background jobs. |
| `wandb-datastores` | "The DB/cache is the bottleneck" (application's view of the stores). |

### Sampled vs. unsampled — read the numbers correctly

Two metric sources feed these dashboards and they answer different questions:

- **DogStatsD counters** (e.g. `graphql_operation_failed`, `operation_*`) are **unsampled** —
  the source of truth for error *counts* and *rates*. Use them to state how bad something is.
- **Trace-derived metrics** (`traces_spanmetrics_*`, produced by the spanmetrics connector) are
  computed from **sampled** traces — great for *attribution* (which service/operation/route),
  but their absolute counts undercount. Trust the *shape*, not the exact number.

Panels label which source they use; notecards call it out where it matters.

### Data-availability caveats (verified against a live `mode=full` stack)

Some panels depend on signals that a given deployment may not emit. Where a metric was
confirmed absent, the panel carries a `TODO:` note in its description rather than showing a
silently-empty chart:

- **DogStatsD panels are empty unless gorilla emits DogStatsD.** These (`operation_*`,
  `ch_hard_read_*`, `gorilla_bigtablev2_*`, `dual_write`, `handle_line`, `graphql_operation_*`,
  the `dist_*` family) require gorilla to run with `GORILLA_STATSD_PORT` set to a non-zero value.
  Several server manifests ship it as `0`, which disables emission — a **core/manifest-side**
  setting, not an operator bug. The percentile mapping (`timer_histogram_mapping`) is in place and
  takes effect once the metrics flow.
- **HTTP metrics carry no route label.** gorilla's `http_server_*` metrics only expose
  `http_request_method` and `http_response_status_code` (the only `http_route` value ever seen is
  `/ping`). Per-operation latency therefore comes from spanmetrics `span_name`
  (`traces_spanmetrics_duration_milliseconds_*`), not `http_route`.
- **gorilla RED counts use `traces_spanmetrics_calls_milliseconds_total`.** The OTLP (gorilla)
  path emits the calls counter under that name; `traces_spanmetrics_calls_total` carries only
  Datadog-SDK services (e.g. weave-trace). Attribution labels (`graphql_service`,
  `graphql_operationName`) populate for named operations; anonymous queries leave them empty.
- **`kube_*` requires the full-profile scrape.** Restart/pod-status panels need
  `telemetry.scrape.kubeStateMetrics: true` (set by the `telemetry-full` profile) so the
  kube-state-metrics `VMServiceScrape` is created.
- **VictoriaLogs can be empty.** If gorilla's OTEL log export isn't populated, the logs panels
  (and Trace → Logs pivots) have no data until logs flow.

### Support → engineering handoff (snapshots)

Retention is short (see `telemetry.retentionPeriod`; the `telemetry-full` profile sets `3d`), so
capture data **during** the incident:

- In Grafana, **Share → Snapshot** freezes the panel data at that moment. This is the artifact to
  hand to engineering — they can import it into any Grafana and inspect it offline.
- A plain dashboard-JSON export carries the layout and queries but **no data** — it is not a
  substitute for a snapshot when the source stack is unreachable or the window has aged out.

### Correlation (pivoting between signals)

The datasources are wired for cross-signal navigation: metric exemplars (spanmetrics attaches
trace IDs) link a latency/error point to an example **trace**; from a trace, **Trace → Logs**
jumps to the correlated logs in VictoriaLogs. Lead with metrics, pivot to a trace for the "why",
drop to logs for detail.
