# operator
// TODO(user): Add simple overview of use/purpose

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Documentation

- [Monitoring and Telemetry Guide](docs/monitoring.md)

## Development

### Prerequisites

- A Kubernetes cluster (e.g. [kind](https://kind.sigs.k8s.io/))
- [Tilt](https://tilt.dev/)
- [Kubebuilder](https://book.kubebuilder.io/quick-start.html)
- [Kustomize](https://kustomize.io/)
- [jq](https://stedolan.github.io/jq/) for some helper scripts

#### Install Kind

A kubernetes cluster is required to run the operator. [kind](https://kind.sigs.k8s.io/) is recommended for local development. Install `kind` and
create a cluster:

```bash
brew install kind
```

#### Create a Cluster

```bash
kind create cluster
```

This will create a new kind cluster with the name `kind`. The kubernetes context will be called `kind-kind`.

Alternatively, you can use the provided scripts to manage the kind cluster.

```bash
# Create cluster
./hack/scripts/setup_kind.sh

# Delete cluster
./hack/scripts/teardown_kind.sh
```

#### Install Tilt

[Tilt](https://tilt.dev/) is a tool for local development of Kubernetes applications. Install `tilt`:

```bash
brew install tilt
```

#### Install Kubebuilder

[kubebuilder](https://book.kubebuilder.io/quick-start.html) is a tool for building Kubernetes operators. Install `kubebuilder`:

```bash
brew install kubebuilder
```

#### Install Kustomize

[kustomize](https://kustomize.io/) is used to build Kubernetes manifests. Install `kustomize`:

```bash
brew install kustomize
```

### Configuring and Running Tilt

#### Tilt Settings

Tilt reads local settings from `tilt-settings.star`. The file is not checked
into source control; start from `tilt-settings.sample.star` and keep local
overrides there.

The default Tilt setup follows the normal operator install path:

- installs one `wandb-operator` Helm release in `wandb-operators`
- builds the local controller image as `controller:latest`
- creates a `WeightsAndBiases` CR in `wandb`
- uses `networkMode="gateway"` with `http://localhost:8080`
- uses the published server manifest repository by default
- keeps telemetry off unless `observabilityMode="full"` is set

Common W&B CR settings are scalar values such as `wandbHostname`,
`wandbVersion`, `size`, `retentionPolicy`, `licenseFile`, `manifestSource`,
and `networkMode`.
Set `networkMode="ingress"` to use the local ingress-nginx path instead of
Gateway API; if `wandbHostname` is not set explicitly, ingress mode uses
`http://wandb.localhost:8080`.

Tilt defaults `manifestSource="published"`, which leaves
`spec.wandb.manifestRepository` empty so the W&B CR webhook applies the same
published OCI repository default as production installs. To test repo-local server manifest
definitions, set `manifestSource="local"` and keep
`localManifestPath="hack/testing-manifests/server-manifest"`. The default local
manifest path currently contains `0.79.0`, so also set `wandbVersion="0.79.0"`
when using that local source.

Use `crFile` for custom CR shapes; Tilt treats it as a base CR and still
applies the scalar settings above.

By default, Tilt is configured to only allow connections to the following Kubernetes contexts:

- `docker-desktop`
- `kind-kind`
- `kind-wandb-operator`
- `minikube`
- `orbstack`

Please add any additional contexts to the `allowedContexts` list in your `tilt-settings.star` file.

#### Running Tilt

```bash
tilt up
```

#### Cleaning Up Tilt

`tilt down` removes Tilt-managed workloads and Helm releases, but it intentionally does not
fully reset the cluster. The following are expected to survive a normal `tilt down`:

- `cert-manager` and its namespace
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

## Testing

### Locally testing external infra

1. Install the WandB CR with Tilt using the default `retentionPolicy="detach"` in `tilt-settings.star`.
2. Delete the WandB CR — infra should be detached but remain in place.
3. Run `./hack/scripts/managed-connections-to-external.sh` to convert the managed connection secrets into external ones.
4. Install the WandB CR with Tilt using a custom `crFile` that points at a CR
   with the external infra connection specs.
5. WandB should now run with externally managed infra.

### Counterfeiter

```bash
go generate ./...
```
