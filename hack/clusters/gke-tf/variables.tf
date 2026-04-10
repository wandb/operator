variable "project_id" {
  type = string
}

variable "cluster_name" {
  type    = string
  default = "wandb-operator-gke"
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "kubernetes_version" {
  type    = string
  default = "1.34"
}

variable "node_count" {
  type    = number
  default = 1

  validation {
    condition     = var.node_count == 1 || var.node_count >= 3
    error_message = "node_count must be 1 (dev) or >= 3 (cross-AZ)."
  }
}

variable "node_instance_type" {
  type    = string
  default = "e2-standard-8"
}

variable "node_disk_size" {
  type    = number
  default = 100
}

variable "storage_class_name" {
  type    = string
  default = "standard-rwo"
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

variable "bucket_name" {
  type    = string
  default = ""
}
