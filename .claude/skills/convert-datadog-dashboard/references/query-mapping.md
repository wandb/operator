# DataDog query → MetricsQL/PromQL translation

VictoriaMetrics speaks MetricsQL, a PromQL superset. Most translations below produce valid PromQL; a few use MetricsQL-only constructs where they're cleaner. The operator's stack accepts both.

## Aggregator + grouping

DataDog: `<agg>:<metric>{<filters>} by {<group>}`
MetricsQL: `<agg> by (<group>) (<metric>{<filters>})`

Mappings:

| DD aggregator | MetricsQL |
|---|---|
| `max:` | `max by (<group>) (...)` |
| `min:` | `min by (<group>) (...)` |
| `sum:` | `sum by (<group>) (...)` |
| `avg:` | `avg by (<group>) (...)` |

If no `by {}` clause, drop the `by ()` — emit a scalar aggregate.

## Suffix functions (chained with `.`)

| DD suffix | MetricsQL |
|---|---|
| `.as_count()` | Drop. Add a TODO description if the underlying metric is clearly a gauge being miscoerced. |
| `.as_rate()` | Wrap whole expression: `rate(<expr>[1m])` |
| `.rollup(max)` | `max_over_time(<expr>[1m])` |
| `.rollup(max, 60)` | `max_over_time(<expr>[60s])` |
| `.rollup(min, N)` | `min_over_time(<expr>[Ns])` |
| `.rollup(avg, N)` | `avg_over_time(<expr>[Ns])` |
| `.rollup(sum, N)` | `sum_over_time(<expr>[Ns])` |
| `.fill(zero)` | `<expr> or vector(0)` |
| `.fill(linear)` | Drop; MetricsQL has `interpolate(<expr>)` but it changes semantics — leave a TODO if precision matters. |

## Wrapper functions used in `formulas[].formula`

| DD | MetricsQL |
|---|---|
| `monotonic_diff(q)` | `increase(<q>[1m])` — leave a TODO noting the 1m window assumption |
| `default_zero(q)` | `(<q>) or vector(0)` |
| `exclude_null(q)` | Drop the wrapper. MetricsQL omits null series by default. |
| `per_second(q)` | `rate(<q>[1m])` |
| `log2(q)` | `log2(<q>)` (MetricsQL; not standard PromQL — flag if portability matters) |
| `cutoff_max(q, N)` | `clamp_max(<q>, N)` |
| `cutoff_min(q, N)` | `clamp_min(<q>, N)` |
| `abs(q)` | `abs(<q>)` |
| `hour_before(q)` / `day_before(q)` / `week_before(q)` | `<expr> offset 1h` / `offset 24h` / `offset 7d` |
| `derivative(q)` | `deriv(<q>[1m])` |
| `timeshift(q, -3600)` | `<q> offset 1h` |
| Arithmetic on multiple queries (`query1 + query2`) | Substitute each `queryN` with its translated expression, keep the operator. |

## Filter substitution

DD filter sets look like `{$customer-ns,$env,$cloud_provider,key:value,!exclude:value}`. Apply in order:

1. Drop any token starting with `$` — those are DD template variables and we don't use templating.
2. Convert `key:value` → `key="value"`.
3. Convert `!key:value` → `key!="value"`.
4. Convert `key:*` → drop the token (PromQL has no glob; `*` matched everything in DD).
5. If after all drops the filter set is empty, drop the braces entirely.

Examples:

- DD: `aws.rds.cpuutilization{$customer-ns,$env,$cloud_provider} by {customer-ns}`
  → `max by (customer_ns) (aws_rds_cpuutilization)`
  (note: also dotted-prefix → underscore conversion, below)

- DD: `kubernetes.cpu.usage.total{$env, $customer-ns, !$excluded-customer-ns, container_name:bufstream} by {customer-ns,pod_name}`
  → `max by (customer_ns, pod_name) (kubernetes_cpu_usage_total{container_name="bufstream"})`

## Metric name conversion

DD metric names use dots: `aws.rds.cpuutilization`. PromQL and most Prometheus exporters use underscores: `aws_rds_cpuutilization`. Always convert dots to underscores when emitting the expression. Preserve the metric name verbatim otherwise — do not invent a "more correct" name; the user can rename in a follow-up.

