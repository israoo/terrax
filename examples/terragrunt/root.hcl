# root.hcl - Example configuration for TerraX testing

# Configure remote state (mock example - not functional)
remote_state {
  backend = "local"

  config = {
    path = "terraform.tfstate"
  }

  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }
}

# Generate a minimal provider configuration for testing
generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
# Auto-generated provider configuration for TerraX examples
terraform {
  required_version = ">= 1.0"

  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
  }
}

# Using null provider for testing (no AWS credentials needed)
provider "null" {}
EOF
}

# Inputs available to all child configurations
inputs = {
  project_name = "terrax-example"
}
