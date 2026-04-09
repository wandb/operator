output "cluster_name" {
  value = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  value = aws_eks_cluster.this.endpoint
}

output "kubeconfig_command" {
  value = "aws eks update-kubeconfig --name ${aws_eks_cluster.this.name} --region ${var.region}${var.aws_profile != "" ? " --profile ${var.aws_profile}" : ""}"
}

output "kube_context_name" {
  value = "arn:aws:eks:${var.region}:${local.account_id}:cluster/${aws_eks_cluster.this.name}"
}

output "cloud_ingress_class" {
  value = var.install_cloud_lb_controller ? "alb" : null
}

output "cloud_gateway_class" {
  value = var.install_cloud_lb_controller ? "amazon-vpc-lattice" : null
}

# Object store outputs — map to wandb-objectstore-connection secret keys
output "objectstore_endpoint" {
  value = var.create_bucket ? "s3.${var.region}.amazonaws.com" : null
}

output "objectstore_port" {
  value = var.create_bucket ? "443" : null
}

output "objectstore_bucket" {
  value = var.create_bucket ? aws_s3_bucket.wandb[0].bucket : null
}

output "objectstore_region" {
  value = var.create_bucket ? var.region : null
}

output "objectstore_access_key" {
  value     = var.create_bucket ? aws_iam_access_key.wandb_s3[0].id : null
  sensitive = true
}

output "objectstore_secret_key" {
  value     = var.create_bucket ? aws_iam_access_key.wandb_s3[0].secret : null
  sensitive = true
}

output "objectstore_url" {
  description = "S3 connection URL for wandb-objectstore-connection"
  value       = var.create_bucket ? "s3://${aws_iam_access_key.wandb_s3[0].id}:${aws_iam_access_key.wandb_s3[0].secret}@s3.${var.region}.amazonaws.com/${aws_s3_bucket.wandb[0].bucket}?region=${var.region}" : null
  sensitive   = true
}
