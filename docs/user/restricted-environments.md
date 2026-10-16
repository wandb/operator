# Private registries and restricted connectivity

A restricted installation must make the Operator chart, Operator and dependency
images, W&B workload images, and the W&B server manifest available to the cluster.
The server manifest is an OCI artifact, separate from the container images.
Inventory all of these for your selected release before installing.

## Registry configuration

Mirror the release's artifacts using your registry tooling. W&B's `wsm registry
mirror` workflow places the server manifest beside the workload images at
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

For a new resource, omitting `manifestRepository` derives it from `imageRegistry`.
On an existing resource, the previously defaulted repository is already stored;
update it explicitly when changing registries. Preserve the image repository
layout expected by the mirror workflow.

The CR's registry settings do not configure the Helm chart's own controller
images. Install from your mirrored chart and set its Operator and dependency image
repositories and pull credentials separately. Obtain the exact values supported by
your selected chart with `helm show values`. A single universal set of image
overrides for every dependency is not defined in this guide.

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
in-cluster exclusions and injects both uppercase and lowercase proxy variables
into application workloads, including init containers and migration Jobs. This
CR setting does not configure the Operator process's own egress proxy.

## Verify access

Check Operator logs for manifest authentication or TLS failures, pod events for
image pull failures, and the W&B status for application readiness. Validate an
artifact upload and browser download through the actual external endpoints.
Refer to [troubleshooting](troubleshooting.md) to distinguish registry credentials,
CA trust, missing mirrored images, and application proxy errors.