Label names with dots (e.g. `kafka.topic.name`, `kafka.consumer.group.id`) follow the same rule when they appear in filter expressions: `kafka_topic_name`, `kafka_consumer_group_id`.

## Known substitutions (DD → MetricsQL)

When the translated DD metric matches one on the left, replace the metric expression in the output with the corresponding MetricsQL on the right. This produces queryable panels for cross-cloud and DD-wrapped-k8s metrics that have clear self-hosted equivalents in our scrape set.

Substitutions are consulted **before** the TODO list (next section). If a metric is in both, substitution wins — see SKILL.md Step 4 for the ordering.

**Confidence bar:** every row below has been verified semantically equivalent in units and meaning. Do not add rows where the LHS and RHS differ in scale, unit, or what they actually measure (e.g. CFS throttled *periods* vs throttled *seconds*) — leave those in the TODO list so the user sees the gap.

| DD metric (literal-translated form) | MetricsQL substitution |
|---|---|
| `aws_rds_cpuutilization`, `azure_dbformysql_flexibleservers_cpu_percent`, `gcp_cloudsql_database_cpu_utilization` | `100 * rate(container_cpu_usage_seconds_total{container=~"mysqld\|pxc"}[$__rate_interval])` |
| `aws_rds_database_connections`, `azure_dbformysql_flexibleservers_total_connections`, `gcp_cloudsql_database_mysql_connections_count` | `mysql_global_status_threads_connected` |
| `aws_elasticache_database_memory_usage_percentage`, `azure_cache_redis_usedmemorypercentage`, `gcp_redis_stats_memory_usage_ratio` | `100 * redis_memory_used_bytes / redis_memory_max_bytes` |
| `aws_elasticache_curr_connections`, `azure_cache_redis_connectedclients`, `gcp_redis_clients_connected` | `redis_connected_clients` |
| `kubernetes_cpu_usage_total` | `rate(container_cpu_usage_seconds_total[$__rate_interval])` — apply label-rename rules below |
| `kubernetes_memory_usage` | `container_memory_working_set_bytes` — apply label-rename rules below |
| `kubernetes_network_rx_bytes` | `rate(container_network_receive_bytes_total[$__rate_interval])` — apply label-rename rules below |
| `kubernetes_network_tx_bytes` | `rate(container_network_transmit_bytes_total[$__rate_interval])` — apply label-rename rules below |
| `kubernetes_state_node_cpu_capacity` | `machine_cpu_cores` |

### Label-rename rules

Apply these label renames only when the substitution's RHS metric is `container_*`, `machine_*`, or `kubelet_*`. cAdvisor/kubelet use different label names than DD's `kubernetes.*` integration.

- DD label `pod_name` → cAdvisor label `pod`
- DD label `container_name` → cAdvisor label `container`
- DD label `kube_namespace` → cAdvisor label `namespace`

Apply the rename in both directions: inside `by (...)` groupings AND inside `{label="value"}` filter sets. Do NOT rename labels when the RHS is a non-cAdvisor metric (e.g. `mysql_*`, `redis_*`) — those exporters use their own label conventions.

### Audit description

When a substitution fires, attach a panel description to make the swap auditable:

```
Substituted from DataDog metric '<original_dd_metric_underscore_form>'.
```

If the panel already has a description (from the widget mapping or scrub), append a new paragraph rather than overwriting.

## Unscraped metric prefixes (TODO list)

If a translated expression's metric prefix matches one of these AND it didn't match a substitution above, attach the TODO description to the panel (see SKILL.md Step 4). Verify this list against `deploy/telemetry/templates/telemetry-scrapes.yaml` at conversion time — the scrape set may have grown.

Currently NOT scraped by `deploy/telemetry/templates/telemetry-scrapes.yaml`:

