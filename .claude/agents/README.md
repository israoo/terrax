# Terrax Claude Agents

This directory contains specialized Claude agents for Terrax development. Each agent is an expert in a specific domain and ensures consistent, high-quality implementation following established patterns.

## Available Agents

### agent-developer
**Purpose:** Agent creation, maintenance, and governance
**Invoke when:**
- Creating new agents for Terrax subsystems
- Reviewing or updating existing agents
- Validating agent frontmatter and architecture
- Optimizing agent instructions for context efficiency
- Designing agent coordination patterns

**File:** [.claude/agents/agent-developer.md](.claude/agents/agent-developer.md)
**Model:** sonnet
**Color:** purple
**Size:** ~18 KB

---

### tui-expert
**Purpose:** Terrax TUI development using Bubble Tea and Lipgloss
**Invoke when:**
- Modifying TUI layout, styling, or rendering logic
- Working with Bubble Tea Model-Update-View pattern
- Implementing or debugging sliding window navigation
- Adjusting column dimensions, overflow indicators, or breadcrumbs
- Handling keyboard navigation or tea.Msg processing
- Applying Lipgloss styles or color schemes
- Debugging focus management or selection propagation

**File:** [.claude/agents/tui-expert.md](.claude/agents/tui-expert.md)
**Model:** sonnet
**Color:** cyan
**Size:** ~29 KB

**Specializations:**
- Horizontal sliding window (3-column navigation)
- Vertical scrolling (viewport pattern)
- Selection auto-propagation across columns
- Dynamic layout calculation
- Focus-driven rendering

---

### testing-expert
**Purpose:** Terrax testing strategy across all layers (unit, integration, TUI)
**Invoke when:**
- Writing unit tests for Navigator, tree parsing, or business logic
- Setting up filesystem mocking with afero for tree scanning tests
- Generating or updating mocks with go.uber.org/mock/mockgen
- Writing TUI tests with bubbletea/teatest helpers
- Implementing table-driven tests
- Debugging test failures or improving test coverage
- Validating test isolation and independence

**File:** [.claude/agents/testing-expert.md](.claude/agents/testing-expert.md)
**Model:** sonnet
**Color:** green
**Size:** ~22 KB

**Specializations:**
- Unit testing (Navigator, tree operations)
- Filesystem isolation with afero
- Mock generation with go.uber.org/mock
- TUI testing with bubbletea/teatest
- Table-driven test patterns
- Coverage analysis and optimization

---

## Agent Architecture

All Terrax agents follow the **Agent-Per-Domain** pattern:

- **One agent per mature subsystem** (3+ files, established patterns)
- **Context-efficient** (target 8-20 KB, max 30 KB)
- **Reference CLAUDE.md** (don't duplicate content)
- **Self-maintaining** (monitor dependencies, request updates)
- **Minimal permissions** (only include needed tools)

## Potential Future Agents

As Terrax functionality expands, new agents will be created:

- **stack-parser-expert** - Tree scanning, node construction, afero testing patterns
- **navigator-expert** - Navigation logic, offset management, sliding window algorithms
- **config-expert** - Viper configuration, environment variables, keyring integration

*Agents are created when subsystems reach maturity (3+ files, stable patterns, >5 KB documentation needed).*

## Usage

Agents are invoked automatically by Claude Code when their trigger scenarios are detected. You can also explicitly invoke an agent:

```
@agent-developer Please review this agent's frontmatter
@tui-expert Debug the sliding window navigation
```

## Maintenance

Agents self-monitor their dependencies and request updates when:

- CLAUDE.md architectural patterns change
- Related code files are refactored
- New mandatory patterns emerge
- Quality standards are refined

**All agent updates require explicit user approval** - agents never self-modify autonomously.

---

**Last Updated:** December 6, 2025
**Agent Count:** 3
**Managed by:** agent-developer
