provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

data "google_compute_zones" "available" {
  region = var.region
}

resource "time_static" "suffix" {
  count = var.append_timestamp ? 1 : 0
}

locals {
  zones        = slice(data.google_compute_zones.available.names, 0, 3)
  cluster_name = var.append_timestamp ? "${var.deployment_name}-${formatdate("YYMMDDhhmm", time_static.suffix[0].rfc3339)}" : var.deployment_name
  bucket_name  = "${local.cluster_name}-wandb"
  # GCP service account IDs must be 6-30 chars
  sa_prefix = substr(local.cluster_name, 0, 24)
  # GCP resource labels must be lowercase
  gcp_labels = { for k, v in var.tags : lower(k) => lower(v) }
}

# -----------------------------------------------------------------------------
# VPC
# -----------------------------------------------------------------------------

resource "google_compute_network" "this" {
  name                    = local.cluster_name
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "nodes" {
  name          = "${local.cluster_name}-nodes"
  network       = google_compute_network.this.id
  ip_cidr_range = "10.0.0.0/20"

  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = "10.1.0.0/16"
  }

  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = "10.2.0.0/20"
  }
}

resource "google_compute_router" "this" {
  name    = local.cluster_name
  network = google_compute_network.this.id
}

resource "google_compute_router_nat" "this" {
  name                               = local.cluster_name
  router                             = google_compute_router.this.name
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

# -----------------------------------------------------------------------------
# IAM
# -----------------------------------------------------------------------------

resource "google_service_account" "nodes" {
  account_id   = "${local.sa_prefix}-nodes"
  display_name = "GKE nodes for ${local.cluster_name}"
}

resource "google_project_iam_member" "node_roles" {
  for_each = toset([
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/artifactregistry.reader",
  ])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.nodes.email}"
}

# -----------------------------------------------------------------------------
# GKE Cluster
# -----------------------------------------------------------------------------

resource "google_container_cluster" "this" {
  provider = google-beta
  name     = local.cluster_name
  location = var.region

  min_master_version = var.kubernetes_version

  remove_default_node_pool = true
  initial_node_count       = 1

  network    = google_compute_network.this.id
  subnetwork = google_compute_subnetwork.nodes.id

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  addons_config {
    gce_persistent_disk_csi_driver_config {
      enabled = true
    }
    http_load_balancing {
      disabled = !var.install_cloud_lb_controller
    }
  }

  dynamic "gateway_api_config" {
    for_each = var.install_cloud_lb_controller ? [1] : []
    content {
      channel = "CHANNEL_STANDARD"
    }
  }

  deletion_protection = false

  resource_labels = merge(local.gcp_labels, {
    "wandb-cluster" = local.cluster_name
    "managed-by"    = "terraform"
  })
}

# -----------------------------------------------------------------------------
# Node Pool
# -----------------------------------------------------------------------------

resource "google_container_node_pool" "this" {
  provider = google-beta
  name     = "${local.cluster_name}-nodes"
  cluster  = google_container_cluster.this.id
  location = var.region

  node_count = var.node_count

  node_locations = var.node_count == 1 ? [local.zones[0]] : local.zones

  node_config {
    machine_type    = var.node_instance_type
    disk_size_gb    = var.node_disk_size
    disk_type       = "pd-ssd"
    service_account = google_service_account.nodes.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]

    labels = {
      "wandb-cluster" = local.cluster_name
    }
  }
}

# -----------------------------------------------------------------------------
# Artifact Registry (conditional)
# -----------------------------------------------------------------------------

resource "google_artifact_registry_repository" "wandb" {
  count         = var.create_registry ? 1 : 0
  location      = var.region
  repository_id = local.cluster_name
  format        = "DOCKER"
  cleanup_policy_dry_run = false
}

# -----------------------------------------------------------------------------
# GCS Bucket + HMAC Key (conditional)
# -----------------------------------------------------------------------------

resource "google_storage_bucket" "wandb" {
  count                       = var.create_bucket ? 1 : 0
  name                        = local.bucket_name
  location                    = var.region
  force_destroy               = true
  uniform_bucket_level_access = true

  labels = {
    "wandb-cluster" = local.cluster_name
  }
}

resource "google_service_account" "wandb_gcs" {
  count        = var.create_bucket ? 1 : 0
  account_id   = "${local.sa_prefix}-gcs"
  display_name = "W&B GCS access for ${local.cluster_name}"
}

resource "google_storage_bucket_iam_member" "wandb_gcs" {
  count  = var.create_bucket ? 1 : 0
  bucket = google_storage_bucket.wandb[0].name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.wandb_gcs[0].email}"
}

resource "google_storage_hmac_key" "wandb_gcs" {
  count                 = var.create_bucket ? 1 : 0
  service_account_email = google_service_account.wandb_gcs[0].email
}
