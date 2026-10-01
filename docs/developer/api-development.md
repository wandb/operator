# Develop APIs and preserve compatibility

Edit source types under [api](../../api), then regenerate using the
[development workflow](workflow.md). The v2 W&B API is the storage/hub version;
v1 conversion logic lives under [api/v1](../../api/v1).

## Adding a field

1. Define the JSON shape and optionality in the Go API type.
2. Add appropriate schema markers, webhook defaults, and validation. Distinguish
   a value that is omitted from a value explicitly supplied by the user.
3. Trace how the reconciler consumes the field and reports status.
4. Check v1/v2 conversion and existing persisted objects when changing an
   established field or representation.
5. Cover defaulting, invalid values, updates, and relevant conversion round trips
   in the API/webhook tests.
6. Regenerate, test, and update the user configuration examples and reference.

Schema acceptance does not replace webhook validation, and a successful
conversion does not establish that reconciliation can access referenced Secrets.

## Connection values

`ValueOrSecret` is shared by connection fields, OIDC, notifications, and proxy
URLs. Its current representation is `value` or `valueFrom.secretKeyRef`. The
defaulter normalizes the legacy `{name, key}` representation. Resolution happens
against Secrets in the W&B namespace. Keep credential values out of error and log
output; the code uses `masq` tags for sensitive fields.

Review the [implemented value-or-secret design](../design/wandb_v2/secret_or_value_connection_fields.md)
for rationale, but use the current API and validator for exact field behavior.
Do not infer that all fields reject sensitive literals merely from design goals;
validation is field-specific.

## Infrastructure instance maps

MySQL, Redis, object storage, and ClickHouse specs and statuses use instance names
as map keys. Validation requires the `default` fallback instance. A change must
preserve per-instance naming, connection resolution, status, and retention.
Kafka retains a single managed spec.

## Legacy configuration

Conversion carries legacy environment and resource overrides under
`spec.wandb.legacyOverrides`. Reconciliation resolves typed migration state and
registered legacy environment mappings before applying workloads. Preserve
precedence between explicit CR fields, migrated values, and manifest defaults.

Relevant implementation files include
[conversion overrides](../../api/v1/weightsandbiases_conversion_overrides.go),
[legacy overrides](../../internal/controller/reconciler/legacy_overrides.go), and
[legacy environment mapping](../../internal/controller/reconciler/legacy_env_mapping.go).
Their [design records](../design/README.md) retain the original proposals and
alternatives; implementation status in historical prose may differ from the code.
