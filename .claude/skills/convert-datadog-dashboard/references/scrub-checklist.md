# Sensitive-content scrub checklist

Run every rule below before writing the dashboard JSON. Some rules "strip" (modify content silently), some "fail loudly" (stop the conversion and surface the problem to the user). Default to failing loudly — silent stripping is for patterns that are obviously safe to drop (DD-internal URLs in notes).

## Always-strip patterns (silently scrub)

These appear in `note`/`text` widget content, panel descriptions, and link targets. Strip them; if the field becomes empty after stripping, drop the field (or the whole panel for notes).

- Any URL matching `app\.datadoghq\.(com|eu|us\d+)` — DD-internal dashboard links won't resolve outside DD
- Any path matching `/dash/integration/\d+` — DD integration dashboard IDs
- Any path matching `/dashboard/[a-z0-9-]+` when host is `*.datadoghq.*` — DD dashboard slugs
- Markdown link wrappers around stripped URLs: if the link text remains meaningful (e.g. "Kubernetes Pods Overview dashboard"), keep the text and drop just the URL; if not, drop the whole link

## Always-strip from DD template variable defaults

Do not carry DD `template_variables[].defaults` into the Grafana JSON. Specifically:

- `template_variables[].defaults` may list real customer namespace names — never preserve these in any form (not as a comment, not as a default filter, not as a panel description)
- The `excluded-customer-ns` variable in the example DD source lists names like `wandb-annirudh`, `wandb-zalando`, `wb-sdd-4020c451`, `wandb-fe-crew`, `wandb-mademoiselle`, `wandb-gcpdaniel`, and various `wandb-*-perf-*` tenants. These are sensitive — they identify specific customers and internal tenants. They must NOT appear anywhere in the output.

Since the converted dashboard uses no templating, the right behavior is to drop the entire `template_variables` array — but the literal customer names may still appear inline in `widget.definition.requests[].queries[].query` strings (e.g. as part of `customer-ns:wandb-foo` filter tokens). Those need fail-loudly handling (next section).

## Fail-loudly patterns (stop the conversion)

If any of these survive substitution and would appear in the final JSON, stop and report the offending value. Do not write the file.

- Any hardcoded customer namespace literal in a query filter, alias, title, or formula. Detect with: regex match against the names listed above, plus any string matching `(wandb|wb)-[a-z0-9]+` that ALSO appears in the DD source's `template_variables` defaults (this catches customer-ns names you might not have seen before — if a name appears in a defaults list AND inline in a query, that's a customer identifier).
- Internal corporate hostnames matching `*.internal`, `*.corp.*`, or `*.private.*`
- Any IP address in private ranges (`10.*`, `172.16-31.*`, `192.168.*`) hardcoded in a query or note
- Any string matching `(eks|gke|aks)_cluster.name:[^,}]+` with a non-empty cluster name — cluster names often double as tenant identifiers
- Bearer-style tokens, API keys: anything matching `[A-Za-z0-9]{32,}` that appears in a `link.url` or note content (likely a session token or API key embedded by mistake)

When you fail loudly, surface the panel title, the field that contains the offending value, and a snippet of the surrounding text. Example output:

```
SCRUB CHECK FAILED: panel "DB CPU Utilization" target A expression contains "customer-ns:wandb-annirudh".
Source DataDog query: max:gcp.cloudsql.database.cpu.utilization{...,customer-ns:wandb-annirudh}
This is a customer-identifying namespace literal and cannot be written to the dashboard JSON.
Resolution: edit the DD source to remove the hardcoded customer-ns filter, or confirm explicitly that this customer name is safe to publish.
```

## DD data sources that drop the panel

Some DD query datasources have no equivalent in this stack. Drop the panel and emit a `text` panel at the same gridPos noting what was removed:

- `data_source: "events"` — DD event search; no equivalent
- `data_source: "rum"` — Real User Monitoring; no equivalent
- `data_source: "security_signals"` — DD security; no equivalent
- `data_source: "audit"` — DD audit logs; no equivalent
- `data_source: "dataset"` with a base64 `dataset_id` — DD-internal DDSQL warehouse query; cannot be translated

Replacement `text` panel:

```json
{
  "type": "text",
  "title": "<original_title>",
  "gridPos": { ...same as source... },
  "options": {
    "mode": "markdown",
    "content": "Original DD widget queried a `<data_source>` source that has no equivalent in this stack. Rewrite manually or drop in a follow-up."
  }
}
```

## Self-test before writing

After all transformations, run these checks against the in-memory JSON:

1. `grep -i "datadoghq" <json>` → no matches
2. `grep -iE "(wandb|wb)-[a-z0-9-]+" <json> | grep -v '"wandb-<component>"' | grep -v '"wandb"'` → no matches (the dashboard's own UID and the `wandb` tag are allowed)
3. `grep -E "\\$[a-z][a-z_-]+" <json>` → no matches outside `${DS_VICTORIAMETRICS}`, `${DS_VICTORIALOGS}`, `${DS_VICTORIATRACES}`, `$__rate_interval`, `$__interval` (these are Grafana built-ins and are fine)

If any check fails, stop and surface the matching line.
