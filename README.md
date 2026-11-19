# W&B Operator

A Kubernetes operator for deploying and managing self-hosted
[Weights & Biases](https://wandb.ai) on your own cluster. Declare a
`WeightsAndBiases` resource to select the W&B version, sizing, networking, and
backing infrastructure; Operator reconciles the deployment to that desired state.

## Documentation

- **[Deploy and operate W&B](docs/user/README.md)** — plan and install a deployment,
  configure infrastructure and networking, monitor it, and manage upgrades.
- **[Develop Operator](docs/developer/README.md)** — run locally with Kind and Tilt,
  understand the architecture, change the APIs/controllers, test, and release.
- **[Design records](docs/design/README.md)** — proposals, implementation rationale,
  and historical notes with links to current guidance.

## Installation

Start with the [prerequisites](docs/user/planning.md) and
[installation guide](docs/user/installation.md). Operator v2 is distributed as the
OCI chart `oci://us-docker.pkg.dev/wandb-production/public/wandb/charts/operator`.
The `wandb/operator` chart in the legacy `charts.wandb.ai` repository is Operator v1.

## Development

Follow [local setup](docs/developer/setup.md), then the
[development workflow](docs/developer/workflow.md) and [testing guide](docs/developer/testing.md).

## Versions

This branch contains Operator v2. Match documentation to your installed release
using the corresponding Git tag. Operator and W&B server versions are selected
separately. See [Operator releases](https://github.com/wandb/operator/releases).
Maintenance and release history for Operator v1 live on the
[v1 branch](https://github.com/wandb/operator/tree/v1).
