# Deploy and operate W&B

Use these guides to deploy published Operator releases and administer W&B on
your Kubernetes cluster. Start with planning, then install Operator and create
the W&B resource.

| Task | Guide |
| --- | --- |
| Understand components, versions, and prerequisites | [Plan a deployment](planning.md) |
| Install Operator and create a W&B deployment | [Installation](installation.md) |
| Configure W&B, credentials, and application settings | [Configuration](configuration.md) |
| Connect managed or external backing services | [Infrastructure](infrastructure.md) |
| Configure Ingress, Gateway API, and TLS | [Networking](networking.md) |
| Use private registries, custom CAs, and proxies | [Restricted environments](restricted-environments.md) |
| Deploy on OpenShift | [OpenShift](openshift.md) |
| Enable and access the administration UI | [Console](console.md) |
| Collect and forward telemetry | [Monitoring](monitoring.md) |
| Upgrade Operator or W&B | [Upgrades](upgrades.md) |
| Migrate an existing Operator v1 deployment | [v1 to v2 migration](migrating-v1-to-v2.md) |
| Understand deletion and data ownership | [Retention and recovery](retention.md) |
| Diagnose a deployment problem | [Troubleshooting](troubleshooting.md) |
| Look up configuration and status fields | [Reference](reference.md) |

These pages describe v2 in this checkout. Match the documentation tag to your
Operator release. Operator and W&B have separate version numbers; examples use
placeholders for the versions you select. See [release notes](https://github.com/wandb/operator/releases)
for published Operator versions.
