# Deploying on OpenShift

This guide covers installing the W&B operator on an OpenShift Container Platform
(OCP) cluster, the OpenShift-specific configuration the operator needs, and the
known limitations of running under OpenShift's default `restricted-v2` Security
Context Constraint (SCC).

For **local** OpenShift development with CRC + Tilt, see
[`config/openshift-dev/README.md`](../config/openshift-dev/README.md) instead —
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
| `OPENSHIFT=true` env on the operator | `profiles/openshift.yaml` | Makes `utils.IsOpenShift()` true, so managed infra specs omit fixed UID/GID and the Kafka pods get a dedicated SA bound to `nonroot-v2`. |
| `openshift.enabled=true` chart value | `profiles/openshift.yaml` | Renders the OpenShift-only RBAC/SCC templates (`openshift-owner-finalizers-rbac.yaml`, `openshift-scc-rbac.yaml`). |

Both are set for you by the `profiles/openshift.yaml` values overlay.

## Prerequisites

- OpenShift 4.x cluster.
- `cluster-admin` (or equivalent) for the install: the chart creates
  cluster-scoped RBAC and SCC grants.
- [`helm`](https://github.com/helm/helm) 3.x and [`oc`](https://formulae.brew.sh/formula/openshift-cli).
- A W&B server version and (for production) a container image registry the
  cluster can pull from.

## Known limitations

| Component | Limitation | Status / workaround |
| --- | --- | --- |
| **Object storage** | Managed SeaweedFS runs its S3 gateway on port 80, which `restricted-v2` blocks (it drops `ALL` caps, stripping `NET_BIND_SERVICE`). | **BYO required.** Use an external object store via `spec.objectStore.externalObjectStore`. |
| **Ingress / Frontend (`frontend-nginx`)** | The bundled frontend image runs as a fixed, non-numeric user (`nginx`) that owns `/usr/share/nginx/html` and rewrites files there at startup. `restricted-v2`'s arbitrary UID cannot write, and `nonroot-v2` rejects the pod because the kubelet cannot verify a non-numeric user is non-root. | **BYO ingress required.** Front W&B with your own ingress/route. |
| **Cluster-scoped install** | The chart creates SCC grants and cluster-scoped RBAC. | Requires `cluster-admin` at install time. |

## Required: bring your own ingress and object storage

On OpenShift you **must** supply your own ingress/edge and your own object
storage. The bundled frontend and managed SeaweedFS do not run under OpenShift's
`restricted-v2` SCC (see [Known limitations](#known-limitations)).

- **Object storage (BYO required).** Point the CR at an external object store
  (S3, GCS, Azure Blob, or any S3-compatible endpoint you run) via
  `spec.objectStore.externalObjectStore`. See
  [Infrastructure Connection Settings](infra-connection-settings.md).
- **Ingress (BYO required).** Front W&B with the cluster's own edge — an
  OpenShift `Route` or your ingress controller.

The rest of the managed infra (MySQL, Redis, ClickHouse, Kafka) is supported on
OpenShift via the adaptations described below.

## Deploying

### 1. Install the operator with the OpenShift profile

From a checkout of this repository:

```bash
helm install wandb-operator ./deploy/operator \
  --namespace wandb-operators --create-namespace \
  -f deploy/operator/profiles/openshift.yaml
```

Installing from the published OCI chart works the same way, but you must supply
the OpenShift values yourself (the `-f` file must be a local path). Save the
snippet below as `openshift-values.yaml`:

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
  --namespace wandb-operators --create-namespace \
  -f openshift-values.yaml
```

### 2. Apply a `WeightsAndBiases` resource

The CR must reference an external object store (BYO is required on OpenShift).
Provide the connection details in a Secret and reference its keys:

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
  networking:
    mode: ingress
  objectStore:
    externalObjectStore:
      bucket:    { name: wandb-object-store, key: bucket }
      region:    { name: wandb-object-store, key: region }
      accessKey: { name: wandb-object-store, key: accessKey }
      secretKey: { name: wandb-object-store, key: secretKey }
      # endpoint: { name: wandb-object-store, key: endpoint }
      # port:     { name: wandb-object-store, key: port }
```

```bash
kubectl apply -f wandb.yaml
```

On OpenShift you must front the deployment with the cluster's own edge — an
OpenShift `Route` or your ingress controller — not a bundled load balancer or
the bundled frontend. See
[`docs/infra-connection-settings.md`](infra-connection-settings.md) for
networking and object-store connection options.

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
