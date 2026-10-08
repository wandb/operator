# Deploying on OpenShift

This guide covers installing the W&B operator on an OpenShift Container Platform
(OCP) cluster, the OpenShift-specific configuration the operator needs, and the
known limitations of running under OpenShift's default `restricted-v2` Security
Context Constraint (SCC).

For **local** OpenShift development with CRC + Tilt, see
[`config/openshift-dev/README.md`](../../config/openshift-dev/README.md) instead —
that path is automated and does not require the manual steps below.

## Why OpenShift needs special handling

OpenShift admits every pod through an SCC. The default `restricted-v2` SCC is
stricter than upstream Kubernetes defaults: it assigns each pod an **arbitrary
UID** from the namespace's `openshift.io/sa.scc.uid-range`, forbids running as a
fixed UID, drops all capabilities, disallows privileged ports (`<1024`), and
requires a `runtime/default` seccomp profile.

Several components the operator manages ship images that assume a fixed UID or a
privileged port, so they must be adapted. The operator does this automatically
when it knows it is running on OpenShift, driven by two switches:

| Switch | Where | Effect |
| --- | --- | --- |
| `OPENSHIFT=true` env on the operator | OpenShift values file | Makes managed infrastructure omit fixed UID/GID and gives Kafka a dedicated ServiceAccount bound to `nonroot-v2`. |
| `openshift.enabled=true` chart value | OpenShift values file | Enables the OpenShift-specific RBAC and SCC grants. |

Both are included in the values file below and the chart's OpenShift profile.

## Prerequisites

Complete the shared [deployment prerequisites](planning.md), including
cert-manager and persistent storage. Create the W&B namespace before its Secrets
and resource, as in [installation](installation.md).


- OpenShift 4.x cluster.
- `cluster-admin` (or equivalent) for the install: the chart creates
  cluster-scoped RBAC and SCC grants.
