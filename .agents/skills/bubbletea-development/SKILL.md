---
name: bubbletea-development
description: Guides implementation, refactoring, and review of Bubble Tea models, commands, messages, state transitions, component composition, input handling, and lifecycle in SetupForge.
---

# Bubble Tea Development Skill

Use this skill when implementing, refactoring, or reviewing Bubble Tea models, messages, commands, state transitions, child components, and event handling in SetupForge.

Bubble Tea (`github.com/charmbracelet/bubbletea`) provides the Elm Architecture runtime for SetupForge. In this architecture:
- State is modeled in explicit Go structs.
- UI transitions occur exclusively through pure `Update` functions handling typed messages.
- Side effects (I/O, timers, application services) are isolated in `tea.Cmd` closures.
- Rendering in `View` is a pure function of current state.

---

## 1. Architectural Boundary (Non-Negotiable)

SetupForge enforces a strict four-layer architecture. Bubble Tea belongs **strictly to the Presentation layer**:

```text
Presentation Layer (Bubble Tea / Bubbles / Lip Gloss)
      ↓ (invokes use cases / commands)
Application Service Layer (Orchestration, confirmation, cancellation, transactions)
      ↓ (interacts via domain interfaces)
Domain Layer (Interfaces, models, value objects, business rules)
      ↓ (implemented by platform adapters)
Infrastructure Layer (Command runner, DNF/RPM/Flatpak adapters, systemd, filesystem)
      ↓
Fedora Host System
```

### Responsibility Matrix

| Concept | Belongs In | Must NOT Contain |
|---|---|---|
| **Bubble Tea Model** | Active focus, selection index, scroll offset, UI modes (inspecting, filtering, confirming), child component models, cached presentation data. | Direct OS calls, `exec.Command`, filesystem mutations, DNF queries, business rules. |
| **Child Component** | Encapsulated UI widgets (tables, lists, forms), localized cursor/scroll, local keybindings, component rendering. | Global application state, routing decisions, direct hardware access. |
| **Command (`tea.Cmd`)** | Wrapping asynchronous Application Service calls, timers, channel listeners, message dispatching. | Direct shell interpolation, privileged operations, long blocking synchronous loops in `Update`. |
| **Message (`tea.Msg`)** | Pure data carriers (keyboard events, window resize, service progress, service result, typed errors). | Mutable pointers modified concurrently, internal infrastructure types leaking into views. |
| **Application Service** | Use case orchestration, sequencing operations, confirmation policy checks, cancellation contexts. | Bubble Tea imports, Lip Gloss styles, terminal layout math. |
| **Domain / Platform** | Capability interfaces (`PackageManager`, `OSDetector`), systemd adapters, Fedora DNF runner. | Presentation concepts, keymaps, UI models. |

> [!CAUTION]
> Never execute `dnf`, `rpm`, `flatpak`, `systemctl`, `sudo`, or shell scripts inside a Bubble Tea model or command. Always call the Application Service layer.

---

## 2. Standard Model Lifecycle

Every interactive screen or component implements `tea.Model`:

```go
type Model interface {
    Init() tea.Cmd
    Update(tea.Msg) (tea.Model, tea.Cmd)
    View() string
}
```

### 2.1 `Init() tea.Cmd`
- Executed once when the program starts or when a screen is mounted.
- Returns initial commands to kick off data loading or listen to external events.
- If no initial command is needed, return `nil`.
- When loading initial data, return an Application Service command that emits a typed result message.

```go
func (m PackageListModel) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        m.loadPackagesCmd(),
    )
}
```

### 2.2 `Update(msg tea.Msg) (tea.Model, tea.Cmd)`
- Pure state machine. Receives a message, updates model fields, and returns the new model and optional commands.
- Never perform blocking work (disk, network, system execution) inside `Update`.
- Never start unmanaged goroutines that mutate model state directly.
- Group message handling into a disciplined dispatch hierarchy:

