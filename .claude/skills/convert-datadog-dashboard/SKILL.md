---
name: convert-datadog-dashboard
description: Convert a DataDog dashboard JSON into a Grafana dashboard JSON that fits the operator's telemetry stack (VictoriaMetrics datasources, wandb-<component> UID, no templating). Trigger when the user provides DataDog dashboard JSON (inline or as a file path) and asks to port, convert, or recreate it as a Grafana dashboard, or when the user asks to add a new dashboard under deploy/telemetry/dashboards/ from a DataDog source.
---

# Convert DataDog dashboard → Grafana dashboard

This skill ports a DataDog dashboard JSON into a Grafana dashboard JSON that matches the operator's existing telemetry stack. It does NOT edit the Helm wiring template — it prints a wiring reminder at the end so the user sees that change alongside the new dashboard file in a single diff.

## Scope: per-CR only (refuses fleet dashboards)

The operator's telemetry stack runs per-W&B-CR — each deployment has its own VictoriaMetrics and Grafana, and only sees its own data. This skill is intentionally narrow: it converts **per-service / per-component** DataDog dashboards (one tenant, one service) into a Grafana dashboard for that per-CR Grafana.

It explicitly refuses **fleet / multi-tenant** DataDog dashboards (anything sharded across `$customer-ns`, `$tenant`, `$workspace`, or similar) — see Step 0 for the detection logic and refusal message. A converted fleet dashboard would show empty panels inside a single CR's Grafana because the cross-tenant data simply isn't there. Fleet observability needs a separate centrally-deployed Grafana federating across CRs and is out of scope here.

## Inputs to collect

If the user hasn't provided these, ask once via `AskUserQuestion`:

1. **DataDog dashboard JSON** — accept as a file path, pasted content, or already-parsed object. Validate it contains both `title` and `widgets` keys before proceeding.
2. **Target component name** — kebab-case, single token describing the service or component this dashboard covers (e.g. `api-deep-dive`, `mysql-deep-dive`, `kafka-deep-dive`). This becomes both the UID suffix (`wandb-<component>`) and the filename (`wandb-<component>.json`). If the user gives a multi-word phrase, propose a slug and confirm.
3. **Overwrite policy** — if `deploy/telemetry/dashboards/wandb-<component>.json` already exists, confirm before overwriting.

## Procedure

Run these steps sequentially. Don't parallelize — later steps assume the output of earlier ones.

### Step 0 — Classify the DataDog source (per-CR vs. fleet)

Before any conversion work, decide whether the source is per-CR (proceed) or fleet/multi-tenant (refuse). This guards against producing a dashboard that would show empty panels inside a per-CR Grafana.

The source is **fleet/multi-tenant** if ANY of the following are true:

1. `template_variables[]` contains an entry whose `name` (case-insensitive) matches: `customer-ns`, `customer_ns`, `customer`, `tenant`, `workspace`, `cluster`, `account`, `org`, `organization`, or starts with `excluded-` / `excluded_`.
2. Any widget query groups by `customer-ns`, `customer_ns`, `tenant`, or `workspace` (the grouping label IS a tenancy identifier). Note: grouping by `kube_namespace` is fine — within a single CR there can be legitimate multi-namespace views.
3. The dashboard `title` contains "Fleet", "MI" (multi-instance), "All Customers", "Tenancy", or "Cross-tenant" (case-insensitive substring match).

If any of those fire, **stop** and print this exact refusal message (substitute the detected variables / labels into `<list>`):

```
This DataDog dashboard appears to be fleet/multi-tenant — it references
tenancy variables (<list>) or groups by tenant labels. The operator's
telemetry stack runs per-W&B-CR (single-tenant), so a converted version
inside one CR's Grafana would never have the cross-tenant data it needs.

Options:
  1. Pick a per-service / per-component DD dashboard instead.
  2. Strip tenancy variables and groupings from the DD source first, then re-run.
  3. Pursue fleet observability separately (a centrally-deployed Grafana
     federating across CRs — out of scope for this skill).
```

