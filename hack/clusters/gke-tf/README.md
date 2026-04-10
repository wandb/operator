# GKE Test Cluster

Terraform configuration for provisioning a GKE cluster to test Operator v2 deployments.

## Prerequisites

- `gcloud` CLI installed and authenticated (`gcloud auth login` and `gcloud auth application-default login`)
- Terraform >= 1.5
- `kubectl`
- Docker (for Artifact Registry image push)
- GCP project with the following APIs enabled:
  - Kubernetes Engine API
  - Artifact Registry API (if `create_registry = true`)

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars — project_id is required

terraform init
terraform apply
```

After apply completes, configure kubectl:

```bash
eval "$(terraform output -raw kubeconfig_command)"
kubectl get nodes  # verify all nodes are Ready
```

If using Artifact Registry (`create_registry = true`), authenticate Docker:

```bash
eval "$(terraform output -raw registry_login_command)"
```

Then configure your `tilt-settings.star` with the terraform outputs:

```bash
terraform output -raw kube_context_name
terraform output -raw registry_url  # if create_registry = true
```

```python
SETTINGS = {
    "allowedContexts": ["<paste kube_context_name>"],
    "defaultRegistry": "<paste registry_url>",  # if create_registry = true
    ...
}
```

Run `tilt up` — Tilt handles cert-manager, ingress/gateway controllers, operators, and the W&B CR.

## Container Registry (Artifact Registry)

Remote clusters need a container registry for Tilt to push operator images to. Set `create_registry = true` to create an Artifact Registry Docker repository. GKE nodes can pull from it automatically (via the `artifactregistry.reader` role already on the node service account).

Unlike ECR, Artifact Registry supports nested repositories natively, so `registrySingleName` is not needed.

## Networking Scenarios

| Scenario | `install_cloud_lb_controller` | W&B CR `networking.mode` | Class name |
|----------|------------------------------|--------------------------|------------|
| W&B nginx ingress | `false` | `ingress` | `nginx` |
| W&B nginx-gateway-fabric | `false` | `gateway` | `nginx` |
| GCE ingress | `true` | `ingress` | `gce` |
| GKE Gateway API | `true` | `gateway` | `gke-l7-global-external-managed` |

GKE has the most mature cloud-native Gateway API support. When `install_cloud_lb_controller = true`, both the GCE ingress controller and the GKE Gateway controller are enabled as built-in cluster add-ons (no Helm install needed).

## Node Sizing

- `node_count = 1`: Single node in one zone for `dev` size deployments
- `node_count = 3`: Spread across 3 zones for `small` and above

Default instance type `e2-standard-8` (8 vCPU, 32 GB RAM) provides sufficient resources per node.

## External Object Store

Set `create_bucket = true` to create a GCS bucket with HMAC credentials (S3-compatible). The outputs map directly to the `wandb-objectstore-connection` / `external-objectstore-connection` secret keys:

| Output | Secret Key |
|--------|------------|
| `objectstore_endpoint` | `Host` |
| `objectstore_port` | `Port` |
| `objectstore_bucket` | `Bucket` |
| `objectstore_region` | `Region` |
| `objectstore_access_key` | `AccessKey` |
| `objectstore_secret_key` | `SecretKey` |
| `objectstore_url` | `url` |

Retrieve sensitive values:

```bash
terraform output -raw objectstore_access_key
terraform output -raw objectstore_secret_key
terraform output -raw objectstore_url
```

## Teardown

```bash
terraform destroy
```
