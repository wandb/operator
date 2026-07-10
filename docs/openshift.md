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

## Required: bring your own ingress (object storage recommended)

On OpenShift you **must** supply your own ingress/edge. Bringing your own object
storage is strongly recommended for production but no longer strictly required —
the operator can run managed SeaweedFS under the built-in `anyuid` SCC:

- **Object storage (BYO recommended).** For production, point the CR at an
  external object store (S3, GCS, Azure Blob, or any S3-compatible endpoint you
  run) via `spec.objectStore.externalObjectStore`. See
  [Infrastructure Connection Settings](infra-connection-settings.md). Managed
  SeaweedFS also works on OpenShift: its S3 gateway binds port 80, which needs
  `NET_BIND_SERVICE`, so on OpenShift the operator runs the S3 gateway under the
  built-in `anyuid` SCC (as root) while the other SeaweedFS components stay on
  `restricted-v2`. See [What the OpenShift profile changes](#what-the-openshift-profile-changes).
- **Ingress (BYO required).** Front W&B with the cluster's own edge — an
  OpenShift `Route` or your ingress controller. The bundled frontend nginx does
  not run under `restricted-v2` (see [Limitations](#known-limitations)).

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
- **Object storage (managed SeaweedFS)** pins its S3 gateway to the built-in
  `anyuid` SCC and its other components (master/volume/filer) to `restricted-v2`
  via `openshift.io/required-scc` annotations. The seaweedfs-operator has no
  per-component ServiceAccount field, so all its pods share the namespace
  `default` SA; the operator grants that SA `use` of `anyuid` so the S3 pod runs
  as root and can bind port 80, while the annotations keep the rest non-root.
- **OwnerReferencesPermissionEnforcement** RBAC is rendered so the built-in
  StatefulSet controller (moco PVCs) can set finalizers on the resources it owns
  — OpenShift's admission plugin requires this and it is a no-op on upstream
  Kubernetes.

## Known limitations

| Component | Limitation | Status / workaround |
| --- | --- | --- |
| **Object storage** | Managed SeaweedFS runs its S3 gateway on port 80, which `restricted-v2` blocks (it drops `ALL` caps, stripping `NET_BIND_SERVICE`). | **Handled automatically.** On OpenShift the operator runs the S3 gateway under `anyuid` (as root) and keeps the other components on `restricted-v2`. **BYO still recommended** for production via `spec.objectStore.externalObjectStore`. |
| **Ingress / Frontend (`frontend-nginx`)** | The bundled frontend image runs as a fixed, non-numeric user (`nginx`) that owns `/usr/share/nginx/html` and rewrites files there at startup. `restricted-v2`'s arbitrary UID cannot write, and `nonroot-v2` rejects the pod because the kubelet cannot verify a non-numeric user is non-root. | **BYO ingress required.** Front W&B with your own ingress/route. Optionally, apply the SCC exception below to run the bundled frontend. |
| **`weave`** | The image is published for `amd64` only and segfaults under emulation on `arm64` clusters (e.g. Apple Silicon CRC). | Runs fine on `amd64` clusters; no workaround on `arm64`. |
| **Cluster-scoped install** | The chart creates SCC grants and cluster-scoped RBAC. | Requires `cluster-admin` at install time. |

### Frontend SCC exception (optional)

If you want to run the bundled frontend instead of supplying your own edge,
create an SCC that keeps `restricted-v2`'s guarantees but allows the image's own
user, grant it to the W&B app ServiceAccount, and pin the frontend Deployment to
it. The operator overwrites the pod spec on every reconcile but **merges** pod
template annotations, so the `openshift.io/required-scc` annotation survives.

```yaml
apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: wandb-frontend-anyuid-v2
allowHostDirVolumePlugin: false
allowHostIPC: false
allowHostNetwork: false
allowHostPID: false
allowHostPorts: false
allowPrivilegeEscalation: false
allowPrivilegedContainer: false
allowedCapabilities:
  - NET_BIND_SERVICE
readOnlyRootFilesystem: false
requiredDropCapabilities:
  - ALL
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: MustRunAs
seccompProfiles:
  - runtime/default
fsGroup:
  type: MustRunAs
supplementalGroups:
  type: RunAsAny
volumes:
  - configMap
  - csi
  - downwardAPI
  - emptyDir
  - ephemeral
  - image
  - persistentVolumeClaim
  - projected
  - secret
users:
  - system:serviceaccount:wandb:wandb-app
```

```bash
oc apply -f wandb-frontend-anyuid-v2.yaml
oc -n wandb patch deploy/frontend --type=merge \
  -p '{"spec":{"template":{"metadata":{"annotations":{"openshift.io/required-scc":"wandb-frontend-anyuid-v2"}}}}}'
```

Only the frontend uses this SCC; the SCC has no `priority`, so every other
`wandb-app` pod continues to run under `restricted-v2`.

### Managed SeaweedFS SCC (automatic)

Managed SeaweedFS works on OpenShift without manual SCC steps. Its S3 gateway
binds port 80 with an empty container securityContext, so it needs
`NET_BIND_SERVICE` in its *effective* capability set. No non-root SCC provides
this out of the box: the `-v2` SCCs drop `ALL` capabilities, and the legacy
`restricted`/`nonroot` SCCs keep `NET_BIND_SERVICE` only in the bounding set
(empty effective set for a non-root UID). The only built-in SCC that runs it
unmodified is **`anyuid`**, which runs the image as its default user (root), so
all bounded caps — including `NET_BIND_SERVICE` — are effective.

The seaweedfs-operator (0.1.24) has no per-component ServiceAccount field, so all
its pods share the namespace `default` SA. On OpenShift the operator therefore:

- creates a `RoleBinding` granting the `default` SA `use` of the `anyuid`
  ClusterRole (`system:openshift:scc:anyuid`), scoped to the object-store
  namespace and owned by the `Seaweed` CR;
- annotates the S3 gateway pod with `openshift.io/required-scc: anyuid` so only
  it runs under `anyuid`;
- annotates the master/volume/filer pods with
  `openshift.io/required-scc: restricted-v2` so granting the shared `default` SA
  `anyuid` does not flip them to root.

The S3 gateway runs as root; the rest of SeaweedFS stays non-root. For a fully
non-root object store, bring your own instead.
