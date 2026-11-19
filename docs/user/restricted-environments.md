# Private registries and restricted connectivity

A restricted installation must make the Operator chart, Operator and dependency
images, W&B workload images, and the W&B server manifest available to the cluster.
The server manifest is an OCI artifact, separate from the container images.
Inventory all of these for your selected release before installing.

## Registry configuration

Mirror the release's artifacts using your registry tooling. W&B's `wsm registry
mirror` workflow places the server manifest in the same registry at
`<imageRegistry>/wandb/server-manifest`. Configure W&B to use that registry:

```yaml
spec:
  global:
    imageRegistry: registry.example.com/wandb-mirror
    imagePullSecrets:
      - name: registry-credentials
  wandb:
    version: <wandb-version>
    manifestRepository: oci://registry.example.com/wandb-mirror/wandb/server-manifest
```

Create `registry-credentials` as a `kubernetes.io/dockerconfigjson` Secret in the
W&B namespace using your credential-management process. These references are used
for both the Operator's manifest retrieval and the workload image pulls.

For a `manifestRepository` other than the public default, Operator also tries its
own pod's cloud identity after any pull Secrets: ECR through IRSA or EKS Pod
Identity, Artifact Registry or GCR through GKE Workload Identity, and ACR through
Entra Workload or Managed Identity. This covers only the manifest pull; workload
image pulls still need pull Secrets or node credentials.

For a new resource, omitting `manifestRepository` derives it from `imageRegistry`.
On an existing resource, the previously defaulted repository is already stored;
update it explicitly when changing registries.

Operator prefixes each image's full source reference, including its source
registry host, with `imageRegistry`. For example,
`us-docker.pkg.dev/wandb-production/public/wandb/megabinary:<tag>` is pulled as
`registry.example.com/wandb-mirror/us-docker.pkg.dev/wandb-production/public/wandb/megabinary:<tag>`.
Only the manifest uses the shorter `<imageRegistry>/wandb/server-manifest` path;
preserve both layouts when mirroring with other tooling.

The CR's registry settings do not configure the Helm chart's own controller
images. Install from your mirrored chart and set its Operator and dependency image
repositories and pull credentials separately. Obtain the exact values supported by
your selected chart with `helm show values`. A single universal set of image
overrides for every dependency is not defined in this guide.

Console, when enabled, runs the Operator image exactly as set by
`wandb-operator.image.repository` and `tag`, in the W&B namespace. It is not
retargeted by `spec.global.imageRegistry` and its Pod receives no
`spec.global.imagePullSecrets`, so nodes must be able to pull that image with
their own credentials.

## Telemetry images and plugins

Configure telemetry images in the Operator Helm values separately from the W&B
CR. Mirror the telemetry repositories under your prefix and merge these settings
into your complete values file:

```yaml
global:
  image:
    registry: registry.example.com/mirror
  imagePullSecrets:
    - name: registry-credentials
```

The telemetry templates apply this prefix to the OTEL collectors, Victoria
components and config reloaders, alerting, and Grafana. Pull Secrets must exist
in each namespace where the corresponding Pods run. `global.imagePullSecrets` is
chart-wide: the Operator Deployment, the CRD-installer hook, and the
VictoriaMetrics operator chart also use it. Configure the Operator and
component-controller images using their own chart values as well.

The full-stack Grafana Pod copies its bundled plugins from the installed Operator
image, including its registry/tag or digest. An explicit
`telemetry.grafana.pluginImage` overrides that source; the image must contain
`/opt/wandb/grafana-plugins`. The standalone telemetry chart requires
`grafana.pluginImage` when running in full mode. Runtime plugin downloading is
disabled, so mirror the plugin source image along with Grafana itself.

Support-bundle collections use a separate Lumen image. Mirror it when that
feature is enabled; see [support bundles](support-bundles.md).

## Trust a custom registry CA

Set one source under the chart's `wandb-operator.caCerts`. For example, reference
an existing Secret containing PEM certificate files in the Operator namespace:

```yaml
wandb-operator:
  caCerts:
    existingSecret: registry-ca
```

Alternatively use `existingConfigMap`, or supply a list of PEM strings in `certs`.
For an inline file when installing the chart:

```bash
helm install wandb-operator "$OPERATOR_CHART" \
  --version "$OPERATOR_VERSION" \
  --namespace wandb-operators --create-namespace \
  -f operator-values.yaml \
  --set-file 'wandb-operator.caCerts.certs[0]=./registry-ca.crt'
```

Set `OPERATOR_CHART` to your selected chart location. The CA is added to the
Operator process's trust store, preserving its system roots. Node/container-runtime
trust for pulling images is configured separately by the cluster administrator.

## Trust custom CAs in W&B workloads

Create a ConfigMap in the W&B namespace whose `.crt` keys contain PEM certificates,
then reference it from the CR:

```yaml
spec:
  global:
    caCertsConfigMap: wandb-custom-ca
```

`spec.global.customCACerts` alternatively accepts inline PEM strings. These
settings apply to W&B workloads; the Operator's registry CA setting above applies
to the Operator process. Configure each trust boundary that uses the private CA.

## Configure application egress proxies

```yaml
spec:
  global:
    proxy:
      httpProxy: {value: http://proxy.example.com:3128}
      httpsProxy: {value: http://proxy.example.com:3128}
      noProxy:
        - storage.example.com
```

Use `valueFrom.secretKeyRef` for a proxy URL containing credentials. Each
`noProxy` entry must be a separate string without commas. Operator adds its
in-cluster exclusions and the cloud metadata address `169.254.169.254`, then
injects both uppercase and lowercase proxy variables
into application workloads, including init containers and migration Jobs. This
CR setting does not configure the Operator process's own egress proxy; set proxy
variables for it under `wandb-operator.containers.operator.env` in the chart
values, with a `NO_PROXY` that covers the Kubernetes API and in-cluster addresses.

## Verify access

Check Operator logs for manifest authentication or TLS failures, pod events for
image pull failures, and the W&B status for application readiness. Validate an
artifact upload and browser download through the actual external endpoints.
Refer to [troubleshooting](troubleshooting.md) to distinguish registry credentials,
CA trust, missing mirrored images, and application proxy errors.
