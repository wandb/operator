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

This prints only the parent chart's values. Subchart defaults, such as
`telemetry.logAgent` and `telemetry.grafana.pluginImage`, are in the subchart
values files; use `helm pull --untar` and read `charts/telemetry/values.yaml`.

## W&B desired state

| Field under `spec` | Purpose and important behavior |
| --- | --- |
| `size` | Manifest sizing profile; defaults to `dev`. Sizes other than `dev` enable Sentinel for managed Redis, so a managed-Redis resource created at `dev` without Sentinel cannot move to another size |
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
| `wandb.applications` | Per-manifest-application autoscaling and security overrides |
| `wandb.securityProfile` | Shared application/migration non-root and read-only-root flags |
| `wandb.migrations` | Security profiles for named migration Jobs, applied to new Jobs |
| `wandb.probes` | Defaults for generated application health probes |
| `wandb.serviceAccount` | Application workload identity |
| `mysql`, `redis`, `objectStore`, `clickhouse` | Instance maps; an omitted map gets one managed `default` instance, and a `default` key is required once you define any instance |
| `kafka.managedKafka` | Managed Bufstream configuration |
| `networking.mode` | Empty, `ingress`, or `gateway` |
| `adminConsoleEnabled` | Enable Console; disabled when omitted |
| `supportBundle.enabled`, `supportBundle.version` | Install the suspended collection template (enabled by default) and optionally override its collector image tag |
| `global.imageRegistry`, `global.imagePullSecrets` | W&B image registry and shared manifest/workload credentials |
| `global.customCACerts`, `global.caCertsConfigMap` | W&B workload CA trust |
| `global.proxy` | Application HTTP/HTTPS proxy and additional bypass entries |
| `affinity`, `tolerations` | Shared scheduling settings with managed-service overrides |

See [configuration](configuration.md), [infrastructure](infrastructure.md), and
[networking](networking.md) for examples. Connection booleans and ports inside
`value` must be strings. Some fields are immutable after creation: the current
validator rejects changing managed Redis storage size, namespace, or Sentinel
mode (enabled by any size other than `dev`), and rejects reducing an established managed MySQL
replica count.

## Helm settings

| Setting | Purpose |
| --- | --- |
| `wandb-operator.image` | Operator image, also used for Console |
| `wandb-operator.caCerts` | CA trust for Operator egress |
| `moco.enabled`, `redis-operator.enabled`, `seaweedfs-operator.enabled`, `altinity-clickhouse-operator.enabled` | Install backing controllers and their bundled CRDs |
| `openshift.enabled` and the OpenShift profile values | OpenShift-specific security and RBAC configuration |
| `telemetry` | Collection mode, CRDs, forwarding, retention, and runtime settings |
| `telemetry.logAgent` | Pod-log collection, extra namespaces, resources, and tolerations |
| `global.image.registry` | Telemetry image registry prefix |
| `global.imagePullSecrets` | Pull Secrets for the Operator pod, CRD installer Job, and telemetry; separate from the CR's `spec.global` settings |
| `telemetry.grafana.pluginImage` | Optional bundled-plugin image override; defaults to the installed Operator image |
| `victoria-metrics-operator.enabled`, `grafana-operator.enabled` | Telemetry controller dependencies |

## Observed state

`status.ready` and the standard `Ready` condition summarize dependency, migration,
and application readiness. Inspect `conditions[].observedGeneration` to determine
whether the condition applies to the current desired state.

Infrastructure statuses mirror the instance maps in `mysqlStatus`, `redisStatus`,
`objectStoreStatus`, and `clickhouseStatus`; Kafka uses `kafkaStatus`.
`wandb.migration` records migration progress and version. `ingressStatus`,
`gatewayStatus`, `watchtowerStatus`, and `telemetryStatus` report their respective
components. Console readiness is separate from overall W&B readiness. `supportBundle` reports
collection-template readiness, its CronJob, image/version, running state, and last
successful time. See [support bundles](support-bundles.md).

Do not copy `status` or server-populated metadata into your desired-state file.
For Operator release notes use [GitHub Releases](https://github.com/wandb/operator/releases);
the root [changelog](../../CHANGELOG.md) contains v1 history.
