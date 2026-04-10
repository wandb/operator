provider "azurerm" {
  features {}
}

locals {
  tags = merge(var.tags, {
    "wandb-cluster" = var.cluster_name
    "ManagedBy"     = "terraform"
  })
  # Azure storage account names: lowercase alphanumeric, 3-24 chars
  storage_account_name = var.bucket_name != "" ? var.bucket_name : replace(substr("${var.cluster_name}wandb", 0, 24), "-", "")
  container_name       = "wandb"
}

# -----------------------------------------------------------------------------
# Resource Group
# -----------------------------------------------------------------------------

resource "azurerm_resource_group" "this" {
  name     = var.cluster_name
  location = var.region
  tags     = local.tags
}

# -----------------------------------------------------------------------------
# VNet
# -----------------------------------------------------------------------------

resource "azurerm_virtual_network" "this" {
  name                = var.cluster_name
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  address_space       = ["10.0.0.0/16"]
  tags                = local.tags
}

resource "azurerm_subnet" "nodes" {
  name                 = "${var.cluster_name}-nodes"
  resource_group_name  = azurerm_resource_group.this.name
  virtual_network_name = azurerm_virtual_network.this.name
  address_prefixes     = ["10.0.0.0/20"]
}

resource "azurerm_subnet" "appgw" {
  count                = var.install_cloud_lb_controller ? 1 : 0
  name                 = "${var.cluster_name}-appgw"
  resource_group_name  = azurerm_resource_group.this.name
  virtual_network_name = azurerm_virtual_network.this.name
  address_prefixes     = ["10.0.16.0/24"]
}

# -----------------------------------------------------------------------------
# AKS Cluster
# -----------------------------------------------------------------------------

resource "azurerm_kubernetes_cluster" "this" {
  name                = var.cluster_name
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  dns_prefix          = var.cluster_name
  kubernetes_version  = var.kubernetes_version
  tags                = local.tags

  identity {
    type = "SystemAssigned"
  }

  default_node_pool {
    name                = "default"
    node_count          = var.node_count
    vm_size             = var.node_instance_type
    os_disk_size_gb     = var.node_disk_size
    vnet_subnet_id      = azurerm_subnet.nodes.id
    zones               = var.node_count >= 3 ? ["1", "2", "3"] : null
    temporary_name_for_rotation = "tempdefault"
  }

  network_profile {
    network_plugin = "azure"
  }

  storage_profile {
    disk_driver_enabled = true
  }

  dynamic "ingress_application_gateway" {
    for_each = var.install_cloud_lb_controller ? [1] : []
    content {
      subnet_id = azurerm_subnet.appgw[0].id
    }
  }
}

# -----------------------------------------------------------------------------
# Azure Container Registry (conditional)
# -----------------------------------------------------------------------------

resource "azurerm_container_registry" "wandb" {
  count               = var.create_registry ? 1 : 0
  name                = replace(var.cluster_name, "-", "")
  resource_group_name = azurerm_resource_group.this.name
  location            = azurerm_resource_group.this.location
  sku                 = "Basic"
  admin_enabled       = true
  tags                = local.tags
}

# Grant AKS pull access to ACR
resource "azurerm_role_assignment" "aks_acr_pull" {
  count                = var.create_registry ? 1 : 0
  principal_id         = azurerm_kubernetes_cluster.this.kubelet_identity[0].object_id
  role_definition_name = "AcrPull"
  scope                = azurerm_container_registry.wandb[0].id
}

# -----------------------------------------------------------------------------
# Azure Blob Storage (conditional)
# -----------------------------------------------------------------------------

resource "azurerm_storage_account" "wandb" {
  count                    = var.create_bucket ? 1 : 0
  name                     = local.storage_account_name
  resource_group_name      = azurerm_resource_group.this.name
  location                 = azurerm_resource_group.this.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  tags                     = local.tags
}

resource "azurerm_storage_container" "wandb" {
  count                = var.create_bucket ? 1 : 0
  name                 = local.container_name
  storage_account_id   = azurerm_storage_account.wandb[0].id
}
