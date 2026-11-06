# operator

// TODO(user): Add simple overview of use/purpose

## Description

// TODO(user): An in-depth paragraph about your project and overview of use

## Managed spec

Set `MANAGED_SPEC_ENABLED=true` on the operator to prefer the
`wandb-spec-managed` ConfigMap in the instance namespace. It must contain
`chart` and `values` keys with JSON objects describing the chart and Helm values.
The flag defaults to false and can also be set with `--managed-spec-enabled`.

The operator continues fetching and caching the Deployer spec on each normal
reconcile, unless the Deployer channel is off (below). When the flag is enabled, it uses a valid
managed spec directly, without comparing it to Deployer. If the ConfigMap is
missing, it uses Deployer; if it cannot be read or decoded, it logs an error and
uses Deployer. Existing Deployer cache fallback still applies during an outage.
Chart rendering or apply failures follow the normal reconciliation error path.

## Deployer channel

`DEPLOYER_CHANNEL_ENABLED` defaults to `true`. Set it to `false`, or pass
`--deployer-channel-enabled=false`, to stop fetching the Deployer channel spec,
including `_releaseId` pins and the `latest-cached-release` cache. `AIRGAPPED=true`
also turns the Deployer channel off; this flag does it without the other
airgapped settings. Charts are still downloaded from their repository, and the
license is still read as before. The operator exports
`wandb_operator_deployer_channel_enabled`, which is 0 when either flag turns
the channel off.

With `MANAGED_SPEC_ENABLED=true` and the Deployer channel off, there is no
Deployer to fall back to. A missing, unreadable or undecodable
`wandb-spec-managed` makes the operator stop before building a desired spec: it
sets the phase to `Waiting For Managed Spec`, records a `WaitingForManagedSpec`
warning event, applies nothing and retries every minute. The ConfigMap watch
reconciles as soon as it appears. This holds even when the CR or user spec sets
`chart`, which would otherwise be applied without the managed values. Deletion
is not affected.

To remove or disable the managed spec on such an install, first restart the
operator with `DEPLOYER_CHANNEL_ENABLED=true` and `AIRGAPPED=false`, so it can
fall back to Deployer. Turning off `MANAGED_SPEC_ENABLED` alone isn't safe:
with the Deployer channel still off, the guard no longer applies, and a chart
set by the CR or user spec would be applied without the managed values.

Selection is reevaluated on each reconcile. No cutover state is written or read;
old `wandb-managed-spec-state` ConfigMaps can remain and are ignored. Setting
the flag to false returns the operator to Deployer after restart, as long as
the Deployer channel is on.

## User spec

`USER_SPEC_ENABLED` defaults to `true`. Set it to `false`, or pass
`--user-spec-enabled=false`, to stop reading and applying the
`<instance-name>-spec-user` Secret for every instance watched by this operator.
Restart the operator after changing the environment variable.

Disabled mode does not create or change the user-spec Secret and ignores its
values, chart, metadata, `_releaseId`, and license inputs. The CR spec, selected
managed/Deployer base, and operator defaults continue to apply. Managed-spec
selection and Deployer fallback retain their existing behavior.

Disable user spec only after the installation's required settings have moved
to retained sources. Keep the Secret for rollback: setting the flag back to
`true` resumes reading it. Console-managed OIDC, Slack, SMTP, and bucket
Secrets/ConfigMaps remain independent of this flag.

## Development

### Prerequisites

- A Kubernetes cluster (e.g. [kind](https://kind.sigs.k8s.io/))
- [Tilt](https://tilt.dev/)

#### Install Kind

A kubernetes cluster is required to run the operator.
[kind](https://kind.sigs.k8s.io/) is recommended for local development. Install
`kind` and create a cluster:

```bash
brew install kind
kind create cluster
```

This will create a new kind cluster with the name `kind`. The kubernetes context
will be called `kind-kind`.

#### Install Tilt

[Tilt](https://tilt.dev/) is a tool for local development of Kubernetes
applications. Install `tilt`:

```bash
brew install tilt
```

#### Install Kubebuilder

[kubebuilder](https://book.kubebuilder.io/quick-start.html) is a tool for
building Kubernetes operators. Install `kubebuilder`:

```bash
brew install kubebuilder
```

### Configuring and Running Tilt

#### Tilt Settings

There are settings for Tilt that can be configured using a `tilt-settings.json`
file. The settings file is not checked into source control. A sample settings
file is provided in `tilt-settings.sample.json`. To use the sample settings
file, copy it to `tilt-settings.json`

By default, Tilt is configured to only allow connections to the following
Kubernetes contexts:

- `docker-desktop`
- `kind-kind`
- `minikube`
- `orbstack`

Please add any additional contexts to the `allowedContexts` list in your
`tilt-settings.json` file.

#### Running Tilt

```bash
tilt up
```

## Testing

### Counterfeiter

```bash
go generate ./...
```
