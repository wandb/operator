output "cluster_name" {
  value = google_container_cluster.this.name
}

output "cluster_endpoint" {
  value = google_container_cluster.this.endpoint
}

output "kubeconfig_command" {
  value = "gcloud container clusters get-credentials ${google_container_cluster.this.name} --region ${var.region} --project ${var.project_id}"
}

output "kube_context_name" {
  value = "gke_${var.project_id}_${var.region}_${google_container_cluster.this.name}"
}

output "cloud_ingress_class" {
  value = var.install_cloud_lb_controller ? "gce" : null
}

output "cloud_gateway_class" {
  value = var.install_cloud_lb_controller ? "gke-l7-global-external-managed" : null
}

# Object store outputs — map to wandb-objectstore-connection secret keys
output "objectstore_endpoint" {
  value = var.create_bucket ? "storage.googleapis.com" : null
}

output "objectstore_port" {
  value = var.create_bucket ? "443" : null
}

output "objectstore_bucket" {
  value = var.create_bucket ? google_storage_bucket.wandb[0].name : null
}

output "objectstore_region" {
  value = var.create_bucket ? var.region : null
}

output "objectstore_access_key" {
  value     = var.create_bucket ? google_storage_hmac_key.wandb_gcs[0].access_id : null
  sensitive = true
}

output "objectstore_secret_key" {
  value     = var.create_bucket ? google_storage_hmac_key.wandb_gcs[0].secret : null
  sensitive = true
}

output "objectstore_url" {
  description = "S3-compatible connection URL for wandb-objectstore-connection"
  value       = var.create_bucket ? "s3://${google_storage_hmac_key.wandb_gcs[0].access_id}:${google_storage_hmac_key.wandb_gcs[0].secret}@storage.googleapis.com/${google_storage_bucket.wandb[0].name}?region=${var.region}" : null
  sensitive   = true
}
