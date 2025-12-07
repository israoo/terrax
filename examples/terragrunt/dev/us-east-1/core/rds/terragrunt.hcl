# Development RDS Stack - us-east-1

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment = "dev"
  region      = "us-east-1"
  service     = "rds"

  # Example RDS configuration
  instance_class = "db.t3.micro"
  engine         = "postgres"
  engine_version = "14.7"
}

# Generate a minimal terraform configuration for testing
generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOF
# Example RDS stack - for TerraX testing only
# This creates a null_resource to simulate infrastructure

variable "environment" {
  type = string
}

variable "region" {
  type = string
}

variable "service" {
  type = string
}

variable "instance_class" {
  type    = string
  default = "db.t3.micro"
}

variable "engine" {
  type    = string
  default = "postgres"
}

variable "engine_version" {
  type    = string
  default = "14.7"
}

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
  description = "Simulated RDS database ID"
}

output "endpoint" {
  value = "$${var.service}-$${var.environment}-$${var.region}.example.com:5432"
  description = "Simulated database endpoint"
}
EOF
}