- `aws_rds_*` — AWS RDS via CloudWatch (DD integration only)
- `aws_elasticache_*` — AWS ElastiCache via CloudWatch
- `aws_ec2_*` — AWS EC2 instance metrics
- `aws_autoscaling_*` — AWS Auto Scaling Groups
- `aws_vpc_*` — AWS VPC metrics
- `azure_dbformysql_flexibleservers_*` — Azure Database for MySQL Flexible Servers
- `azure_cache_redis_*` — Azure Cache for Redis
- `gcp_cloudsql_*` — GCP Cloud SQL
- `gcp_redis_*` — GCP Memorystore for Redis
- `bufstream_*` — Bufstream (DD-only OTel collection path; not via VMPodScrape today)
- `helm_release` — Helm release status (no exporter scraped today)
- `terraform_workspace_*` — Terraform Cloud workspace status (DD-only)
- `oom_kill_*` — DD OOM Kill check
- `kubernetes_*` (DD-style, only the ones NOT in the substitutions table above) — DD's wrapped kubernetes.* metrics. The substitutions table handles `kubernetes_cpu_usage_total`, `kubernetes_memory_usage`, `kubernetes_network_rx_bytes`, `kubernetes_network_tx_bytes` cleanly. Everything else under this prefix (e.g. `kubernetes_cpu_requests`, `kubernetes_cpu_limits`, `kubernetes_memory_requests`, `kubernetes_memory_limits`, `kubernetes_containers_restarts`, `kubernetes_pods_running`, `kubernetes_cpu_cfs_throttled_periods`) requires kube-state-metrics, which we don't currently scrape — TODO.
- `kubernetes_state_*` — DD's wrapped kube-state-metrics. We do not run kube-state-metrics in this stack today (`kubernetes_state_node_cpu_capacity` is the one exception covered by the substitutions table → `machine_cpu_cores`). Everything else stays TODO until kube-state-metrics is added to the scrape set.

Currently scraped (rough prefixes the user can rely on):

- `up`, `scrape_*`, `process_*`, `go_*` — always present
- `container_*`, `machine_*`, `cadvisor_*` — from kubelet/cadvisor scrape
- `kubelet_*` — from kubelet scrape
- `controller_runtime_*`, `workqueue_*`, `rest_client_*` — operator controller metrics
- `mysql_*`, `mysql_global_status_*`, `mysql_global_variables_*` — mysql exporter / mysqld
- `kafka_*`, `strimzi_*`, `kafka_server_*` — Strimzi-managed Kafka brokers
- `minio_*` — MinIO tenant metrics
- `redis_*` — redis_exporter on standalone Redis
- `ClickHouse*` (uppercase prefix is correct for ClickHouse native metrics endpoint), plus `chi_*` from the operator
- `grafana_*`, `vm_*` — Grafana operator, VictoriaMetrics operator

## Cross-cloud "first non-null" patterns

DD dashboards often define multiple queries (one per CSP) and `formulas` that just list them as alternatives. After dropping the cloud-managed-service metrics that aren't scraped, you may be left with a single self-hosted-metric query (e.g. `kubernetes.cpu.usage.total` for the self-hosted Redis on k8s). Prefer keeping the self-hosted query as the panel's primary target and dropping the cloud-managed alternatives — but keep them in the panel description as comments so a future scrape-config addition can re-enable them:

```
"description": "TODO: cloud-managed alternatives (currently not scraped): aws_elasticache_cpuutilization, azure_cache_redis_server_load, gcp_redis_stats_cpu_utilization"
```

## Examples

DD: `max:gcp.cloudsql.database.cpu.utilization{$customer-ns,$env,$cloud_provider} by {customer-ns}.rollup(max)`
→ `max_over_time(max by (customer_ns) (gcp_cloudsql_database_cpu_utilization)[1m])`
→ panel description: TODO unscraped `gcp_cloudsql_*`

DD: `monotonic_diff(sum:kubernetes.containers.restarts{$env,$customer-ns} by {customer-ns})`
→ `increase(sum by (customer_ns) (kubernetes_containers_restarts)[1m])`
→ panel description: TODO unscraped `kubernetes_*` (DD-style); window assumption 1m

DD: `sum:bufstream.kafka.produce.bytes{$customer-ns,$env,$cloud_provider,$size,$kafka.topic.name} by {customer-ns,kafka.topic.name,kafka.topic.partition}.as_count()`
→ `sum by (customer_ns, kafka_topic_name, kafka_topic_partition) (bufstream_kafka_produce_bytes)`
→ panel description: TODO unscraped `bufstream_*`; `.as_count()` dropped
