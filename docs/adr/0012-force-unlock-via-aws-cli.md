# ADR-0012: Force Unlock via AWS CLI Lock Discovery

**Status**: Accepted

**Date**: 2026-06-20

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md)
- [ADR-0007: Configuration Management Strategy](0007-configuration-management-strategy.md)

## Context

Terraform state locks can be left behind by failed or interrupted operations. Unlocking requires knowing the lock ID, which is stored in a `.tflock` file alongside the state in the S3 backend.

TerraX users need a way to force-unlock a stack from the TUI without leaving the tool. The challenge is obtaining the lock ID automatically. Three constraints shaped the decision:

1. The lock ID must be discovered without user input — requiring users to copy-paste a UUID from another terminal defeats the purpose of having a TUI.
2. The lock may have been created by a process outside TerraX (CI/CD pipeline, manual terraform run), so TerraX cannot rely on its own history or prior failed executions to know the ID.
3. TerraX must not introduce a heavy new dependency (Go AWS SDK) for what is a low-frequency operational command.

Without a solution, users must resort to `aws s3 cp` + `terraform force-unlock` manually, losing the context of the TUI navigation.

## Decision

We introduce a dedicated `internal/state` package that discovers the lock ID by invoking the AWS CLI directly, then delegates execution to a new `executor.RunForceUnlock` function.

**Lock discovery** (`internal/state/locker.go`):

```go
func GetLockID(ctx context.Context, bucket, project, stackRelPath, region, profile, configFile string) (string, error)
```

The function constructs the S3 key as `path.Join(project, stackRelPath, "terraform.tfstate.tflock")`, runs `aws s3 cp s3://<bucket>/<key> -`, and parses the JSON `ID` field. If the file does not exist (AWS returns `NoSuchKey`), it returns `("", nil)` — no lock, no error. Profile and config file are passed via `--profile` flag and `AWS_CONFIG_FILE` env var respectively.

**Force-unlock execution** (`internal/executor/executor.go`):

```go
func RunForceUnlock(ctx context.Context, historyLogger HistoryLogger, lockID, absoluteStackPath string) error
```

Args: `terragrunt run --working-dir <path> --non-interactive -- force-unlock -force <lockID>`. Uses `--working-dir` without `--all` (single-stack operation), and logs to history with command `"force-unlock"`.

**Wiring** (`cmd/root.go`): `runForceUnlock` reads `state.bucket`, `state.project`, `state.region`, `state.aws_profile`, and `state.aws_config_file` from viper. It checks for missing required keys before calling `GetLockID`, and prints `"No lock found"` and returns nil when the stack is not locked.

The `force-unlock` branch is inserted **before** `executor.Run` at all three dispatch sites (normal TUI, `--last`, `--history re-execute`) so it never reaches the general executor path.

**Configuration**:

```yaml
state:
  bucket: "my-terraform-state-bucket"
  project: "caas-workloads"
  region: "us-east-1"
  aws_profile: "my-profile"          # optional
  aws_config_file: "/path/to/config" # optional

commands:
  - force-unlock
```

## Consequences

### Positive

- **Zero user input** — lock ID is discovered automatically from S3; the user only navigates to the stack and selects `force-unlock`.
- **Source-agnostic** — works regardless of who created the lock (TerraX, CI/CD pipeline, manual terraform), because it reads the current S3 state rather than TerraX's history.
- **No new Go dependencies** — uses the AWS CLI subprocess pattern already established in `internal/plan/collector.go`, keeping the binary size and dependency surface unchanged.
- **History-tracked** — force-unlock operations are logged with the same mechanism as other commands.
- **Testable** — `GetLockID` uses the same `execCommandContext` mock pattern as the plan collector, enabling full unit test coverage without real AWS credentials.

### Negative

- **Requires AWS CLI** — the feature silently fails if `aws` is not in `PATH`. There is no compile-time or startup check for the CLI.
- **Requires explicit config** — `state.bucket` and `state.project` must be configured in `.terrax.yaml`; there is no auto-detection from the stack's HCL files.
- **S3-only** — the lock discovery mechanism is specific to S3 backends. Other backends (GCS, Azure, local) are not supported.
- **AWS credentials assumed** — the tool assumes the runtime environment has valid AWS credentials; it does not guide the user through authentication failures.

