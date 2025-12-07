# Development NLB Stack - us-east-1

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment = "dev"
  region      = "us-east-1"
  service     = "nlb"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOF
variable "environment" { type = string }
variable "region" { type = string }
variable "service" { type = string }

resource "null_resource" "nlb_placeholder" {
  triggers = {
    environment = var.environment
    region      = var.region
    service     = var.service
  }
}

output "nlb_id" {
  value = null_resource.nlb_placeholder.id
  description = "Simulated NLB ID"
}

output "nlb_dns" {
  value = "$${var.service}-$${var.environment}.$${var.region}.elb.amazonaws.com"
  description = "Simulated NLB DNS"
}
EOF
}
