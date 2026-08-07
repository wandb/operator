# W&B Operator Helm chart

This chart installs the [Weights & Biases Kubernetes operator](https://github.com/wandb/operator)
(Operator v2) together with the component operators it uses to provision managed
backing services (MySQL, Redis, ClickHouse, object storage) and, optionally, the
telemetry stack. It is published as an OCI artifact:

```
oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator
```

Once the operator is running you drive your deployment with a `WeightsAndBiases`
custom resource; this chart installs the controller, not the W&B application
itself.

## Prerequisites

### cert-manager (required)

The chart renders cert-manager resources (`Certificate` and `Issuer`) to provision
the webhook serving certificate and inject the CA into the operator's webhooks and
CRDs. **cert-manager must be installed first**, otherwise `helm install` fails with:

```
INSTALLATION FAILED: no matches for kind "Certificate"/"Issuer" in version "cert-manager.io/v1" - ensure CRDs are installed first
```

cert-manager is intentionally **not** a dependency of this chart (many clusters
already run it, and it is a cluster-wide singleton). Install it with its CRDs
before installing the operator:

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

## Clean install

```bash
helm install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --namespace wandb-operators --create-namespace
```

Then apply a `WeightsAndBiases` resource describing your deployment:

```yaml
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
  namespace: wandb
spec:
  size: small
  wandb:
    version: <wandb-version>
  networking:
    mode: ingress
```

```bash
kubectl apply -f wandb.yaml
```

## Bring your own component operators

Each managed backing service is provisioned by a component operator that this
chart installs as a subchart (and whose CRDs the bundled crd-installer applies).
If you already run one of these operators cluster-wide, installing it again causes
CRD ownership conflicts, for example:

```
Installation failed: failed to install CRD clickhouseinstallations.clickhouse.altinity.com … conflicts with "kubectl": .spec.versions
```

Disable the conflicting component operator so the chart reuses your existing one.
The relevant toggles are:

| Component operator | Value | Default |
| --- | --- | --- |
| MySQL (Moco) | `moco.enabled` | `true` |
| Redis | `redis-operator.enabled` | `true` |
| Object storage (SeaweedFS) | `seaweedfs-operator.enabled` | `true` |
| ClickHouse (Altinity) | `altinity-clickhouse-operator.enabled` | `true` |
| VictoriaMetrics (telemetry) | `victoria-metrics-operator.enabled` | `false` |
| Grafana (telemetry) | `grafana-operator.enabled` | `false` |

For example, to keep your existing standalone Altinity ClickHouse operator and its
CRDs:

```bash
helm install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --namespace wandb-operators --create-namespace \
  --set altinity-clickhouse-operator.enabled=false
```

Disabling a component operator also drops its CRDs from the bundled crd-installer,
so the chart will not fight your existing installation. When a component operator
is disabled, provision that backing service externally and point the
`WeightsAndBiases` CR at it (see
[Infrastructure Connection Settings](../../docs/infra-connection-settings.md)).

> Note: the `wandb-operator.operators.*` keys in `values.yaml` are **not**
> consumed by the chart. Use the `*-operator.enabled` / `<name>.enabled` toggles
> above to enable or disable component operators.

## Upgrading from Operator v1

The v2 operator adopts an existing v1 `WeightsAndBiases` CR in place: a conversion
webhook converts the v1 resource to v2, and the operator reconnects to the same
backing services (no data migration). When upgrading with this Helm chart
directly:

1. **Install cert-manager first** (see [Prerequisites](#cert-manager-required)).
   The v2 chart requires it even though v1 did not.
2. **Disable any component operators whose CRDs or instances you already run**,
   using the toggles in
   [Bring your own component operators](#bring-your-own-component-operators). This
   is common in v1 environments that ran a standalone ClickHouse/Altinity operator
   (`--set altinity-clickhouse-operator.enabled=false`).
3. **Install the v2 operator chart** into your operators namespace. The existing
   `WeightsAndBiases` CR is adopted and converted automatically.
4. **Verify external backends reconnected.** MySQL, Redis, and object storage
   convert to their external connection specs automatically. After reconcile,
   confirm each backing service reports `Available` in the CR status.

### Known migration caveats

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

## Configuration

The full set of options lives in
[`values.yaml`](values.yaml). Provide Helm only the values you need and let the
chart supply the rest; do not copy the whole file. Deployment-specific guidance:

- [Configuration API](../../docs/config-api.md)
- [Infrastructure Connection Settings](../../docs/infra-connection-settings.md)
- [Monitoring and Telemetry Guide](../../docs/monitoring.md)
- [Deploying on OpenShift](../../docs/openshift.md)
