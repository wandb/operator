# DataDog widget → Grafana panel mapping

Each row tells you which Grafana `type` to emit and the field-level translations that matter. Field names not listed should be dropped — Grafana defaults are fine.

## Type lookup

| DataDog `definition.type` | Grafana panel `type` | Notes |
|---|---|---|
| `timeseries` | `timeseries` | Map `requests[].queries[]` → `targets[]` (one per query) using refIds A, B, C…; map `requests[].formulas[]` → additional `targets[]` with `expr` built from the formula after query substitution. |
| `toplist` | `bargauge` | Drop `conditional_formats` (use Grafana thresholds via `fieldConfig.defaults.thresholds` instead, only if the user wants them). Map `sort.order_by[0].order` → `options.sortBy`. |
| `query_table` | `table` | Per-formula `cell_display_mode: "bar"` → field override with `custom.cellOptions.type: "gauge"`. |
| `treemap` | `barchart` | No native treemap in core Grafana. Set `options.orientation: "horizontal"` and add a panel description: `"Source DD widget was a treemap; rendered as a horizontal bar chart."`. |
| `sunburst` | `piechart` | Set `options.displayMode: "pie"`, `options.legend.displayMode: "table"`. Sunburst's nested groupings flatten — emit only the top group. |
| `distribution` | `histogram` | Single-formula histograms only. For multi-formula distributions, keep `histogram` but add a TODO description listing the dropped formulas. |
| `note` | `text` | Set `options.mode: "markdown"`, `options.content` to the scrubbed note text. Run scrub-checklist BEFORE writing. If content becomes empty after scrubbing, drop the panel entirely. |
| `list_stream` | `logs` | Datasource → `${DS_VICTORIALOGS}`. DD log-pattern stream queries don't map cleanly — emit a `logs` panel with an empty query and a TODO description: `"Source DD widget queried <data_source>; rewrite as a LogsQL expression."`. |
| `group` | `row` | Emit `{ "type": "row", "title": <group.title>, "gridPos": { "h": 1, "w": 24, "x": 0, "y": <y> }, "collapsed": false }`, then recurse into `definition.widgets`. Increment the outer `y` cursor past the row's children. |
| `manage_status`, `alert_graph`, `service_summary`, `service_map`, `slo`, `slo_list`, `geomap`, `image`, `iframe`, `free_text`, `event_stream`, `event_timeline`, `change`, `topology_map`, `funnel`, `wildcard` | (skip with note) | No reasonable equivalent in this stack. Emit a `text` panel at the same gridPos with `options.content: "Source DD widget type '<dd_type>' has no Grafana equivalent in this stack. Original title: <title>"`. |

## Common panel structure (apply to all metric panels)

```json
{
  "type": "<grafana_type>",
  "title": "<dd_title>",
  "datasource": {
    "type": "victoriametrics-metrics-datasource",
    "uid": "${DS_VICTORIAMETRICS}"
  },
  "gridPos": { "h": <h>, "w": <w*2>, "x": <x*2>, "y": <y> },
  "targets": [
    { "expr": "<translated_query>", "refId": "A", "legendFormat": "<alias_or_empty>" }
  ],
  "fieldConfig": { "defaults": {}, "overrides": [] },
  "options": {}
}
```

`legendFormat` comes from DD `formulas[].alias` when present. If a formula has no alias, omit `legendFormat`.

## Per-type field details

### `timeseries`

- `definition.display_type: "line"` → leave `fieldConfig.defaults.custom` empty (line is the default)
- `definition.display_type: "area"` → set `fieldConfig.defaults.custom.fillOpacity: 30`
- `definition.display_type: "bars"` → set `fieldConfig.defaults.custom.drawStyle: "bars"`
- `definition.show_legend: true` → set `options.legend.showLegend: true` (default), pull columns from `legend_columns`
- `definition.yaxis.min: "0"` → set `fieldConfig.defaults.min: 0`
- `definition.yaxis.scale: "log"` → `fieldConfig.defaults.custom.scaleDistribution: { "type": "log", "log": 10 }`
- `definition.yaxis.scale: "sqrt"` → no clean Grafana equivalent; leave linear and add a panel description

### `query_table`

- For each formula with `cell_display_mode: "bar"`, emit a per-field override:
  ```json
  {
    "matcher": { "id": "byName", "options": "<formula_alias_or_value>" },
    "properties": [
      { "id": "custom.cellOptions", "value": { "type": "gauge" } }
    ]
  }
  ```
- DD `query_table` with `data_source: "dataset"` (DDSQL queries against DD's internal warehouse) — drop the panel entirely with a TODO note in a `text` panel at the same gridPos; the base64 dataset IDs are DD-internal and won't translate.

### `note`

- `definition.background_color` → ignore (Grafana text panels don't theme per-panel cleanly)
- `definition.content` → run through scrub checklist, then assign to `options.content` with `options.mode: "markdown"`
- If content becomes empty or contains only stripped patterns, drop the panel

### `toplist`

- `definition.requests[].sort.count` → `options.maxItems` (cap at 50 even if DD specified higher)
- `definition.requests[].sort.order_by[0].order: "desc"` → `options.sortBy: "Last *"`, `options.sortDir: "desc"`

### `group`

- Always emit `collapsed: false` (matches existing wandb-* dashboards)
- The row's children inherit the same y-cursor accounting as top-level panels
- DD `background_color` on the group → ignore

## Layout coordinates

DD `widget.layout = {x, y, width, height}` on a 12-column grid → Grafana `gridPos = {x: x*2, y: y, w: width*2, h: height}` on a 24-column grid.

Group children carry their own local layout coordinates that DD already computes relative to the group's origin. Treat them as already-absolute within Grafana's grid (DD does the same — verify by inspecting `wandb-telemetry-overview` DD source if you have it).
