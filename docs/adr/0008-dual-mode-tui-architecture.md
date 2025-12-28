# ADR-0007: Dual-Mode TUI Architecture with State Machine

**Status**: Accepted

**Date**: 2025-12-28

**Deciders**: TerraX Core Team

**Related**:
- [ADR-0001: Bubble Tea Architecture](0001-bubble-tea-architecture.md)
- [ADR-0006: Execution History Management](0006-execution-history-management.md)

## Context

TerraX has two distinct user workflows that require different UI presentations:

1. **Navigation Mode**: Browse directory tree hierarchy and select stacks to execute commands.
2. **History Mode**: Browse execution history and re-run previous commands.

Both workflows share characteristics:
- Terminal-based UI with Bubble Tea framework.
- List navigation (up/down, selection).
- Exit to execute selected command.
- Similar input handling patterns.

### Problem

How should we implement these two different UI modes?

**Options**:
1. **Two separate applications**: `terrax` and `terrax-history`.
2. **Two separate TUI implementations**: Completely different code paths.
3. **Single TUI with mode switching**: Shared Model with state machine.

### Requirements

- Support both navigation and history workflows.
- Minimize code duplication.
- Maintain clean separation between modes.
- Allow easy addition of future modes (e.g., config editor).
- Keep codebase maintainable and testable.

## Decision

Implement **dual-mode TUI architecture** with:

1. **AppState enum**: Explicit state machine to track current mode.
2. **Single Model**: Shared Bubble Tea model with mode-specific fields.
3. **Branched Update**: Update method delegates to mode-specific handlers.
4. **Branched View**: View method delegates to mode-specific renderers.
5. **Mode-specific constructors**: Different entry points for each mode.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      TUI Model (Bubble Tea)                  │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ AppState: StateNavigation | StateHistory               │ │
│  └────────────────────────────────────────────────────────┘ │
│                           ↓                                  │
│         ┌─────────────────┴─────────────────┐               │
│         │                                   │               │
│  ┌──────▼───────┐                  ┌────────▼────────┐      │
│  │ Navigation   │                  │  History Mode   │      │
│  │    Mode      │                  │                 │      │
│  │              │                  │                 │      │
│  │ - Navigator  │                  │ - Entries list  │      │
│  │ - Columns    │                  │ - Table view    │      │
│  │ - Filters    │                  │ - Selection     │      │
│  │ - Breadcrumbs│                  │ - Re-execute    │      │
│  └──────────────┘                  └─────────────────┘      │
│                                                              │
│  Update() ──→ Switch on AppState ──→ Delegate to handler    │
│  View()   ──→ Switch on AppState ──→ Delegate to renderer   │
└─────────────────────────────────────────────────────────────┘
```

### AppState Enum

Explicit state machine using Go enum pattern:

```go
// internal/tui/model.go

// AppState represents the current application mode.
type AppState int

const (
    // StateNavigation is the default tree navigation mode.
    StateNavigation AppState = iota

    // StateHistory is the execution history viewer mode.
    StateHistory
)

// String returns human-readable state name.
func (s AppState) String() string {
    switch s {
        case StateNavigation:
            return "Navigation"
        case StateHistory:
            return "History"
        default:
            return "Unknown"
    }
}
```

**Benefits**:
- **Explicit**: State is first-class citizen, not implicit.
- **Type-safe**: Compiler enforces valid states.
- **Debuggable**: Easy to print current state.
- **Extensible**: Add new states easily.

### Model Structure

Single Model with mode-specific fields:

```go
// internal/tui/model.go

