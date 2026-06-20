# ADR-0010: Plan Analysis Mode via Binary Plan Generation

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0008: Dual-Mode TUI Architecture](0008-dual-mode-tui-architecture.md)
- [ADR-0009: Executor Isolation Pattern](0009-executor-isolation-pattern.md)

## Context

TerraX users need to analyze the results of `terragrunt run-all plan` to identify pending changes, dependencies, and potential issues before applying them.

Parsing raw stdout is brittle and error-prone due to interleaved logs, varying color codes, and unstructured text. Users need a reliable, machine-readable validation of their infrastructure changes without sacrificing the real-time feedback of the console output.

## Decision

We will implement a **Plan Analysis Mode** using a **Binary-to-JSON** workflow:

1.  **Binary Output**: The Executor automatically appends `-out=tfplan.binary` to plan commands.
2.  **Post-Processing**: After execution, TerraX runs `terragrunt show -json tfplan.binary` to generate a stable JSON representation.
3.  **TUI Viewer**: A dedicated `StatePlanReview` mode parses this JSON to present a structured Master-Detail view of the changes.

## Consequences

### Positive
*   **Reliability**: `terragrunt show -json` provides a schema-stable, machine-readable output that is immune to log formatting changes.
*   **Decoupling**: The display logic is completely independent of the execution logging or console output.
*   **User Experience**: Users get both live console feedback during the run and structured, interactive analysis afterwards.

### Negative
*   **Disk I/O**: Generates large binary files on disk that need to be managed (cleaned up or ignored).
*   **Latency**: Adds a post-processing step (`terragrunt show`) which slightly increases total workflow time.

## Alternatives Considered

### Option 1: Capture and Parse Stdout

**Description**: Attempt to parse the live standard output stream of the Terragrunt process to extract plan details.

**Pros**:
- Zero overhead (no extra commands run).
- No temporary files created.

**Cons**:
- Extremely brittle; a single change in log format breaks the parser.
- Difficult to handle interleaved output from parallel executions.

**Why rejected**: Reliability is paramount for infrastructure tools; regex parsing of stdout is too flaky for production use.

### Option 2: JSON-Only Execution

**Description**: Run `terragrunt plan -json` directly, suppressing standard human-readable output.

**Pros**:
- Single execution step.
- No temporary binary files.

**Cons**:
- Hides the familiar, friendly console output from the user during the long-running process.
- Large JSON payloads can be memory-intensive to stream and parse in real-time.

**Why rejected**: We want to preserve the "watch it run" experience for the user while still getting structured data.

## Future Enhancements

**Potential Improvements**:
1.  **Auto-Cleanup**: Automatically delete `tfplan.binary` files upon exiting the application to save disk space.
2.  **Summary Dashboard**: A high-level view summarizing total adds, changes, and destroys across the entire environment.
3.  **Apply from Plan**: Allow users to execute `terragrunt apply tfplan.binary` directly from the TUI, ensuring exactly the reviewed changes are applied.

## References

- **Terraform Plan Binary**: https://developer.hashicorp.com/terraform/cli/commands/plan#out-path
- **Terraform Show**: https://developer.hashicorp.com/terraform/cli/commands/show
