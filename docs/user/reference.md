# Configuration and status reference

This is an index of the main v2 configuration surfaces, not an exhaustive
field-by-field schema. Read the schema installed in your cluster for the release
you are running:

```bash
kubectl explain weightsandbiases.spec --api-version=apps.wandb.com/v2
kubectl explain weightsandbiases.spec --api-version=apps.wandb.com/v2 --recursive
kubectl explain weightsandbiases.spec.mysql --api-version=apps.wandb.com/v2
kubectl explain weightsandbiases.status --api-version=apps.wandb.com/v2
```

For chart settings, use the exact selected version:

```bash
helm show values \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version "$OPERATOR_VERSION"
```

## W&B desired state

| Field under `spec` | Purpose and important behavior |
| --- | --- |
| `size` | Manifest sizing profile; defaults to `dev` |
| `requireLimits` | Include resource limits in generated workload sizing |
| `retentionPolicy.onDelete` | `detach` by default; `purge` requests infrastructure deletion |
| `wandb.hostname` | Required public W&B URL |
| `wandb.version` | Required pinned W&B version; `latest` is rejected |
| `wandb.license` | License string; not a value-or-secret object |
| `wandb.manifestRepository` | OCI manifest location; derived from the image registry when initially omitted |
| `wandb.features` | Feature flags defined by the selected manifest |
| `wandb.oidc` | Application OIDC connection settings |
| `wandb.notifications` | Email and Slack settings |
| `wandb.security` | Supported application security flags |
| `wandb.retention` | Application data-retention settings; distinct from infrastructure deletion policy |
| `wandb.applications` | Per-manifest-application autoscaling overrides |
| `wandb.probes` | Defaults for generated application health probes |
| `wandb.serviceAccount` | Application workload identity |
| `mysql`, `redis`, `objectStore`, `clickhouse` | Instance maps requiring a `default` entry |
| `kafka.managedKafka` | Managed Bufstream configuration |
| `networking.mode` | Empty, `ingress`, or `gateway` |
| `adminConsoleEnabled` | Enable Console; disabled when omitted |
| `global.imageRegistry`, `global.imagePullSecrets` | W&B image registry and shared manifest/workload credentials |
| `global.customCACerts`, `global.caCertsConfigMap` | W&B workload CA trust |
| `global.proxy` | Application HTTP/HTTPS proxy and additional bypass entries |
| `affinity`, `tolerations` | Shared scheduling settings with managed-service overrides |

See [configuration](configuration.md), [infrastructure](infrastructure.md), and
[networking](networking.md) for examples. Connection booleans and ports inside
`value` must be strings. Some fields are immutable after creation: the current
validator rejects changing managed Redis storage size, namespace, or Sentinel
mode, and rejects reducing an established managed MySQL replica count.

## Helm settings

| Setting | Purpose |
| --- | --- |
| `wandb-operator.image` | Operator image, also used for Console |
| `wandb-operator.caCerts` | CA trust for Operator egress |
| `moco.enabled`, `redis-operator.enabled`, `seaweedfs-operator.enabled`, `altinity-clickhouse-operator.enabled` | Install backing controllers and their bundled CRDs |
| `openshift.enabled` and the OpenShift profile values | OpenShift-specific security and RBAC configuration |
| `telemetry` | Collection mode, CRDs, forwarding, retention, and runtime settings |
| `victoria-metrics-operator.enabled`, `grafana-operator.enabled` | Telemetry controller dependencies |

## Observed state

`status.ready` and the standard `Ready` condition summarize dependency, migration,
and application readiness. Inspect `conditions[].observedGeneration` to determine
whether the condition applies to the current desired state.

Infrastructure statuses mirror the instance maps in `mysqlStatus`, `redisStatus`,
`objectStoreStatus`, and `clickhouseStatus`; Kafka uses `kafkaStatus`.
`wandb.migration` records migration progress and version. `ingressStatus`,
`gatewayStatus`, `watchtowerStatus`, and `telemetryStatus` report their respective
components. Console readiness is separate from overall W&B readiness.

Do not copy `status` or server-populated metadata into your desired-state file.
For Operator release notes use [GitHub Releases](https://github.com/wandb/operator/releases);
the root [changelog](../../CHANGELOG.md) contains v1 history.