type Model struct {
    // Application state
    state AppState

    // Common fields (used by both modes)
    width  int
    height int
    ready  bool

    // Navigation mode fields
    navigator         *stack.Navigator
    navState          *stack.NavigationState
    focusedColumn     int
    selectedCommand   int
    navigationOffset  int
    columnFilters     map[int]textinput.Model
    scrollOffsets     map[int]int
    // ... more navigation fields

    // History mode fields
    historyEntries       []history.ExecutionLogEntry
    selectedHistoryIndex int
    reExecuteFromHistory bool
    selectedHistoryEntry *history.ExecutionLogEntry
}
```

**Design Principles**:
- **Shared fields**: Common state (width, height, ready).
- **Mode-specific fields**: Prefixed by mode (history*, nav* implied).
- **Nil when inactive**: Navigation fields nil in history mode, vice versa.

### Mode-Specific Constructors

Different entry points for each mode:

```go
// Navigation mode constructor
func NewModel(root *stack.Node, maxDepth int) Model {
    return Model{
        state:     StateNavigation,
        navigator: stack.NewNavigator(root, maxDepth),
        navState: &stack.NavigationState{
            SelectedIndices: []int{0},
        },
        focusedColumn:    1,
        selectedCommand:  0,
        navigationOffset: 0,
        columnFilters:    make(map[int]textinput.Model),
        scrollOffsets:    make(map[int]int),
        // History fields remain nil/zero
    }
}

// History mode constructor
func NewHistoryModel(entries []history.ExecutionLogEntry) Model {
    return Model{
        state:                StateHistory,
        historyEntries:       entries,
        selectedHistoryIndex: 0,
        reExecuteFromHistory: false,
        // Navigation fields remain nil/zero
    }
}
```

**Benefits**:
- **Clear intent**: Calling code explicitly chooses mode.
- **Correct initialization**: Each mode gets appropriate fields set.
- **Type safety**: Can't accidentally create invalid state.

### Branched Update

Update method delegates based on state:

```go
// internal/tui/model.go

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch m.state {
    case StateNavigation:
        return m.handleNavigationUpdate(msg)
    case StateHistory:
        return m.handleHistoryUpdate(msg)
    default:
        return m, nil
    }
}
```

**Navigation Update Handler**:

```go
// internal/tui/update.go

func (m Model) handleNavigationUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "up", "down":
            return m.handleVerticalNavigation(msg.String()), nil
        case "left", "right":
            return m.handleHorizontalNavigation(msg.String()), nil
        case "enter":
            return m.handleCommandExecution(), nil
        case "/":
            return m.handleFilterActivation(), nil
        // ... more navigation keys
        }
    case tea.WindowSizeMsg:
        return m.handleWindowResize(msg), nil
    }
    return m, nil
}
```

**History Update Handler**:

```go
// internal/tui/update.go

func (m Model) handleHistoryUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c", "esc":
            return m, tea.Quit
        case "up":
            return m.handleHistoryNavigationUp(), nil
        case "down":
            return m.handleHistoryNavigationDown(), nil
        case "enter":
            return m.handleHistorySelection(), tea.Quit
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil
    }
    return m, nil
}
```

**Benefits**:
- **Separation**: Each mode's input handling isolated.
- **Clarity**: Easy to see which keys do what in each mode.
- **Maintainability**: Changes to one mode don't affect other.
- **Testability**: Can test each handler independently.

### Branched View

View method delegates based on state:

```go
// internal/tui/model.go

func (m Model) View() string {
    if !m.ready {
        return "Initializing..."
    }

    switch m.state {
    case StateNavigation:
        return m.renderNavigationView()
    case StateHistory:
        return m.renderHistoryView()
    default:
        return "Unknown state"
    }
}
```

**Navigation View**:

```go
// internal/tui/view.go

func (m Model) renderNavigationView() string {
    calc := LayoutCalculator{}
    renderer := Renderer{}

    // Render breadcrumbs
    breadcrumbs := m.navigator.GenerateBreadcrumbs(m.navState)
    breadcrumbsView := renderer.RenderBreadcrumbs(breadcrumbs)

    // Render columns with sliding window
    columnsView := m.renderColumns(calc, renderer)

    // Render footer
    footerView := renderer.RenderFooter()

    return lipgloss.JoinVertical(
        lipgloss.Left,
        breadcrumbsView,
        columnsView,
        footerView,
    )
}
```

**History View**:

```go
// internal/tui/view_history.go

