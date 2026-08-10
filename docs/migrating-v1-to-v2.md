# Migrating from Operator v1 to v2

Use this runbook for an existing W&B deployment managed by Operator v1. Treat
the migration as a controlled cutover: preserve stateful dependencies, run one
operator controller at a time, and keep a tested rollback path until v2 is
healthy.

Operator v2 adopts an existing v1 `WeightsAndBiases` resource in place: a
conversion webhook converts it to `apps.wandb.com/v2`, and the operator
reconnects to the same backing services without migrating data. MySQL, Redis,
and object storage convert to their external connection specs automatically.

Operator v2 does not uninstall the v1 Helm releases or modify migration
metadata inside MySQL or ClickHouse. Those actions can destroy data or make
rollback impossible and must remain explicit operator decisions.

## Prerequisites

### cert-manager (required)

The v2 chart renders cert-manager resources (`Certificate` and `Issuer`) to
provision the webhook serving certificate and inject the CA into the operator's
webhooks and CRDs. cert-manager must be installed first — even though Operator
v1 did not require it — otherwise `helm install` fails with:

```
INSTALLATION FAILED: no matches for kind "Certificate"/"Issuer" in version "cert-manager.io/v1" - ensure CRDs are installed first
```

cert-manager is intentionally not a chart dependency (many clusters already run
it, and it is a cluster-wide singleton). Install it with its CRDs before the
operator:

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true
```

### Other tooling

- `kubectl` within one minor version of your cluster.
- Helm v3.5.2 or later.
- A default `StorageClass` (managed backing services request PersistentVolumes).

## Before the change window

1. Record the v1 operator and W&B Helm release names, namespaces, chart
   versions, values, and current replica counts.
2. Back up MySQL, object storage, ClickHouse, and Redis according to the
   provider's restore procedure, and verify that the backups can be read.
3. Use `spec.retentionPolicy.onDelete: detach` in the v2 resource while
   validating the migration. Do not delete the v1 resource or uninstall a
   state-owning Helm release as a migration step.
4. Inspect the converted `apps.wandb.com/v2` `WeightsAndBiases` resource before
   applying it:

   - Preserve the public `spec.wandb.hostname`.
   - Verify every external infrastructure selector names an existing Secret
     and key.
   - Verify Redis has a non-empty host and port (or a complete URL), not only a
     password.
   - Verify object-store endpoint, bucket, region, TLS, and path-style settings.
   - Remove `status` and other server-populated metadata from a live-resource
     export before applying it as desired state.

5. Determine whether the v1 W&B Helm release owns Redis or another dependency
   that v2 will continue using:

   ```bash
   helm get manifest <v1-wandb-release> -n <wandb-namespace>
   kubectl get statefulset,service,pvc -n <wandb-namespace>
   ```

   If the release owns Redis, keep that release installed until Redis has been
   migrated to an external service or rehomed outside the release's ownership.
   Pointing v2 at the v1 Redis Service does not make it safe to uninstall the
   v1 release.

## TODO: What needs to happen before applying `helm upgrade`

## Install Operator v2 and apply the resource

Each managed backing service is provisioned by a component operator that the v2
chart installs as a subchart (and whose CRDs its bundled crd-installer applies).
If your v1 environment already runs one of these operators cluster-wide — most
commonly a standalone Altinity ClickHouse operator — disable the overlapping
component operator so the chart reuses your existing one instead of fighting it
for CRD ownership. A conflict looks like:

```
Installation failed: failed to install CRD clickhouseinstallations.clickhouse.altinity.com … conflicts with "kubectl": .spec.versions
```

Disable the conflicting component operator with the matching toggle:

| Component operator | Value | Default |
| --- | --- | --- |
| MySQL (Moco) | `moco.enabled` | `true` |
| Redis | `redis-operator.enabled` | `true` |
| Object storage (SeaweedFS) | `seaweedfs-operator.enabled` | `true` |
| ClickHouse (Altinity) | `altinity-clickhouse-operator.enabled` | `true` |
| VictoriaMetrics (telemetry) | `victoria-metrics-operator.enabled` | `false` |
| Grafana (telemetry) | `grafana-operator.enabled` | `false` |

Disabling a component operator also drops its CRDs from the bundled
crd-installer. Provision that backing service externally and point the
`WeightsAndBiases` CR at it (see
[Infrastructure Connection Settings](infra-connection-settings.md)).

Pin a reviewed version from the Operator v2 OCI repository, adding any
`*-operator.enabled=false` toggles from the table above:

```bash
helm upgrade --install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version <operator-version> \
  --namespace <v2-operator-namespace> \
  --create-namespace

