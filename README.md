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

Alternatively, you can use the provided scripts to manage the kind cluster, uses kindClusterName from
`tilt-settings.star`, if present.

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

There are settings for Tilt that can be configured using a `tilt-settings.star` file. The settings file is not checked
into source control. A sample settings file is provided in `tilt-settings.sample.star`. To use the sample settings file,
copy it to `tilt-settings.star`

By default, Tilt is configured to only allow connections to the following Kubernetes contexts:

- `docker-desktop`
- `kind-kind`
- `kind-wandb-operator`
- `minikube`

Please add any additional contexts to the `allowedContexts` list in your `tilt-settings.star` file.

#### Running Tilt

```bash
tilt up
```

#### Cleaning Up Tilt

`tilt down` removes Tilt-managed workloads and Helm releases, but it intentionally does not
fully reset the cluster. The following are expected to survive a normal `tilt down`:

- `cert-manager` and its namespace
- operator CRDs, including the W&B CRDs and third-party operator CRDs
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

### Counterfeiter

```bash
go generate ./...
```