func (m Model) renderHistoryView() string {
    styles := newHistoryTableStyles()

    // Render header
    header := m.renderHistoryTableHeader(styles)

    // Render rows
    rows := m.renderHistoryTableRows(styles)

    // Render footer with help
    footer := styles.footerStyle.Render("↑/↓: Navigate • Enter: Execute • q/Esc: Quit")

    return lipgloss.JoinVertical(
        lipgloss.Left,
        header,
        rows,
        footer,
    )
}
```

**Benefits**:
- **Visual separation**: Each mode has distinct appearance.
- **Dedicated rendering logic**: Complex rendering isolated per mode.
- **Reusable components**: Can extract shared rendering helpers.

### Mode Transitions

Currently, modes don't transition (single-mode sessions). Future support possible:

```go
// Example: Future mode switching
func (m Model) switchToHistoryMode() (Model, tea.Cmd) {
    // Save navigation state
    savedNavigator := m.navigator
    savedNavState := m.navState

    // Load history
    entries, _ := history.DefaultService.FilterByProject(projectRoot)

    // Transition to history mode
    m.state = StateHistory
    m.historyEntries = entries
    m.selectedHistoryIndex = 0

    return m, nil
}
```

### Exit Behavior

Each mode has different exit semantics:

**Navigation Mode**:
- `q`, `ctrl+c`: Exit without executing.
- `enter`: Execute selected command, then exit.

**History Mode**:
- `q`, `ctrl+c`, `esc`: Exit without executing.
- `enter`: Mark entry for re-execution, then exit.

Exit flag checked in cmd layer:

```go
// cmd/root.go

finalModel, _ := program.Run()

// Check if user selected history entry to re-execute
if finalModel.ShouldReExecuteFromHistory() {
    entry := finalModel.GetSelectedEntry()
    // Execute entry.Command at entry.AbsolutePath
}
```

### Testing Strategy

**Unit Tests**:

```go
// Test state transitions
func TestModel_StateTransitions(t *testing.T) {
    // Test navigation mode creation
    navModel := NewModel(root, 10)
    assert.Equal(t, StateNavigation, navModel.state)

    // Test history mode creation
    histModel := NewHistoryModel(entries)
    assert.Equal(t, StateHistory, histModel.state)
}

// Test update delegation
func TestModel_UpdateDelegation(t *testing.T) {
    // Test navigation mode handles nav keys
    navModel := NewModel(root, 10)
    updated, _ := navModel.Update(tea.KeyMsg{Type: tea.KeyDown})
    // Assert navigation state changed

    // Test history mode handles different keys
    histModel := NewHistoryModel(entries)
    updated, _ := histModel.Update(tea.KeyMsg{Type: tea.KeyDown})
    // Assert history index changed
}
```

**Integration Tests**:

```go
// Test full navigation workflow
func TestNavigationMode_FullWorkflow(t *testing.T) {
    model := NewModel(root, 10)

    // Simulate navigation
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

    // Assert correct command selected
}

