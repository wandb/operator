variable "deployment_name" {
  type    = string
  default = "wandb-operator-aks"
}

variable "append_timestamp" {
  type    = bool
  default = false
}

variable "region" {
  type    = string
  default = "eastus2"
}

variable "kubernetes_version" {
  type    = string
  default = "1.34"
}

variable "node_count" {
  type    = number
  default = 1
}

variable "node_zones" {
  type    = list(string)
  default = null
  description = "Availability zones for the default node pool. Set to [] to disable for regions without AZ support."
}

variable "node_instance_type" {
  type    = string
  default = "Standard_D8s_v5"
}

variable "node_disk_size" {
  type    = number
  default = 128
}

variable "storage_class_name" {
  type    = string
  default = "managed-csi"
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "install_cloud_lb_controller" {
  type    = bool
  default = false
}

variable "create_registry" {
  type    = bool
  default = false
}

variable "create_bucket" {
  type    = bool
  default = false
}
