# GKE Test Cluster

Terraform configuration for provisioning a GKE cluster to test Operator v2 deployments.

## Prerequisites

- `gcloud` CLI authenticated (`gcloud auth application-default login`)
- Terraform >= 1.5
- `kubectl`
- GCP project with Kubernetes Engine API enabled

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars — project_id is required

terraform init
terraform apply
```

After apply completes, configure kubectl:

```bash
$(terraform output -raw kubeconfig_command)
```

Then add the context to your `tilt-settings.star`:

```python
SETTINGS = {
    "allowedContexts": ["<paste kube_context_name output here>"],
    ...
}
```

Run `tilt up` — Tilt handles cert-manager, ingress/gateway controllers, operators, and the W&B CR.

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

Retrieve sensitive values with `terraform output -raw objectstore_access_key`.

## Teardown

```bash
terraform destroy
```