// Test history selection workflow
func TestHistoryMode_Selection(t *testing.T) {
    entries := []ExecutionLogEntry{...}
    model := NewHistoryModel(entries)

    // Navigate to second entry
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})

    // Select entry
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

    // Assert correct entry selected
    assert.True(t, model.ShouldReExecuteFromHistory())
    assert.Equal(t, entries[1], *model.GetSelectedEntry())
}
```

## Consequences

### Positive

- **Single codebase**: Both modes in one binary.
- **Shared infrastructure**: Common Bubble Tea setup, styling, window handling.
- **Clear separation**: State machine makes mode explicit.
- **Maintainable**: Mode-specific code isolated in separate handlers.
- **Testable**: Each mode's logic independently testable.
- **Extensible**: Easy to add new modes (config editor, logs viewer, etc.).
- **Type-safe**: Compiler catches invalid state transitions.
- **Debuggable**: Can log current state, easy to trace issues.

### Negative

- **Nil fields**: Navigation fields nil in history mode, vice versa (potential for nil pointer bugs).
- **Larger Model**: Single Model struct contains all fields for all modes.
- **Mode coupling**: Changes to Model affect both modes (must be careful).
- **Testing complexity**: Must test both modes' behavior in shared Model.

### Neutral

- **State enum overhead**: Minimal performance cost for state checking.
- **No runtime transitions**: Currently modes don't switch mid-session (may add later).

## Alternatives Considered

### Alternative 1: Two Separate Applications

Create `terrax` and `terrax-history` as separate binaries.

**Pros**:
- Complete separation (no shared code complexity).
- Each optimized for its purpose.
- Smaller binaries.

**Cons**:
- Code duplication (Bubble Tea setup, styling, input handling).
- Two binaries to maintain and distribute.
- Inconsistent UX between apps.
- Users must know about two separate commands.

**Decision**: Single binary with modes is more cohesive.

### Alternative 2: Completely Separate Model Structs

Create `NavigationModel` and `HistoryModel` as distinct types.

```go
type NavigationModel struct { ... }
type HistoryModel struct { ... }

func (n NavigationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
func (h HistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
```

**Pros**:
- Strong type separation.
- No nil fields (each model has only what it needs).
- Clear interface boundaries.

**Cons**:
- Cannot transition between modes at runtime.
- Duplicate common fields (width, height, ready).
- More complex program initialization (which model to create?).
- Harder to add shared functionality.

**Decision**: Single model with state field is more flexible.

### Alternative 3: Mode as Interface

Define `Mode` interface, implement as `NavigationMode` and `HistoryMode`.

```go
type Mode interface {
    Update(msg tea.Msg) (Mode, tea.Cmd)
    View() string
}

type Model struct {
    currentMode Mode
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    newMode, cmd := m.currentMode.Update(msg)
    m.currentMode = newMode
    return m, cmd
}
```

**Pros**:
- Polymorphic behavior.
- Clean separation of modes.
- Easy to add new modes (just implement interface).

**Cons**:
- More abstraction than needed for two modes.
- Harder to share state between modes.
- Complex type assertions if modes need different data.
- More files and indirection.

**Decision**: Simple state enum is sufficient and more straightforward.

### Alternative 4: Command Pattern for Mode Switching

Use command pattern to encapsulate mode transitions.

**Pros**:
- Flexible transition logic.
- Can undo/redo mode changes.

**Cons**:
- Overkill for current requirements (no mid-session transitions).
- Adds complexity with little benefit.

**Decision**: Not needed for current use case.

### Alternative 5: Unified View (No Mode Distinction)

Show both navigation and history in single view (e.g., split panes).

**Pros**:
- No mode switching needed.
- See both at once.

**Cons**:
- Cluttered UI (too much info on screen).
- Confusing navigation (which pane is focused?).
- Limited terminal space.

**Decision**: Dedicated modes provide clearer UX.

## Future Enhancements

**Potential Improvements**:
1. **Runtime mode switching**: Switch from navigation to history and back with hotkey.
2. **Mode stack**: Support modal overlays (e.g., help dialog over navigation).
3. **Config editor mode**: Interactive configuration editing.
4. **Logs viewer mode**: Stream Terragrunt logs in TUI.
5. **Diff viewer mode**: Show Terraform plan diffs.
6. **Mode history**: Remember mode transitions for debugging.

## References

- **State Machine Pattern**: "Design Patterns: Elements of Reusable Object-Oriented Software" by Gang of Four
- **Bubble Tea Documentation**: https://github.com/charmbracelet/bubbletea
- **Elm Architecture**: https://guide.elm-lang.org/architecture/
- **Related ADRs**: [ADR-0001: Bubble Tea Architecture](0001-bubble-tea-architecture.md)
