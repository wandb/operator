# Plan a deployment

Operator reconciles a `WeightsAndBiases` custom resource into W&B applications
and their backing infrastructure. You install Operator with a Helm chart, then
declare each W&B deployment with an `apps.wandb.com/v2` resource.

## Components and ownership

| Component | Provisioning in this version |
| --- | --- |
| MySQL | Managed through Moco, or an external connection |
| Redis | Managed through the Redis operator, or an external connection |
| Object storage | Managed through SeaweedFS, or an external S3, GCS, or Azure connection |
| ClickHouse | Managed through the Altinity operator, or an external connection |
| Kafka | Managed Bufstream and its supporting resources; no external Kafka field is exposed |
| Console | Optional administration UI, enabled per W&B resource |
| Telemetry | Optional VictoriaMetrics stack and Grafana, configured through Helm |

Omitted infrastructure configuration defaults to managed services. Choose
external connections before creating a deployment when you intend to use
existing data services. Changing a connection is not a data migration.

## Select versions

The Operator chart version selects the controller and bundled dependency
operators. `spec.wandb.version` selects a separately published W&B server
manifest, which describes that W&B release's applications, sizing, and migrations.
It must be a specific version; an empty value or `latest` is rejected.

Use the release documentation and your deployment's tested version combination.
This repository does not currently publish a comprehensive Operator/W&B/Kubernetes
compatibility matrix or production capacity table. A size name such as `small`
is a manifest sizing profile, not a guarantee that a particular cluster is large
enough. Review the release's workload requests and persistent storage needs.

## Prepare the cluster

- Install `kubectl` and an OCI-capable Helm 3 client. Repository CI uses Helm
  3.19.0; use a `kubectl` version appropriate for your cluster.
- Arrange cluster-level permissions to install CRDs, webhooks, RBAC, and component
  operators, plus namespace-level permissions to create the deployment.
- Install cert-manager with its CRDs. The Operator chart requires its
  `Certificate` and `Issuer` resources for the admission webhook.
- Provide dynamic persistent storage for managed services, normally through a
  default `StorageClass`.
- Install an Ingress controller, or Gateway API CRDs and a Gateway controller,
  and identify the corresponding class. Operator creates routing resources;
  the installation guide assumes the routing controller already exists.
- Choose the public W&B URL, configure DNS, and prepare a TLS certificate or
  issuer. Workloads and browsers must be able to reach the relevant application
  and object-storage endpoints.
- Select your W&B release and arrange the license required for your deployment.
- Ensure access to the chart, controller/dependency images, W&B images, and
  server manifest. For isolated networks, prepare the
  [restricted environment configuration](restricted-environments.md).

Review existing cluster-wide component operators before installing another copy.
Disable overlapping chart dependencies only when their required controller and
CRDs already exist, or the corresponding service will be external. See
[infrastructure configuration](infrastructure.md).

Continue with [installation](installation.md), or the
[migration guide](migrating-v1-to-v2.md) for an existing v1 deployment.
