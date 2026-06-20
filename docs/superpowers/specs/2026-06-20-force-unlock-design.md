# Force Unlock Feature Design

**Date:** 2026-06-20
**Status:** Approved

---

## Goal

Allow users to force-unlock a Terraform state from the TerraX TUI by selecting a `force-unlock` command on a stack. TerraX discovers the lock ID automatically from S3 using the AWS CLI — no manual input, no dependency on a failed plan.

## Context

Terraform state locks are stored as `.tflock` files in S3 alongside the state file. The lock JSON contains an `ID` field required by `terraform force-unlock`. TerraX reads this file via `aws s3 cp` and passes the ID to `terragrunt run -- force-unlock -force <id>`.

---

## Configuration Schema

```yaml
state:
  bucket: "my-terraform-state-bucket"   # S3 bucket holding Terraform state
  project: "caas-workloads"             # Key prefix inside the bucket
  region: "us-east-1"                   # Default: us-east-1
```

The S3 lock key is: `<project>/<stack_relative_path>/terraform.tfstate.tflock`

`stack_relative_path` is computed with the existing `history.GetRelativeStackPath(absolutePath, rootConfigFile)`.

---

## Architecture

### `internal/state/locker.go` (NEW)

Pure business logic. No UI imports. Uses `exec.Command` to invoke the AWS CLI.

```go
// GetLockID fetches the Terraform state lock ID from S3 for the given stack.
// Returns ("", nil) when no lock exists.
// Returns ("", error) on AWS CLI failure or JSON parse error.
func GetLockID(ctx context.Context, bucket, project, stackRelPath, region string) (string, error)
```

Internals:
1. Build S3 key: `fmt.Sprintf("%s/%s/terraform.tfstate.tflock", project, stackRelPath)`
2. Run: `aws s3 cp s3://<bucket>/<key> - --region <region>`
3. If exit code != 0 and stderr contains "NoSuchKey" → return `"", nil` (no lock)
4. If exit code != 0 for other reason → return `"", fmt.Errorf("aws s3 cp failed: %w", err)`
5. Parse JSON: `{"ID": "...", "Who": "...", ...}` → return `id, nil`

**Mockable exec:** follow existing `internal/plan/collector.go` pattern — package-level `var execCommandContext = exec.CommandContext`.

### `internal/executor/executor.go`

New exported function:

```go
// RunForceUnlock executes a Terraform force-unlock for the given stack.
// Unlike Run, it uses --working-dir without --all and passes the lock ID directly.
func RunForceUnlock(ctx context.Context, historyLogger HistoryLogger, lockID, absoluteStackPath string) error
```

Arg construction (NOT using `buildTerragruntArgs` — force-unlock has different shape):

```
terragrunt run --working-dir <absoluteStackPath> --non-interactive -- force-unlock -force <lockID>
```

No logging flags, no `-out=`, no `--all`. Logs to history with command `"force-unlock"`.

### `cmd/root.go`

In the post-selection dispatch (after user confirms stack + command), add a branch before calling `executor.Run`:

```go
if command == "force-unlock" {
    return runForceUnlock(ctx, historyService, absoluteStackPath)
}
```

`runForceUnlock`:
1. Read `state.bucket`, `state.project`, `state.region` from viper. If bucket or project empty → return error with instructions.
2. Compute `stackRelPath` via `history.GetRelativeStackPath`.
3. Call `state.GetLockID(ctx, bucket, project, stackRelPath, region)`.
4. If `lockID == ""` → print `"No lock found for <stackRelPath>"` → return nil.
5. Print `"🔓 Unlocking <stackRelPath> (lock: <lockID>)"`.
6. Call `executor.RunForceUnlock(ctx, historyService, lockID, absoluteStackPath)`.

This branch exists in all three dispatch sites (normal TUI, `--history`, `--last`).

---

## Error Handling

| Scenario | Behavior |
|---|---|
| `state.bucket` or `state.project` not configured | Return error: "state.bucket and state.project must be set in .terrax.yaml to use force-unlock" |
| `aws` CLI not found | Return wrapped error from exec |
| S3 key not found (NoSuchKey) | Return `"", nil` → print "No lock found" |
| S3 access denied | Return wrapped error |
| Lock JSON malformed | Return `fmt.Errorf("failed to parse lock file: %w", err)` |
| No lock on stack | Print "No lock found for <path>" — not an error |

---

## Testing

### `internal/state/locker_test.go`

Table-driven. Mock `execCommandContext` (same pattern as `internal/plan/collector_test.go`).

Test cases:
- Lock found → returns correct ID
- No lock (NoSuchKey in stderr, non-zero exit) → returns `"", nil`
- AWS CLI error (non-NoSuchKey) → returns error
- Malformed JSON → returns parse error
- Empty bucket/project → lock key is constructed correctly (verify via captured args)

### `internal/executor/executor_test.go`

Test `RunForceUnlock` verifies:
- Arg shape: `["run", "--working-dir", path, "--non-interactive", "--", "force-unlock", "-force", lockID]`
- History is logged with command `"force-unlock"`

---

## Standards Compliance

- `internal/state/` — pure business logic, no UI imports (architecture-guard rule).
- All comments end with periods.
- Imports: 3 groups (stdlib, third-party, `github.com/israoo/terrax/...`).
- `execCommandContext` mockable var follows `internal/plan/collector.go` pattern.
- Errors wrapped with `fmt.Errorf("...: %w", err)`.
- Table-driven tests, `viper.Reset()` / `resetViper()` per test case.

---

## `.terrax.yaml` Addition

```yaml
# State backend configuration (required for force-unlock)
# state:
#   bucket: "my-terraform-state-bucket"
#   project: "caas-workloads"
#   region: "us-east-1"   # Default: us-east-1
```

And `force-unlock` can be added to the `commands` list:
```yaml
commands:
  - plan
  - apply
  - force-unlock
```

---

## Out of Scope

- Listing all locked stacks across the project (subtree/all-locks scan from the workflow).
- Showing lock details (who, when, operation) before unlocking.
- Confirmation prompt before force-unlock.
- Non-S3 backends (local, GCS, Azure).
