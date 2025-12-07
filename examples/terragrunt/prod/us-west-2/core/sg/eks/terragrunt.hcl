# prod sg Stack - us-west-2 (core)

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment = "prod"
  region      = "us-west-2"
  category    = "core"
  service     = "sg"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOT
variable "environment" { type = string }
variable "region" { type = string }
variable "category" { type = string }
variable "service" { type = string }

resource "null_resource" "sg_placeholder" {
  triggers = {
    environment = var.environment
    region      = var.region
    category    = var.category
    service     = var.service
  }
}

output "resource_id" {
  value       = null_resource.sg_placeholder.id
  description = "Simulated sg resource ID"
}

output "details" {
  value = {
    environment = var.environment
    region      = var.region
    service     = var.service
  }
  description = "sg details"
}
EOT
}