## Alternatives Considered

### Option 1: Manual lock ID input via TUI prompt

**Description**: When the user selects `force-unlock`, TerraX displays a text input field where the user types the lock ID obtained from another terminal or from a prior failed plan output.

**Pros**:

- No AWS CLI or S3 configuration required.
- Works with any Terraform backend.

**Cons**:

- Requires the user to obtain the lock ID through other means before using TerraX, negating the TUI convenience.
- TerraX currently has no modal text input UI pattern; adding one solely for this feature would be disproportionate scope.

**Why rejected**: The core requirement was zero manual input. Forcing the user to look up a UUID elsewhere breaks the workflow the feature is meant to streamline.

### Option 2: Parse lock ID from a failed plan output

**Description**: TerraX runs a `terraform plan -lock-timeout=0` to trigger a lock error, then parses the lock ID from the error message.

**Pros**:

- No AWS credentials or bucket configuration needed.
- Works as long as Terraform can reach the backend.

**Cons**:

- Adds an extra network round-trip before every force-unlock attempt.
- The lock could have been created by a long-running CI process using different credentials; triggering a plan against that state may itself fail for unrelated reasons, making the ID unrecoverable.
- Error message parsing is fragile and backend-specific.

**Why rejected**: The user explicitly required that the solution not depend on a failed plan, because locks from outside TerraX may not produce a plan error the tool can interpret reliably.

### Option 3: Parse the S3 backend config from terragrunt.hcl

**Description**: TerraX reads the stack's `terragrunt.hcl` to extract the `remote_state` block and derive `bucket`, `key`, and `region` automatically, eliminating the need for `state.*` config keys.

**Pros**:

- No extra configuration required beyond what already exists in the repository.
- Self-documenting — the source of truth for the backend is the HCL files themselves.

**Cons**:

- HCL parsing in Go requires a dedicated library and significant complexity to handle includes, locals, and interpolations that are common in Terragrunt configurations.
- A parsing failure would silently produce wrong bucket/key values, leading to confusing "no lock found" results.
- TerraX is intentionally a thin UI layer over Terragrunt, not an HCL interpreter.

**Why rejected**: The fragility and complexity of HCL parsing outweigh the configuration convenience. A single `state:` block in `.terrax.yaml` is a small and predictable trade-off.

### Option 4: Go AWS SDK integration

**Description**: Replace the AWS CLI subprocess with direct S3 API calls using the `github.com/aws/aws-sdk-go-v2` package.

**Pros**:

- No external dependency on the `aws` binary being in `PATH`.
- Better error handling and structured API responses.
- Could support credential providers not available via the CLI.

**Cons**:

- Adds a significant dependency tree to a tool that currently has no cloud-provider SDKs.
- The AWS SDK itself requires credential configuration that users already have set up for the CLI.
- Increases binary size and compilation time for a low-frequency feature.

**Why rejected**: The AWS CLI is already a prerequisite for any Terragrunt workflow involving S3-backed state. Adding the SDK would increase binary weight and dependency surface without meaningfully expanding the tool's capabilities for this use case.

## Future Enhancements

**Potential Improvements**:

1. **Auto-detect backend config from HCL** — once a robust HCL parser is available in TerraX, `state.bucket` and `state.project` could be inferred automatically from `terragrunt.hcl`.
2. **Multi-stack unlock** — scan all `.tflock` files under a project prefix and display a list of locked stacks for batch unlock, mirroring the all-stacks mode of the reference pipeline workflow.
3. **Lock detail display** — show `Who`, `Operation`, and `Created` from the lock JSON before executing force-unlock, giving the user context about what created the lock.
4. **Non-S3 backend support** — abstract the lock discovery behind an interface to support GCS, Azure Blob, and other backends.

## References

- `internal/state/locker.go` — `GetLockID` implementation
- `internal/executor/executor.go` — `RunForceUnlock` implementation
- `cmd/root.go` — `runForceUnlock` wiring at all dispatch sites
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md)
- Reference pipeline: `.github/workflows/force-unlock.yml` in `efex-tpl-do-pipeline-infrastructure-terragrunt`
