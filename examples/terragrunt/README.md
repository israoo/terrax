# TerraX examples with Terragrunt

This directory contains example structures to test TerraX.

## 📁 Example structure

```text
terragrunt/
├── dev/                    # development environment
│   └── us-east-1/
│       ├── core/           # core services
│       │   ├── rds/        # database
│       │   ├── eks/        # Kubernetes cluster
│       │   ├── nlb/        # load balancer
│       │   └── sg/         # security groups
│       │       ├── eks/
│       │       └── nlb/
│       └── transversal/    # transversal services
│           ├── vpc/        # networking
│           └── s3/         # object storage
├── prod/                   # production environment
│   ├── us-east-1/
│   └── us-west-2/
├── qa/                     # QA environment
│   ├── us-east-1/
│   └── us-west-2/
└── root.hcl                # shared root configuration
```

## 🚀 How to test TerraX

### 1. Build TerraX

```bash
cd terrax
make build
```

### 2. Navigate to examples directory

```bash
cd examples/terragrunt
```

### 3. Run TerraX

```bash
# from examples/terragrunt directory
../../build/terrax
```

### 4. Test commands

Inside the TerraX TUI:

- **Navigate** with arrow keys ← ↑ ↓ → between environments/regions/services
- **Select** a stack (e.g., `dev` → `us-east-1` → `core` → `rds`)
- **Choose** a command (e.g., `init`, `validate`, `plan`)
- **Execute** with `Enter`

### 5. Recommended commands for testing

#### ✅ `init`

Initializes the Terragrunt stack (downloads providers, configures backend)

```bash
# in TerraX: select stack → init → Enter
# equivalent to: terragrunt run --all --working-dir <stack-path> -- init
```

#### ✅ `validate`

Validates Terraform configuration without accessing external resources

```bash
# in TerraX: select stack → validate → Enter
# equivalent to: terragrunt run --all --working-dir <stack-path> -- validate
```

#### ✅ `plan`

Shows what changes would be applied (dry-run mode)

```bash
# in TerraX: select stack → plan → Enter
# equivalent to: terragrunt run --all --working-dir <stack-path> -- plan
```

## 📝 Important notes

### No AWS credentials required

Examples use the **`null` provider** instead of AWS provider, so:

- ✅ You don't need AWS credentials
- ✅ No real resources are created
- ✅ Ideal for quick TerraX testing
- ✅ `terraform` commands work without issues

### Local backend

Backend is configured as `local`:

- States are saved in `.terragrunt-cache/`
- No S3 or DynamoDB required
- Perfect for disposable examples

### Generated files

Terragrunt automatically generates:

- `backend.tf` - backend configuration
- `provider.tf` - null provider configuration
- `main.tf` - example resources (null_resource)

These files are auto-generated and can be deleted/regenerated:

```bash
# clean generated files
find . -name ".terragrunt-cache" -type d -exec rm -rf {} +
find . -name ".terraform" -type d -exec rm -rf {} +
find . -name "*.tfstate*" -delete
find . -name ".terraform.lock.hcl" -delete
find . -name "*.tf" -delete
```

## 📚 Additional resources

- [TerraX documentation](../../README.md)
- [TerraX configuration](.terrax.yaml)
- [Terragrunt docs](https://terragrunt.gruntwork.io/docs/)