- [`helm`](https://github.com/helm/helm) 3.x and [`oc`](https://formulae.brew.sh/formula/openshift-cli).
- A W&B server version and (for production) a container image registry the
  cluster can pull from.

## Known limitations

The frontend constraints below are inherited from the existing OpenShift guide
and depend on the W&B image version. Revalidate them for your selected release;
a tested OpenShift Route recipe is still tracked in the
[documentation backlog](../developer/documentation.md#remaining-coverage).


| Component | Limitation | Status / workaround |
| --- | --- | --- |
| **Ingress / Frontend (`frontend-nginx`)** | The bundled frontend image runs as a fixed, non-numeric user (`nginx`) that owns `/usr/share/nginx/html` and rewrites files there at startup. `restricted-v2`'s arbitrary UID cannot write, and `nonroot-v2` rejects the pod because the kubelet cannot verify a non-numeric user is non-root. | **BYO ingress required.** Front W&B with your own ingress/route. |
| **Kafka (bufstream)** | The distroless broker image ships a `0700` binary owned by a fixed UID (65532) that can only be executed as that exact user, so it cannot run under `restricted-v2`. | Currently runs under `nonroot-v2` (via a dedicated ServiceAccount) rather than `restricted-v2`. |
| **Cluster-scoped install** | The chart creates SCC grants and cluster-scoped RBAC. | Requires `cluster-admin` at install time. |

## Required: bring your own ingress

On OpenShift you **must** supply your own ingress/edge. The bundled frontend does
not run under OpenShift's `restricted-v2` SCC (see
[Known limitations](#known-limitations)).

- **Ingress (BYO required).** Front W&B with the cluster's own edge — an
  OpenShift `Route` or your ingress controller.
- **Object storage (optional BYO).** Managed SeaweedFS runs under `restricted-v2`
  (its S3 gateway binds an unprivileged port, so no root/`anyuid` grant is
  needed). You can still point the CR at an external object store (S3, GCS, Azure
  Blob, or any S3-compatible endpoint you run) via
  `spec.objectStore.default.externalObjectStore` if you prefer. See
  [Infrastructure Connection Settings](infrastructure.md).

The rest of the managed infra (MySQL, Redis, ClickHouse, Kafka) is supported on
OpenShift via the adaptations described below.

## Deploying

### 1. Install the operator with the OpenShift profile

Set `OPERATOR_VERSION` to the published v2 chart version you selected. Save the
following as `openshift-values.yaml` and merge in your other chart settings:

```yaml
openshift:
  enabled: true
wandb-operator:
  podSecurityContext:
    runAsUser: null
    runAsGroup: null
    fsGroup: null
    fsGroupChangePolicy: null
  containers:
    operator:
      env:
        OPENSHIFT:
          value: "true"
redis-operator:
  podSecurityContext: { runAsUser: null, runAsGroup: null, fsGroup: null, fsGroupChangePolicy: null }
altinity-clickhouse-operator:
  podSecurityContext: { runAsUser: null, runAsGroup: null, fsGroup: null, fsGroupChangePolicy: null }
seaweedfs-operator:
  podSecurityContext: { runAsUser: null, runAsGroup: null, fsGroup: null }
moco:
  extraArgs:
    - --disable-default-security-context
grafana-operator:
  isOpenShift: true
```

```bash
helm install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version "$OPERATOR_VERSION" \
  --namespace wandb-operators --create-namespace \
  -f openshift-values.yaml
```

### 2. Apply a `WeightsAndBiases` resource

Object storage can be managed (SeaweedFS runs under `restricted-v2`) or external.
The example below wires an external object store; omit the `objectStore` block to
use the managed default. When bringing your own, provide the connection details
in a Secret and reference its keys:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: wandb-object-store
  namespace: wandb
stringData:
  bucket: my-wandb-bucket
  region: us-east-1
  accessKey: <access-key>
  secretKey: <secret-key>
  # endpoint/port only for non-AWS, S3-compatible stores (e.g. MinIO)
  # endpoint: minio.example.com
  # port: "9000"
---
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: wandb
  namespace: wandb
spec:
  size: small
  retentionPolicy:
    onDelete: detach
  wandb:
    version: <wandb-version>
    hostname: https://wandb.example.com
  networking:
    mode: ingress
    ingress:
      ingressClassName: <ingress-class-name>
      managed: false
  objectStore:
    default:
      externalObjectStore:
        provider: {value: s3}
        bucket: {valueFrom: {secretKeyRef: {name: wandb-object-store, key: bucket}}}
        region: {valueFrom: {secretKeyRef: {name: wandb-object-store, key: region}}}
        accessKey: {valueFrom: {secretKeyRef: {name: wandb-object-store, key: accessKey}}}
        secretKey: {valueFrom: {secretKeyRef: {name: wandb-object-store, key: secretKey}}}
        tlsEnabled: {value: "true"}
        forcePathStyle: {value: "false"}
```

```bash
kubectl apply -f wandb.yaml
```

On OpenShift you must front the deployment with the cluster's own edge — an
OpenShift `Route` or your ingress controller — not a bundled load balancer or
the bundled frontend. See [networking](networking.md) for routing and
[infrastructure](infrastructure.md) for object-store connections.

### 3. Verify

```bash
oc get pods -n wandb
```

Every managed pod should reach `Running`. You can confirm the SCC each pod was
admitted under with:

```bash
oc get pods -n wandb \
  -o custom-columns=NAME:.metadata.name,SCC:'.metadata.annotations.openshift\.io/scc'
```

Managed infra pods run under `restricted-v2`, except the Kafka broker/etcd pods,
which run under `nonroot-v2` (see below).

## What the OpenShift profile changes

- **wandb-operator, crd-installer, and the dependency operators** (redis,
  Altinity ClickHouse, SeaweedFS) drop their hardcoded `runAsUser`/`runAsGroup`/
  `fsGroup`, so OpenShift assigns compliant IDs at admission.
- **MySQL (moco)** runs the controller with `--disable-default-security-context`
  so it does not inject a fixed UID/GID (10000) that `restricted-v2` rejects.
- **Kafka (bufstream)** gets a dedicated ServiceAccount bound to the
  `nonroot-v2` SCC, because its distroless broker image ships a `0700` binary
  owned by a fixed UID (65532) that can only be executed as that exact user.
- **OwnerReferencesPermissionEnforcement** RBAC is rendered so the built-in
  StatefulSet controller (moco PVCs) can set finalizers on the resources it owns
  — OpenShift's admission plugin requires this and it is a no-op on upstream
  Kubernetes.
