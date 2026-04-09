variable "aws_profile" {
  type    = string
  default = ""
}

variable "cluster_name" {
  type    = string
  default = "wandb-operator-test"
}

variable "region" {
  type    = string
  default = "us-west-2"
}

variable "kubernetes_version" {
  type    = string
  default = "1.31"
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
  default = "m5.2xlarge"
}

variable "node_disk_size" {
  type    = number
  default = 100
}

variable "storage_class_name" {
  type    = string
  default = "gp2"
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "install_cloud_lb_controller" {
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
