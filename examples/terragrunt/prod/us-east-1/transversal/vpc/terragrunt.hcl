# Production VPC Stack - us-east-1

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment = "prod"
  region      = "us-east-1"
  service     = "vpc"
  cidr_block  = "10.10.0.0/16"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOF
variable "environment" { type = string }
variable "region" { type = string }
variable "service" { type = string }
variable "cidr_block" { type = string }

resource "null_resource" "vpc_placeholder" {
  triggers = {
    environment = var.environment
    region      = var.region
    service     = var.service
    cidr_block  = var.cidr_block
  }
}

output "vpc_id" {
  value = null_resource.vpc_placeholder.id
}

output "cidr_block" {
  value = var.cidr_block
}
EOF
}
