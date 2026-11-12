# Migrating from Operator v1 to v2

Install Operator v2 onto the existing v1 deployment. A conversion webhook
converts the live `WeightsAndBiases` resource to `apps.wandb.com/v2`, and the
v2 reconciler starts immediately. The operator reconnects to the same backing
services without migrating data. MySQL, Redis, and object storage convert to
their external connection specs automatically.

Leave the v1 operator and W&B application Helm releases in place during this
install. Set `wandb.install=false` on the v2 chart so it does not create a
second sample CR.

Operator v2 does not uninstall the v1 Helm releases or modify migration
metadata inside MySQL or ClickHouse. Those actions can destroy data or make
rollback impossible and must remain explicit operator decisions.

`spec.retentionPolicy.onDelete` defaults to `detach`. Conversion does not map
this field. Leave it at `detach` while validating the migration.

## Prerequisites

### cert-manager (required)

The v2 chart renders cert-manager resources (`Certificate` and `Issuer`) to
provision the webhook serving certificate and inject the CA into the operator's
webhooks and CRDs. cert-manager must be installed first — even though Operator
v1 did not require it — otherwise `helm install` fails with:

```text
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
- Helm v3.8.0 or later.
- A default `StorageClass` (managed backing services request PersistentVolumes).

## Before the change window

Conversion runs when Operator v2 is installed. Before then, review the v1
values it will read:

1. Record the v1 operator and W&B Helm release names, namespaces, chart
   versions, and values.
2. Back up MySQL, object storage, ClickHouse, and Redis according to the
   provider's restore procedure, and verify that the backups can be read.
3. Review the values conversion will read. Prefer the `<cr-name>-spec-active`
   Secret's `data.values` when that Secret exists; otherwise conversion uses
   `spec.values`.
4. Pin the W&B server version on the v1 side. Conversion sets
   `spec.wandb.version` from `app.image.tag`, falling back to `api.image.tag`.
   The v2 validating webhook rejects an empty version or `latest`. Confirm
   `global.host` is set; conversion maps it to `spec.wandb.hostname`, which is
   required.
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

## Before `helm upgrade`

Complete these steps before installing Operator v2. Adoption starts as soon as
the v2 manager is up.

1. Confirm cert-manager is installed (see [Prerequisites](#cert-manager-required)).
2. Plan Helm values with **`wandb.install=false`**. The chart default is
   `true`, which creates a sample `WeightsAndBiases` named `wandb` (hostname
   `http://localhost`, managed infra). The same name as an existing CR would
   overwrite desired state; a different name would start a second instance.
3. Disable overlapping component operators if your v1 environment already runs
   one cluster-wide — most commonly a standalone Altinity ClickHouse operator.
   Each managed backing service is provisioned by a component operator that the
   v2 chart installs as a subchart (and whose CRDs its bundled crd-installer
   applies). A conflict looks like:

   ```text
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
4. Leave the v1 operator Helm release installed. Uninstalling it can delete the
   `WeightsAndBiases` CRD and CR. Leave the v1 **W&B application** Helm release
   installed until Redis or other owned dependencies are rehomed.
5. Size the cluster for leftover v1 Deployments **plus** the full v2
   application set. v1 Helm Deployments (`<cr>-app-bc`, `<cr>-console-bc`, and
   similar) keep running while v2 creates differently named Application
   Deployments (`api`, `frontend`, and other manifest apps, including
   Watchtower). If v2 pods stay Pending after node groups reach maximum size,
   add capacity for that overlap.

## Install Operator v2

Pin a reviewed version from the Operator v2 OCI repository. Always pass
`--set wandb.install=false`, and add any `*-operator.enabled=false` toggles
from the table above:

```bash
helm upgrade --install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version <operator-version> \
  --namespace <v2-operator-namespace> \
  --create-namespace \
  --set wandb.install=false
```

The `wandb/operator` chart in `charts.wandb.ai` is Operator v1 and is not an
Operator v2 upgrade source.

The v2 chart installs and upgrades its CRDs with server-side apply. If the CRD
installer Job fails, inspect that Job and resolve the ownership or RBAC error;
do not delete existing CRDs as a recovery shortcut.

Conversion and reconcile start on their own once the manager is up. For
already-external MySQL, Redis, and similar, the infrastructure gate can pass
quickly, so migration Jobs and v2 Applications may appear immediately.

### Inspect and patch the converted resource

After install, inspect the converted resource:

```bash
kubectl -n <wandb-namespace> get wandb.v2.apps.wandb.com <wandb-name> -o yaml
```

Confirm:

- `spec.wandb.hostname` matches the public hostname (`global.host`).
- `spec.wandb.version` is a published pin, not empty or `latest`. Patch it if
  conversion inherited nothing (v1 often left the version to the deployer
  channel).
- Every external infrastructure selector names an existing Secret and key.
- Redis has a non-empty host and port (or a complete URL), not only a
  password.
- Object-store endpoint, bucket, region, TLS, and path-style settings are
  correct.
- ClickHouse landed as expected (see [Known migration caveats](#known-migration-caveats)).

If a field is wrong, patch that field on the live CR.

## Wait for the cutover gates

Applications are gated on infrastructure readiness, MySQL initialization, and
the W&B migration Jobs. Ingress, Gateway, and Watchtower reconcile **before**
that gate, so public routing can move to v2 Service names (`api`, and similar)
while those Services do not exist yet. Do not treat the upgrade as complete
until all of these checks pass:

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
- Legacy v1 Deployments named `<cr>-app-bc`, `<cr>-console-bc`,
  `<cr>-executor-bc`, `<cr>-parquet-bc`, and `<cr>-weave-bc` are gone. The
  operator deletes those only after every desired v2 Deployment is healthy.
  Other v1 chart resources (Services, ConfigMaps, Redis StatefulSets, nginx)
  are not deleted by this path.

If v2 pods are Pending after node groups have reached maximum size, add
capacity for the overlap window.

If a migration Job fails, collect its status and logs:

```bash
kubectl -n <wandb-namespace> describe job/<migration-job>
kubectl -n <wandb-namespace> logs job/<migration-job> -c migrate
```

Do not infer migration success from a similarly named Deployment. Do not
automatically clear `partially_applied_version` or edit migration tables: that
requires a migration-specific recovery decision and a verified backup.

## Known migration caveats

- **v1 and v2 workloads overlap.** v2 Applications use manifest names (`api`,
  `frontend`, …). v1 Helm left Deployments named `<cr>-*-bc`. Both run until
  every desired v2 Deployment is rolled out; then only the five `*-bc`
  suffixes above are deleted. Size the cluster for leftover v1 Deployments
  plus the full v2 application set (often more apps than v1, including
  Watchtower).
- **Ingress can retarget before v2 Services exist.** If conversion mapped
  `ingress.nameOverride`, v2 updates that existing Ingress to v2 Service
  backends. Watchtower is also created before infrastructure is ready.
- **ClickHouse must land as external.** If your v1 deployment used an external
  (for example weave-trace) ClickHouse, confirm the converted CR resolves it to
  `spec.clickhouse.default.externalClickhouse` rather than a managed instance.
  Conversion only sets `externalClickhouse` when connection fields are present;
  `install: true` with no host or port leaves ClickHouse empty, and the
  defaulter then inserts a managed instance. If the status shows a managed
  ClickHouse (`ClickHouseConnectionInfo: NoResource`), patch the live CR to
  null `managedClickhouse` and set `externalClickhouse` pointing at your
  cluster via a connection secret.
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
