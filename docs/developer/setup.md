# Set up local development

Run commands from the repository root. Use Go matching [go.mod](../../go.mod),
Docker, kubectl, and the tools below. Tilt creates `tilt-settings.star` from the
sample if it is absent; preserve an existing settings file.


## Prerequisites

- A Kubernetes cluster (e.g. [kind](https://kind.sigs.k8s.io/))
- [Tilt](https://tilt.dev/)
- [jq](https://stedolan.github.io/jq/) for some helper scripts
- [helm](https://github.com/helm/helm)
- Optional: [Kubebuilder](https://book.kubebuilder.io/quick-start.html) and
  [Kustomize](https://kustomize.io/) CLIs. `make` downloads its own
  controller-gen, kustomize, setup-envtest, and golangci-lint into `bin/`.

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

# Create a cluster with a specific Kubernetes version and two worker nodes
./hack/scripts/setup_kind.sh 2 --kubernetes-version 1.35

# Select an exact patch version
./hack/scripts/setup_kind.sh --kubernetes-version v1.35.2

# Delete cluster
./hack/scripts/teardown_kind.sh
```

The optional version selects a `kindest/node` image; `1.35` selects `v1.35.0`.
Use an image supported by your installed [kind release](https://github.com/kubernetes-sigs/kind/releases).
Without a version, kind uses its default image. An existing cluster is reused only
if its nodes match the requested minor or exact patch version; the script does
not recreate mismatched clusters automatically.

Unlike plain `kind create cluster`, the script also installs metrics-server with
`--kubelet-insecure-tls`. It names the cluster `kind` unless `kindClusterName`
is set in `tilt-settings.star`.

## Install development tools

### Tilt

[Tilt](https://tilt.dev/) is a tool for local development of Kubernetes applications. Install `tilt`:

```bash
brew install tilt
```

### Kubebuilder (optional)

[kubebuilder](https://book.kubebuilder.io/quick-start.html) is only needed to scaffold new APIs or controllers. Install `kubebuilder`:

```bash
brew install kubebuilder
```

### Kustomize (optional)

[kustomize](https://kustomize.io/) builds the Kustomize manifests under `config/`; the `make` targets use their own copy in `bin/`. Install `kustomize`:

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
- keeps telemetry off unless `observabilityMode` is set to `full` or `forward`

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

### Chart dependency repositories

Tilt's `Operator-Chart-Deps` resource runs `helm dependency build ./deploy/operator --skip-refresh`,
which needs the chart's dependency repositories added and indexed locally. Add
them once, as CI does:

```bash
helm repo add ci-wandb https://charts.wandb.ai/
helm repo add ci-moco https://cybozu-go.github.io/moco/
helm repo add ci-ot-container-kit https://ot-container-kit.github.io/helm-charts
helm repo add ci-seaweedfs https://seaweedfs.github.io/seaweedfs-operator/
helm repo add ci-prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add ci-altinity https://helm.altinity.com
helm repo add ci-victoria-metrics https://victoriametrics.github.io/helm-charts/
helm repo add ci-grafana https://grafana.github.io/helm-charts
helm repo update
```

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
For OpenShift, see [OpenShift local development](../../config/openshift-dev/README.md).
The out-of-cluster webhook guide in `config/local-dev/` is out of date: its
`setup-local-webhook` script is gone and `make run-local-webhook` passes a flag the
manager no longer defines. Use Tilt instead.
