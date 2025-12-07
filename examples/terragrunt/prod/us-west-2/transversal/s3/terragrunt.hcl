# prod s3 Stack - us-west-2 (transversal)

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment = "prod"
  region      = "us-west-2"
  category    = "transversal"
  service     = "s3"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOT
variable "environment" { type = string }
variable "region" { type = string }
variable "category" { type = string }
variable "service" { type = string }

resource "null_resource" "s3_placeholder" {
  triggers = {
    environment = var.environment
    region      = var.region
    category    = var.category
    service     = var.service
  }
}

output "resource_id" {
  value       = null_resource.s3_placeholder.id
  description = "Simulated s3 resource ID"
}

output "details" {
  value = {
    environment = var.environment
    region      = var.region
    service     = var.service
  }
  description = "s3 details"
}
EOT
}
