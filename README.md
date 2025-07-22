# operator

## Development

### Prerequisites

- A Kubernetes cluster (e.g. [kind](https://kind.sigs.k8s.io/))
- [Tilt](https://tilt.dev/)

#### Install Kind

A kubernetes cluster is required to run the operator. [kind](https://kind.sigs.k8s.io/) is recommended for local development. Install `kind` and
create a cluster:

```bash
brew install kind
kind create cluster
```

This will create a new kind cluster with the name `kind`. The kubernetes context will be called `kind-kind`.

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
