# Set up local development

Run commands from the repository root. Use Go matching [go.mod](../../go.mod),
Docker, kubectl, and the tools below. Tilt creates `tilt-settings.star` from the
sample if it is absent; preserve an existing settings file.


## Prerequisites

- A Kubernetes cluster (e.g. [kind](https://kind.sigs.k8s.io/))
- [Tilt](https://tilt.dev/)
- [Kubebuilder](https://book.kubebuilder.io/quick-start.html)
- [Kustomize](https://kustomize.io/)
- [jq](https://stedolan.github.io/jq/) for some helper scripts
- [helm](https://github.com/helm/helm)

### Install Chart Testing

Install the chart-testing CLI for chart validation:

```bash
brew install chart-testing
```

After updating a chart, run the same lint command used by CI:

```bash
ct lint --all --config deploy/ct.yaml
```

### Install Kind

Install [kind](https://kind.sigs.k8s.io/) to create a local Kubernetes cluster:

```bash
brew install kind
```

## Create a cluster

```bash
kind create cluster
```

This creates a cluster named `kind` with Kubernetes context `kind-kind`.

Alternatively, you can use the provided scripts to manage the kind cluster.

```bash
# Create cluster
./hack/scripts/setup_kind.sh

# Delete cluster
./hack/scripts/teardown_kind.sh
```

## Install development tools

### Tilt

[Tilt](https://tilt.dev/) is a tool for local development of Kubernetes applications. Install `tilt`:

```bash
brew install tilt
```

### Kubebuilder

[kubebuilder](https://book.kubebuilder.io/quick-start.html) is a tool for building Kubernetes operators. Install `kubebuilder`:

```bash
brew install kubebuilder
```

### Kustomize

[kustomize](https://kustomize.io/) is used to build Kubernetes manifests. Install `kustomize`:

```bash
brew install kustomize
```

## Configuring and Running Tilt

### Tilt Settings

Tilt reads local settings from `tilt-settings.star`. The file is not checked
into source control; start from `tilt-settings.sample.star` and keep local
overrides there.

The default Tilt setup follows the normal operator install path:

- installs one `wandb-operator` Helm release in `wandb-operators`
- builds the local controller image as `controller:latest`
- creates a `WeightsAndBiases` CR in `wandb`
- uses `networkMode="gateway"` with `http://localhost:8080`
- rewrites non-loopback W&B hostnames through cluster CoreDNS so workloads can
  call the same URL through the local gateway or ingress
- uses the published server manifest repository by default
- keeps telemetry off unless `observabilityMode="full"` is set

Common W&B CR settings are scalar values such as `wandbHostname`,
`wandbVersion`, `size`, `retentionPolicy`, `licenseFile`, `manifestSource`,
and `networkMode`.
Set `networkMode="ingress"` to use the local ingress-nginx path instead of
Gateway API; if `wandbHostname` is not set explicitly, ingress mode uses
`http://wandb.localhost:8080`.

`enableCoreDNSRewrite=True` makes the hostname from `wandbHostname` resolve to
the local Gateway or ingress-nginx Service from inside the cluster. This is
enabled by default for non-loopback hostnames such as `wandb.localhost`. Tilt
updates its marked CoreDNS rules without replacing other CoreDNS configuration.
The rules remain in the development cluster after Tilt stops. Set the option to
`False` when cluster DNS is managed outside this local development setup.
OpenShift is excluded because its DNS Operator owns cluster DNS configuration.

Gateway mode defaults to the loopback-only `http://localhost:8080`. To use the
same external-style URL from the host and from workloads, override it locally:

```python
SETTINGS = {
    "wandbHostname": "http://wandb.localhost:8080",
}
```

Tilt defaults `manifestSource="published"`, which leaves
`spec.wandb.manifestRepository` empty so the W&B CR webhook applies the same
published OCI repository default as production installs. To test repo-local server manifest
definitions, set `manifestSource="local"` and keep
`localManifestPath="hack/testing-manifests/server-manifest"`. Choose a `wandbVersion` whose directory exists under that path.
The local fixture versions change independently of the published default.

Use `crFile` for custom CR shapes; Tilt treats it as a base CR and still
applies the scalar settings above.

When `useExternalObjectStore=True`, Tilt exposes the test SeaweedFS S3 API at
`http://s3.localhost:8333` by default. The connection Secret uses that same
hostname for direct presigned URLs, Tilt forwards the port for host access, and
CoreDNS resolves it to the SeaweedFS Service for cluster workloads. Override
`externalObjectStoreHostname` or `externalObjectStorePort` if the defaults
conflict with another local service. Keep `enableCoreDNSRewrite=True` unless
the configured hostname already resolves to the SeaweedFS Service in the
cluster. This automatic DNS setup is not supported on OpenShift.

By default, Tilt is configured to only allow connections to the following Kubernetes contexts:

- `docker-desktop`
- `kind-kind`
- `kind-wandb-operator`
- `minikube`
- `orbstack`
- `crc-admin`

Add any additional contexts to `allowedContexts` in `tilt-settings.star`.

For CRC/OpenShift Local, run `./hack/scripts/setup_crc.sh` and use the
`crc-admin` context. Tilt auto-enables `openshiftSCC` for CRC, pushes the dev
image through CRC's internal registry, and applies the OpenShift Helm profile.
For other OpenShift clusters, set `openshiftSCC=True` explicitly.

### Running Tilt

```bash
tilt up
```

### Cleaning Up Tilt

`tilt down` removes Tilt-managed workloads and Helm releases, but it intentionally does not
fully reset the cluster. The following are expected to survive a normal `tilt down`:

- `cert-manager` and its namespace
- `kube-state-metrics` and its namespace
- operator CRDs, including the W&B CRDs and operator dependency CRDs
- `wandb-operators` and dependency namespaces
- dev PVC-backed data unless the backing operator deletes it

For a true dev reset, use the helper script instead:

```bash
./hack/scripts/tilt-down-dev-clean.sh
```

This performs a safer teardown sequence for local development:

1. Deletes the `WeightsAndBiases` CR
2. Waits for finalizer-driven cleanup while the operators are still running
3. Uninstalls the Tilt-managed Helm releases
4. Deletes dev PVCs and generated secrets for the app
5. Runs `tilt down`

If you are already in the Tilt UI, you can trigger the manual `Dev-Clean` resource first,
then run `tilt down`.


Continue with the [development workflow](workflow.md) and [testing](testing.md).
For specialized environments, see [local webhook development](../../config/local-dev/README.md)
and [OpenShift local development](../../config/openshift-dev/README.md). The
colocated webhook guide contains older Tilt instructions; verify its settings
against the current Tiltfile before using that optional path.
