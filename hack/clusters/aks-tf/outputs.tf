output "cluster_name" {
  value = azurerm_kubernetes_cluster.this.name
}

output "cluster_endpoint" {
  value = azurerm_kubernetes_cluster.this.fqdn
}

output "kubeconfig_command" {
  value = "az aks get-credentials --resource-group ${azurerm_resource_group.this.name} --name ${azurerm_kubernetes_cluster.this.name}"
}

output "kube_context_name" {
  value = azurerm_kubernetes_cluster.this.name
}

output "cloud_ingress_class" {
  value = var.install_cloud_lb_controller ? "azure-application-gateway" : null
}

output "cloud_gateway_class" {
  description = "Azure does not have a mature native Gateway API controller. Use GKE for cloud Gateway API testing, or use Tilt's nginx-gateway-fabric."
  value       = null
}

# ACR outputs
output "registry_url" {
  value = var.create_registry ? azurerm_container_registry.wandb[0].login_server : null
}

output "registry_login_command" {
  value = var.create_registry ? "az acr login --name ${azurerm_container_registry.wandb[0].name}" : null
}

# Object store outputs — map to wandb-objectstore-connection secret keys
# Azure Blob Storage exposes an S3-compatible interface via the storage account key
output "objectstore_endpoint" {
  value = var.create_bucket ? "${azurerm_storage_account.wandb[0].name}.blob.core.windows.net" : null
}

output "objectstore_port" {
  value = var.create_bucket ? "443" : null
}

output "objectstore_bucket" {
  value = var.create_bucket ? azurerm_storage_container.wandb[0].name : null
}

output "objectstore_region" {
  value = var.create_bucket ? var.region : null
}

output "objectstore_access_key" {
  value     = var.create_bucket ? azurerm_storage_account.wandb[0].name : null
  sensitive = true
}

output "objectstore_secret_key" {
  value     = var.create_bucket ? azurerm_storage_account.wandb[0].primary_access_key : null
  sensitive = true
}

output "objectstore_url" {
  description = "Connection URL for wandb-objectstore-connection"
  value       = var.create_bucket ? "s3://${azurerm_storage_account.wandb[0].name}:${azurerm_storage_account.wandb[0].primary_access_key}@${azurerm_storage_account.wandb[0].name}.blob.core.windows.net/${azurerm_storage_container.wandb[0].name}?region=${var.region}" : null
  sensitive   = true
}
