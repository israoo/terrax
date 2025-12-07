# Development EKS Stack - us-east-1

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment    = "dev"
  region         = "us-east-1"
  service        = "eks"
  cluster_name   = "terrax-dev-eks"
  cluster_version = "1.34"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOF
variable "environment" { type = string }
variable "region" { type = string }
variable "service" { type = string }
variable "cluster_name" { type = string }
variable "cluster_version" { type = string }

resource "null_resource" "eks_placeholder" {
  triggers = {
    environment     = var.environment
    region          = var.region
    service         = var.service
    cluster_name    = var.cluster_name
    cluster_version = var.cluster_version
  }
}

output "cluster_id" {
  value = null_resource.eks_placeholder.id
  description = "Simulated EKS cluster ID"
}

output "cluster_endpoint" {
  value = "https://$${var.cluster_name}.eks.$${var.region}.amazonaws.com"
  description = "Simulated EKS cluster endpoint"
}
EOF
}