kubectl apply -f <weightsandbiases-v2.yaml>
```

The `wandb/operator` chart in `charts.wandb.ai` is Operator v1 and is not an
Operator v2 upgrade source.

The v2 chart installs and upgrades its CRDs with server-side apply. If the CRD
installer Job fails, inspect that Job and resolve the ownership or RBAC error;
do not delete existing CRDs as a recovery shortcut.

## Wait for the cutover gates

Applications are gated on infrastructure readiness, MySQL initialization, and
the W&B migration Jobs. Do not switch traffic or remove v1 workloads until all
of these checks pass:

```bash
kubectl -n <wandb-namespace> get wandb <wandb-name> -o yaml
kubectl -n <wandb-namespace> get jobs \
  -l app.kubernetes.io/instance=<wandb-name>,app.kubernetes.io/component=migration
kubectl -n <wandb-namespace> get applications
kubectl -n <wandb-namespace> get deployments
```

Confirm:

- Every configured infrastructure status is ready.
- `status.wandb.migration.ready` is `true`, its version matches
  `spec.wandb.version`, and every migration Job succeeded.
- Every v2 application Deployment is fully rolled out with available replicas.
- The public route targets the intended v2 Services and no longer returns
  transient 5xx responses.

If a migration Job fails, collect its status and logs:

```bash
kubectl -n <wandb-namespace> describe job/<migration-job>
kubectl -n <wandb-namespace> logs job/<migration-job> -c migrate
```

Do not infer migration success from a similarly named Deployment. Do not
automatically clear `partially_applied_version` or edit migration tables: that
requires a migration-specific recovery decision and a verified backup.

## Known migration caveats

- **ClickHouse must land as external.** If your v1 deployment used an external
  (for example weave-trace) ClickHouse, confirm the converted CR resolves it to
  `spec.clickhouse.default.externalClickhouse` rather than a managed instance. If
  the status shows a managed ClickHouse (`ClickHouseConnectionInfo: NoResource`),
  patch the CR to null `managedClickhouse` and set `externalClickhouse` pointing
  at your cluster via a connection secret.
- **`weave-worker-auth` token encoding.** If the v1 `weave-worker-auth` secret
  holds a non-UTF-8 (binary) token, weave-trace pods can fail with
  `CreateContainerError: grpc: error while marshaling: string field contains
  invalid UTF-8`. Regenerate the token, overwrite the `weave-worker-auth` secret,
  and restart the weave-trace deployments.

## Smoke test before cleanup

Run the tests through the customer-facing hostname:

1. Sign in and load the main UI.
2. Create an SDK run using an existing, valid entity.
3. Upload and download an artifact through the SDK.
4. Download the artifact in a browser using the presigned URL.
5. Check the browser request for the expected external object-store endpoint,
   trusted TLS, and successful CORS headers.
6. Exercise Runs and Weave views used by the deployment.

An SDK upload alone is not sufficient for S3-compatible storage. Browser
downloads can still fail when the external endpoint or CORS policy is wrong.

## Further reading

- [Configuration API](config-api.md)
- [Infrastructure Connection Settings](infra-connection-settings.md)
- [Monitoring and Telemetry Guide](monitoring.md)
- [Deploying on OpenShift](openshift.md)
