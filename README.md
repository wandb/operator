# operator
// TODO(user): Add simple overview of use/purpose

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Development

### Prerequisites

- A Kubernetes cluster (e.g. [kind](https://kind.sigs.k8s.io/))
- [Tilt](https://tilt.dev/)
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

Alternatively, you can use the provided scripts to manage the kind cluster:

```bash
# Create cluster (uses kindClusterName from tilt-settings.json if present)
./scripts/setup_kind.sh

# Delete cluster
./scripts/teardown_kind.sh
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

### Configuring and Running Tilt

#### Tilt Settings

There are settings for Tilt that can be configured using a `tilt-settings.json` file. The settings file is not checked
into source control. A sample settings file is provided in `tilt-settings.sample.json`. To use the sample settings file,
copy it to `tilt-settings.json`

By default, Tilt is configured to only allow connections to the following Kubernetes contexts:

- `docker-desktop`
- `kind-kind`
- `kind-wandb-operator`
- `minikube`

Please add any additional contexts to the `allowedContexts` list in your `tilt-settings.json` file.

#### Running Tilt

```bash
tilt up
```

## Testing

### Counterfeiter

```bash
go generate ./...
```
