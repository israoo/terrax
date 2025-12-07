# Development S3 Stack - us-east-1

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment  = "dev"
  region       = "us-east-1"
  service      = "s3"
  bucket_name  = "terrax-dev-data"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOF
variable "environment" { type = string }
variable "region" { type = string }
variable "service" { type = string }
variable "bucket_name" { type = string }

resource "null_resource" "s3_placeholder" {
  triggers = {
    environment = var.environment
    region      = var.region
    service     = var.service
    bucket_name = var.bucket_name
  }
}

output "bucket_id" {
  value = null_resource.s3_placeholder.id
  description = "Simulated S3 bucket ID"
}

output "bucket_name" {
  value = var.bucket_name
  description = "S3 bucket name"
}
EOF
}
