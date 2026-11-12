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
| `OPENSHIFT=true` env on the operator | OpenShift values file | Makes managed infrastructure omit fixed UID/GID and binds Kafka's ServiceAccount to `nonroot-v2`. |
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
| **Frontend (`frontend-nginx`)** | The bundled frontend image runs as a fixed, non-numeric user (`nginx`) that owns `/usr/share/nginx/html` and rewrites files there at startup. `restricted-v2`'s arbitrary UID cannot write, and `nonroot-v2` rejects the pod because the kubelet cannot verify a non-numeric user is non-root. | **Unverified for current images.** Recent server manifests request non-root, read-only-root settings for this app. The frontend Pod runs in every networking mode, so check its SCC admission and startup with your release. |
| **Kafka (bufstream)** | The distroless broker image ships a `0700` binary owned by a fixed UID (65532) that can only be executed as that exact user, so it cannot run under `restricted-v2`. | Currently runs under `nonroot-v2` (via its ServiceAccount's `nonroot-v2` binding) rather than `restricted-v2`. |
| **Pod-log agent** | Node log files need root and hostPath access, which `restricted-v2` blocks. | Disabled by the OpenShift Helm profile; use application OTLP logging or a platform-approved log collector. |
| **SeaweedFS** | The pinned image declares root; non-root runtime and SCC acceptance are not established by the read-only-root changes. | Use a validated image/platform combination or an external object store. |
| **Cluster-scoped install** | The chart creates SCC grants and cluster-scoped RBAC. | Requires `cluster-admin` at install time. |

## Required: bring your own ingress

On OpenShift you **must** supply your own ingress/edge. This is separate from the
frontend runtime note in [Known limitations](#known-limitations): your edge
routes to the frontend Pod, it does not replace it.

- **Ingress (BYO required).** Set `spec.networking.ingress.managed: false` and
  front W&B with the cluster's own edge — an OpenShift `Route` or your ingress
  controller — routing the paths listed in
  [external Ingress](networking.md#external-ingress).
- **Object storage.** Use an external object store unless your managed SeaweedFS
  image and SCC behavior have been validated. Set
  `spec.objectStore.default.externalObjectStore` as described in
  [infrastructure configuration](infrastructure.md).

The adaptations below configure managed infrastructure for OpenShift, but the
actual upstream-generated Pods still require runtime and admission validation
with your selected images. Read-only roots alone do not establish compatibility.

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
telemetry:
  logAgent:
    enabled: false
```

```bash
helm install wandb-operator \
  oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator \
  --version "$OPERATOR_VERSION" \
  --namespace wandb-operators --create-namespace \
  -f openshift-values.yaml
```

### 2. Apply a `WeightsAndBiases` resource

The example uses an external object store. Provide the connection details in a
Secret and reference its keys. Omitting `objectStore` selects managed SeaweedFS;
validate that image's SCC and filesystem behavior before choosing it.

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
OpenShift `Route` or your ingress controller — that routes to the generated
Services. See [networking](networking.md#external-ingress) for the required paths
and [infrastructure](infrastructure.md) for object-store connections.

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

Check each upstream-generated Pod rather than assuming the assigned SCC. Kafka
uses a dedicated `nonroot-v2` grant; validate the other components against your
cluster policy and selected images.

## What the OpenShift profile changes

- **wandb-operator, crd-installer, and the dependency operators** (redis,
  Altinity ClickHouse, SeaweedFS) drop their hardcoded `runAsUser`/`runAsGroup`/
  `fsGroup`, so OpenShift assigns compliant IDs at admission.
- **MySQL (moco)** runs the controller with `--disable-default-security-context`
  so it does not inject a fixed UID/GID (10000) that `restricted-v2` rejects.
- **Kafka (bufstream)** has its ServiceAccount bound to the `nonroot-v2` SCC,
  because its distroless broker image ships a `0700` binary owned by a fixed UID
  (65532) that can only be executed as that exact user. The chart grants the
  Operator `use` of `nonroot-v2` so it can create that binding.
- **Pod-log collection** is disabled because the telemetry log agent requires
  root access and hostPath mounts.
- **OwnerReferencesPermissionEnforcement** RBAC lets the built-in StatefulSet
  controller (moco PVCs) and the SeaweedFS operator set finalizers on the
  resources they own, as OpenShift's admission plugin requires. These grants are
  rendered only when `openshift.enabled=true`.