```go
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    // 1. Global lifecycle and quit
    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "ctrl+q":
            return m, tea.Quit
        }

    // 2. Terminal geometry changes
    case tea.WindowSizeMsg:
        return m.handleResize(msg)

    // 3. Global overlay toggles (Palette, Activity Sheet, Help)
    case TogglePaletteMsg:
        return m.togglePalette()

    // 4. Asynchronous system operations & service results
    case OperationProgressMsg:
        return m.handleOperationProgress(msg)
    case OperationCompleteMsg:
        return m.handleOperationComplete(msg)
    case OperationErrorMsg:
        return m.handleOperationError(msg)
    }

    // 5. Active overlay delegation (Dialogs capture all input when open)
    if m.activeDialog != nil {
        var cmd tea.Cmd
        m.activeDialog, cmd = m.activeDialog.Update(msg)
        return m, cmd
    }

    // 6. Active stage page delegation
    var stageCmd tea.Cmd
    m.stage, stageCmd = m.stage.Update(msg)
    cmds = append(cmds, stageCmd)

    return m, tea.Batch(cmds...)
}
```

### 2.3 `View() string`
- Deterministic rendering function. Produces the terminal output string from the current state.
- **Rule**: `View` must be a pure read operation. Never mutate model state, never trigger commands, and never start goroutines inside `View`.
- Compose views using Lip Gloss (`lipgloss.JoinVertical`, `lipgloss.JoinHorizontal`).
- Measure display widths and heights using `lipgloss.Width()` and `lipgloss.Height()`, never byte lengths.

---

## 3. Model Composition and Hierarchy

SetupForge uses a hierarchical model tree:

```text
                  AppModel (Root: Frame, Navigation, Overlays)
                   ├── HeaderView (ambient breadcrumbs, badge, clock)
                   ├── RailModel (destinations 1–7)
                   ├── StageModel (active page container)
                   │     └── SoftwarePageModel
                   │           ├── TableModel (Bubbles table for packages)
                   │           └── InspectorModel (detail side pane)
                   ├── ActivitySheetModel (collapsible operations bottom sheet)
                   ├── PaletteModel (Ctrl+K overlay)
                   └── DockView (contextual key hints & status line)
```

### Parent-Child Communication Rules

1. **Top-Down State Flow**: Parents pass configuration, window sizes, and state downward into children.
2. **Bottom-Up Message Flow**: Children emit typed messages or commands back to the parent.
3. **Narrow Interfaces**: Do not pass the entire root model to children. Pass only the data and dependencies the child needs.
4. **Child Delegation**: Parents selectively delegate `tea.Msg` to active children. Inactive children should not receive keyboard input.

Example child delegation:

```go
func (m SoftwarePageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "tab":
            m.focus = m.focus.Next()
            return m, nil
        case "i":
            m.inspectorVisible = !m.inspectorVisible
            return m, m.syncInspectorSize()
        }
    }

    // Route input to the focused child component
    switch m.focus {
    case FocusTable:
        var cmd tea.Cmd
        m.table, cmd = m.table.Update(msg)
        cmds = append(cmds, cmd)
    case FocusInspector:
        var cmd tea.Cmd
        m.inspector, cmd = m.inspector.Update(msg)
        cmds = append(cmds, cmd)
    }

    return m, tea.Batch(cmds...)
}
```

---

## 4. Asynchronous Commands and Safe Concurrency

All background work (DNF transactions, Flatpak queries, filesystem checks) runs asynchronously via `tea.Cmd`.

### Command Construction Pattern

```go
// 1. Define typed messages for results and errors
type PackagesLoadedMsg struct {
    Packages []domain.Package
}

type OperationErrorMsg struct {
    OpTitle string
    Err     error
}

// 2. Command constructor calling the Application Service
func loadPackagesCmd(svc appservice.PackageService) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        pkgs, err := svc.ListInstalled(ctx)
        if err != nil {
            return OperationErrorMsg{
                OpTitle: "Loading packages",
                Err:     err,
            }
        }
        return PackagesLoadedMsg{Packages: pkgs}
    }
}
```

