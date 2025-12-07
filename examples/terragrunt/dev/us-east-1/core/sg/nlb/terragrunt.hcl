# Development NLB Security Group - us-east-1

include "root" {
  path = find_in_parent_folders("root.hcl")
}

terraform {
  source = "."
}

inputs = {
  environment = "dev"
  region      = "us-east-1"
  service     = "sg-nlb"
}

generate "main" {
  path      = "main.tf"
  if_exists = "overwrite"
  contents  = <<EOF
variable "environment" { type = string }
variable "region" { type = string }
variable "service" { type = string }

resource "null_resource" "sg_nlb_placeholder" {
  triggers = {
    environment = var.environment
    region      = var.region
    service     = var.service
  }
}

output "sg_id" {
  value = null_resource.sg_nlb_placeholder.id
  description = "Simulated Security Group ID for NLB"
}
EOF
}