Do not write any file. Do not proceed to Step 1. The user can either pick a different DD source or strip the tenancy bits from this one and re-run.

If none of the rules fire, the source is per-CR — continue to Step 1.

### Step 1 — Read the canonical references

Read `deploy/telemetry/dashboards/wandb-telemetry-overview.json` and `deploy/telemetry/dashboards/wandb-mysql.json` to refresh the exact shape of the target JSON (especially `__inputs`, panel structures, and `gridPos` conventions). Do not skip this — schema 39 details change subtly across Grafana releases.

### Step 2 — Build the skeleton

Use this skeleton, filling in `<component>` and the title:

```json
{
  "__inputs": [
    {
      "name": "DS_VICTORIAMETRICS",
      "label": "VictoriaMetrics",
      "type": "datasource",
      "pluginId": "victoriametrics-metrics-datasource",
      "pluginName": "VictoriaMetrics"
    }
  ],
  "annotations": { "list": [] },
  "editable": true,
  "fiscalYearStartMonth": 0,
  "graphTooltip": 1,
  "links": [],
  "panels": [],
  "refresh": "30s",
  "schemaVersion": 39,
  "style": "dark",
  "tags": ["wandb", "<component>", "observability"],
  "templating": { "list": [] },
  "time": { "from": "now-6h", "to": "now" },
  "timezone": "browser",
  "title": "W&B <Component Title-Cased>",
  "uid": "wandb-<component>",
  "version": 1
}
```

**Datasource inclusion rule:** add `DS_VICTORIALOGS` to `__inputs` only if at least one converted panel queries logs (DD `data_source: "logs"` or `data_source: "logs_pattern_stream"`). Add `DS_VICTORIATRACES` only if a panel references traces or you preserve a "Open Traces in Explore" link. Mirror the per-dashboard pattern: `wandb-mysql.json` declares Metrics + Logs only; `wandb-telemetry-overview.json` declares Metrics + Traces only. The Helm wiring (`deploy/telemetry/templates/telemetry-ui.yaml`) passes all three regardless, so declaring fewer is safe.

### Step 3 — Walk the widget tree

DataDog `widgets` is a flat array of objects, but `definition.type == "group"` widgets contain a nested `widgets` array. Emit a Grafana `row` panel per group, then recurse into its children. For each leaf widget look up the type in `references/widget-mapping.md` and build the matching Grafana panel.

### Step 4 — Translate queries

For each `definition.requests[].queries[]` and `definition.requests[].formulas[]`, apply the rules in `references/query-mapping.md`. Drop DD template variables (`$customer-ns`, `$env`, `$cloud_provider`, `$size`, etc.) from filter sets — do not map them to Grafana variables; we follow the no-templating convention.

Per-query routing — substitutions take precedence over TODOs:

1. **Syntactic translation** — apply the aggregator/grouping rewrite, suffix functions (`.as_rate`, `.rollup`, `.fill`), formula wrappers (`monotonic_diff`, `default_zero`, `per_second`, etc.), filter-token drops for `$<var>`, and dot→underscore metric name conversion. This gives you the literal-translated metric name (e.g. DD `gcp.redis.clients.connected` → `gcp_redis_clients_connected`).
2. **Substitution check** — look the literal-translated metric name up in the "Known substitutions" table in `references/query-mapping.md`.
   - **If found:** swap the metric expression with the substitution's RHS. Apply the label-rename rules (DD `pod_name` → cAdvisor `pod`, etc.) inside `by (...)` groupings and `{...}` filter sets, but ONLY when the RHS is a cAdvisor/kubelet/machine metric. Attach a panel description: `"Substituted from DataDog metric '<literal_translated_name>'."` (so the swap is auditable). Done — do NOT also add a TODO description.
   - **If not found:** continue to step 3.