### Safe Streaming / Progress Pattern

When an operation produces multiple progress events over time (e.g., package downloads):

```go
type InstallProgressMsg struct {
    ItemName string
    Percent  float64
    ETA      string
}

type InstallFinishedMsg struct {
    PackageName string
    Success     bool
}

// Stream progress from an Application Service channel
func listenToProgress(ch <-chan domain.ProgressEvent) tea.Cmd {
    return func() tea.Msg {
        event, ok := <-ch
        if !ok {
            return nil // Channel closed
        }
        return InstallProgressMsg{
            ItemName: event.ItemName,
            Percent:  event.Percent,
            ETA:      event.ETA,
        }
    }
}

// In Update:
case InstallProgressMsg:
    m.progress = msg.Percent
    m.statusText = msg.ItemName
    // Continue listening for the next progress event
    return m, listenToProgress(m.eventChannel)
```

---

## 5. Input Handling Discipline

Keyboard is primary. Mouse is additive. Follow these rules for all screens:

1. **Core 12 Keys**: Respect the project keymap contract (`docs/UI-UX-DESIGN.md` §5.2). Never rebind `1`–`7`, `Tab`, `j/k`, `Enter`, `Esc`, `/`, `Space`, `i`, `r`, `Ctrl+K`, `?`, or `Ctrl+Q` to incompatible local actions.
2. **One Focus Target**: Exactly one pane or field receives active input. Its border is `color.focus` and selection is reverse video.
3. **Esc Unwinds**: `Esc` closes an overlay, cancels search/filter, or moves up one navigation tier. `Esc` NEVER quits the entire application directly.
4. **Quit Confirmation**: `Ctrl+Q` triggers quit. If a system operation is actively running, intercept with an explicit confirmation dialog before terminating.

---

## 6. Anti-Patterns to Prevent

| Anti-Pattern | Why It Fails | What to Do Instead |
|---|---|---|
| **Direct System Commands in UI** | Violates architecture; untestable; dangerous privilege escalation. | Delegate to `ApplicationService` through `tea.Cmd`. |
| **Blocking `Update`** | Freezes terminal rendering, stops spinner animation, makes UI unresponsive. | Run I/O in `tea.Cmd` goroutine; return results via `tea.Msg`. |
| **Monolithic Model Structs** | God structs with 50+ mixed fields become unmaintainable and prone to state bugs. | Break into focused child models (`RailModel`, `TableModel`, `InspectorModel`). |
| **1000-Line `Update` Functions** | Giant unstructured switches make navigation and state transitions untraceable. | Separate into sub-handlers (`handleKey`, `handleResize`, `handleServiceMsg`). |
| **Side Effects in `View()`** | Produces non-deterministic views, double-invocations, and visual glitches. | `View()` must be a pure function. Side effects belong only in `Update()` via `tea.Cmd`. |
| **Direct Goroutine State Mutation** | Race conditions and data corruption when modifying model fields outside `Update`. | Goroutines must communicate solely by returning `tea.Msg` to the runtime. |
| **Hardcoded Terminal Dimensions** | Layout breaks and clips when terminal is resized or running on 80×24. | Calculate sizes dynamically from `tea.WindowSizeMsg`. |

---

## 7. Implementation Checklist

Before submitting Bubble Tea code, verify:

- [ ] Model belongs strictly to the Presentation layer (no `exec.Command`, no `sudo`, no direct DNF).
- [ ] Model implements `Init()`, `Update()`, `View()` with standard signatures.
- [ ] All background work and I/O runs inside `tea.Cmd` with `context.Context`.
- [ ] No goroutines mutate model fields directly.
- [ ] Child components have narrow contracts and receive input only when focused.
- [ ] `View()` is deterministic and side-effect free.
- [ ] `tea.WindowSizeMsg` updates child dimensions dynamically.
- [ ] `Esc` unwinds cleanly without trapping the user or quitting unexpectedly.
- [ ] Destructive actions prompt for confirmation before executing.
- [ ] State transitions are covered by unit tests.
