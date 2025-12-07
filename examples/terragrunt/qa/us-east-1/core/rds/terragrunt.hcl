# QA RDS Stack - us-east-1

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment = "qa"
  region      = "us-east-1"
  service     = "rds"
  instance_class = "db.t3.small"
  engine         = "postgres"
  engine_version = "14.7"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOF
variable "environment" { type = string }
variable "region" { type = string }
variable "service" { type = string }
variable "instance_class" { type = string }
variable "engine" { type = string }
variable "engine_version" { type = string }

resource "null_resource" "rds_placeholder" {
  triggers = {
    environment    = var.environment
    region         = var.region
    service        = var.service
    instance_class = var.instance_class
    engine         = var.engine
    engine_version = var.engine_version
  }
}

output "database_id" {
  value = null_resource.rds_placeholder.id
}

output "endpoint" {
  value = "$${var.service}-$${var.environment}-$${var.region}.example.com:5432"
}
EOF
}