3. **TODO fallback** — check whether the literal-translated metric's prefix is in the "Unscraped metric prefixes (TODO list)" in `references/query-mapping.md`.
   - **If yes:** keep the literal-translated expression and attach this panel description:
     ```
     TODO: metric '<prefix>' is not currently scraped by the operator telemetry stack. Verify VictoriaMetrics has data before relying on this panel; add a scrape config in deploy/telemetry/templates/telemetry-scrapes.yaml if needed.
     ```
   - **If no:** keep the literal-translated expression unchanged and add no description — the metric is presumably already in our scrape set under the same name (e.g. `mysql_global_status_threads_connected` is queryable as-is).

Keep panels in place in all three cases — the user wants to see what existed in DD even when we can't query it yet. The audit description distinguishes a substituted panel from a pass-through panel.

### Step 5 — Convert layout

DataDog uses a 12-column grid (`{x, y, width, height}` on `widget.layout`). Grafana uses a 24-column grid (`gridPos: {x, y, w, h}`). Multiply DD `x` and `width` by 2; keep `y` and `height` as-is. Row panels span the full grid: `{ "h": 1, "w": 24, "x": 0, "y": <row_y> }`.

### Step 6 — Scrub sensitive content

Apply every rule in `references/scrub-checklist.md` before writing. If any rule fails loudly (e.g. a hardcoded customer namespace survived substitution), stop and report the offending value — do not write the file.

### Step 7 — Output validation

Before writing, validate the in-memory JSON:

- `uid == "wandb-<component>"` and matches the planned filename
- Every panel `datasource.uid` is one of `${DS_VICTORIAMETRICS}`, `${DS_VICTORIALOGS}`, `${DS_VICTORIATRACES}`
- `tags` contains `"wandb"`, `<component>`, `"observability"`
- `templating.list` is empty
- No string in any `expr` contains `$customer-ns`, `$env`, `$cloud_provider`, `$size`, `$kube_namespace`, `$kube_deployment`, `$kube_daemon_set`, `$kube_stateful_set`, `$image_tag`, `$kafka.topic.name`, `$helm_chart_name`, or `$helm_chart_version`

### Step 8 — Write the file

Write to `deploy/telemetry/dashboards/wandb-<component>.json`. Use `jq -S .` (or equivalent) to keep field order deterministic if jq is available; otherwise use a stable JSON serializer with two-space indent so future diffs stay minimal.

### Step 9 — Print the wiring reminder

Output verbatim:

```
Dashboard written to deploy/telemetry/dashboards/wandb-<component>.json

Wire it into the Helm chart manually (this skill does not edit templates):

  1. Open deploy/telemetry/templates/telemetry-ui.yaml
  2. Find the line beginning with:    {{- range $component := list "operator" "api" "mysql"
  3. Add "<component>" to the list (any position is fine)
  4. Verify with:    helm template deploy/telemetry/ --set mode=full | grep "uid: wandb-<component>"
     (--set mode=full is required — the chart's UI templates are gated on telemetry.uiEnabled, which is true only when mode=full. Without the flag, the command renders nothing and the wiring will look broken even when it isn't.)

Then visually inspect the dashboard in Grafana and address any TODO descriptions left on panels whose source metrics aren't yet scraped.
```

Confirm the `range` line exists before printing — if `grep -n 'range \$component' deploy/telemetry/templates/telemetry-ui.yaml` returns nothing, the template has changed and the reminder needs to be re-derived.

## When NOT to use this skill

- The user has a Grafana dashboard already and just wants to edit it — use the file directly.
- The user wants to add panels to an existing dashboard — open the JSON and edit.
- The user wants a new dashboard from scratch (no DD source) — write the Grafana JSON directly, following the skeleton above.
- The DD source uses log analytics, security signals, RUM, or APM features that have no metric/log equivalent in this stack — flag this up front and ask if a partial port is acceptable.

## References (load on demand)

- `references/widget-mapping.md` — DD widget type → Grafana panel type table
- `references/query-mapping.md` — DD query function → MetricsQL rules and the unscraped-prefix list
- `references/scrub-checklist.md` — sensitive-content patterns
