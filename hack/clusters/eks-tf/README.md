# EKS Test Cluster

Terraform configuration for provisioning an EKS cluster to test Operator v2 deployments.

## Prerequisites

- AWS CLI configured with appropriate credentials
- Terraform >= 1.5
- `kubectl`

### Required IAM Permissions

The caller needs permissions to create: VPC, subnets, internet gateway, route tables, IAM roles/policies, EKS cluster, EKS node groups, EKS add-ons, and (optionally) an OIDC provider and Helm releases.

## Usage

```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars as needed

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
| AWS ALB ingress | `true` | `ingress` | `alb` |
| AWS Gateway API | `true` | `gateway` | `amazon-vpc-lattice` |

The W&B controllers (nginx ingress and nginx-gateway-fabric) are installed by Tilt. The AWS Load Balancer Controller requires IAM integration and is installed by Terraform when `install_cloud_lb_controller = true`.

## Node Sizing

- `node_count = 1`: Single node for `dev` size deployments
- `node_count = 3`: Cross-AZ for `small` and above

Default instance type `m5.2xlarge` (8 vCPU, 32 GB RAM) provides sufficient resources per node.

## External Object Store

Set `create_bucket = true` to create an S3 bucket with a dedicated IAM user. The outputs map directly to the `wandb-objectstore-connection` / `external-objectstore-connection` secret keys:

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
