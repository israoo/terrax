# qa nlb Stack - us-east-1 (core)

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment = "qa"
  region      = "us-east-1"
  category    = "core"
  service     = "nlb"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOT
variable "environment" { type = string }
variable "region" { type = string }
variable "category" { type = string }
variable "service" { type = string }

resource "null_resource" "nlb_placeholder" {
  triggers = {
    environment = var.environment
    region      = var.region
    category    = var.category
    service     = var.service
  }
}

output "resource_id" {
  value       = null_resource.nlb_placeholder.id
  description = "Simulated nlb resource ID"
}

output "details" {
  value = {
    environment = var.environment
    region      = var.region
    service     = var.service
  }
  description = "nlb details"
}
EOT
}
