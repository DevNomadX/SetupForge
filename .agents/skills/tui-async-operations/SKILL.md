---
name: tui-async-operations
description: Guides asynchronous system operation lifecycle, background execution, progress streaming, cancellation, error diagnostics, and Activity Sheet feedback in SetupForge.
---

# TUI Asynchronous Operations Skill

Use this skill when designing, implementing, or reviewing background execution, system transactions (DNF, Flatpak, systemd, downloads), cancellation contexts, progress streaming, error formatting, and the Activity Sheet in SetupForge.

Modifying a Linux system takes time. Package downloads, dependency resolution, and service restarts can take seconds or minutes. SetupForge must **never freeze the UI event loop**, must provide honest progress feedback, and must handle errors and cancellations with graceful recovery.

---

## 1. The Operation Lifecycle

Every system mutation follows an explicit state machine (`docs/UI-UX-DESIGN.md` §12.1):

```text
               ┌──────────┐
               │   IDLE   │
               └────┬─────┘
                    │ User initiates action
                    ▼
               ┌──────────┐
               │ CONFIRM  │ (Destructive/privileged ops require explicit confirmation)
               └────┬─────┘
                    │ User confirms
                    ▼
               ┌──────────┐
               │ PREPARE  │ (Dependency resolution, transaction planning)
               └────┬─────┘
                    │ Plan ready
                    ▼
               ┌──────────┐
               │ RUNNING  │ ◄────── Progress streaming (Downloads, RPM installs)
               └────┬─────┘
          ┌─────────┼─────────┐
          │ Success │ Failure │ Cancellation
          ▼         ▼         ▼
    ┌──────────┐ ┌────────┐ ┌───────────┐
    │COMPLETE  │ │ FAILED │ │ CANCELLED │
    └──────────┘ └────────┘ └───────────┘
```

### Lifecycle States Defined

| State | Visual Indicator | Interaction Capabilities |
|---|---|---|
| **Idle** | Ready state, normal dock hints | Full navigation, row selection, filtering. |
| **Confirm** | Centered modal dialog | `[Cancel]` focused by default; `Esc` cancels safely. |
| **Prepare** | Labeled Braille spinner ("Resolving dependencies… 12/45") | User can cancel (`c` or `Esc`). Stage remains responsive. |
| **Running** | Determinate progress bar or active spinner in Activity Sheet | Dock shows live operation count `▶ 1 running`. Cancellable where supported. |
| **Complete** | Quiet toast message (dock-adjacent, ≤4s) | Returns to Idle state. Installed badges update. |
| **Failed** | 4-Question Error Dialog | Offers actionable forward paths: `[Retry]`, `[Skip]`, `[View Log]`. |
| **Cancelled**| Toast: `⚠ Operation cancelled by user` | Rollback invoked where applicable. No half-finished orphan states. |

---

## 2. Concurrency Invariants in Bubble Tea

### Rule 1: Never Block `Update()`
Never call synchronous network functions, file writes, or shell runners inside `Update()`.

```go
// VIOLATION: Blocks the Bubble Tea event loop!
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if msg, ok := msg.(InstallMsg); ok {
        time.Sleep(5 * time.Second) // UI FROZEN! No spinner ticks, no redraws!
    }
    return m, nil
}
```

### Rule 2: Never Mutate Model State from Arbitrary Goroutines
All state transitions must occur synchronously inside `Update()` upon receipt of a `tea.Msg`.

```go
// VIOLATION: Data race and memory corruption!
go func() {
    m.isInstalling = true // FORBIDDEN! Mutating model outside Update()
}()
```

### Rule 3: Use `tea.Cmd` and Channel Listeners
Spawn operations through `tea.Cmd` closures that return typed messages to the program runtime:

```go
func startInstallCmd(svc appservice.PackageService, pkgs []string) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithCancel(context.Background())
        // Register cancel function in state
        progressCh, errCh := svc.ExecuteTransaction(ctx, pkgs)
        return TransactionStartedMsg{
            CancelFn:   cancel,
            ProgressCh: progressCh,
            ErrCh:      errCh,
        }
    }
}
```

---

## 3. Streaming Progress Events

When an operation emits multiple progress updates (e.g., download percentage, step counts):

```go
type OperationProgressMsg struct {
    OpID    string
    Phase   string  // e.g., "Downloading", "Verifying", "Installing"
    Step    int
    Total   int
    Percent float64
    Message string
}

type OperationCompletedMsg struct {
    OpID    string
    Summary string
}

type OperationFailedMsg struct {
    OpID string
    Err  error
}

// Command that reads one event from the channel and schedules the next read
func waitForNextProgress(opID string, ch <-chan domain.ProgressEvent) tea.Cmd {
    return func() tea.Msg {
        event, ok := <-ch
        if !ok {
            return nil // Channel closed
        }
        return OperationProgressMsg{
            OpID:    opID,
            Phase:   event.Phase,
            Percent: event.Percent,
            Message: event.Message,
        }
    }
}

// In Update:
case OperationProgressMsg:
    m.currentPercent = msg.Percent
    m.currentStepText = msg.Message
    // Continue reading from the channel
    return m, waitForNextProgress(msg.OpID, m.activeProgressCh)
```

---

## 4. Graceful Cancellation & Context Propagation

System operations must be cancellable wherever the underlying tool (DNF, Flatpak, curl) allows:

```go
type Model struct {
    activeCancelFn context.CancelFunc
    cancelling     bool
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "c" && m.activeCancelFn != nil {
            m.cancelling = true
            m.activeCancelFn() // Triggers context cancellation in the background runner
            return m, nil
        }
    case OperationCancelledMsg:
        m.activeCancelFn = nil
        m.cancelling = false
        return m, showToastCmd("Operation cancelled safely")
    }
    return m, nil
}
```

---

## 5. The 4-Question Error Format

Raw error strings like `exit status 1` or DNF python tracebacks **must never be shown as the primary user-facing message**.

Every error dialog must answer four questions in this order (`docs/UI-UX-DESIGN.md` §12.4):

```text
┌─ Installation Failed ────────────────────────────────┐
│                                                      │
│  1. WHAT HAPPENED:                                   │
│  Could not install 3 of 5 requested packages.        │
│                                                      │
│  2. WHY IT HAPPENED:                                 │
│  libxcrypt-devel conflicts with an existing file     │
│  owned by libxcrypt-compat on Fedora 43.             │
│                                                      │
│  3. WHAT WAS AFFECTED:                               │
│  Affected: libxcrypt-devel, krb5-devel               │
│                                                      │
│  4. WHAT YOU CAN DO NEXT:                            │
│  [Retry]   [Skip Conflicting]   [View Log in Journal]│
│                                                      │
│  Details: file /usr/lib64/libcrypt.so.1 conflict...  │
└──────────────────────────────────────────────────────┘
```

### Actionable Recovery Buttons
Every error dialog must offer forward actions:
- `[Retry]`: Re-run the operation.
- `[Skip]`: Exclude the failing package and continue with remaining tasks.
- `[View Log]`: Jump directly to the Journal destination (`6`) to inspect full stdout/stderr logs.
- `[Dismiss]`: Close dialog without changing system state.

---

## 6. The Activity Sheet (`` ` ``)

The Activity Sheet is a collapsible bottom panel that hosts running operations and recent logs:

```text
 Journal ────────────────────────────── filter: severity ≥ warn ──
 12:04:03  ✓ dnf  Install git-2.49.0-1.fc43.x86_64         14s
 12:03:41  ⚠ dnf  Repo 'terra' unreachable (retry 2/3)      —      running ▶
 12:01:12  ✗ svc  Enable docker.socket                     failed
```

### Activity Sheet Rules:
1. **Shortcut**: Toggle with backtick (`` ` ``).
2. **Auto-Open**: Automatically slides up when a multi-step background operation begins.
3. **Non-Blocking**: The user can continue browsing other packages or surfaces while operations run in the Activity Sheet.
4. **Log Deep-Linking**: Pressing `Enter` on any entry in the sheet jumps directly to that transaction in the Journal.

---

## 7. Success Feedback: Quiet Toasts

Success states should be quiet and unobtrusive:
- Rendered dock-adjacent in single-line format: `✓ git installed successfully`.
- Disappears automatically after ≤ 4 seconds.
- Never use full-screen modals to announce routine success.

---

## 8. Anti-Patterns to Prevent

| Anti-Pattern | Consequence | Prevention |
|---|---|---|
| **Blocking Network / Disk Calls** | Freezes the Bubble Tea event loop; crashes responsiveness. | Always wrap in `tea.Cmd` and pass `context.Context`. |
| **Bare "Busy" Spinners** | Leaves user unsure whether the machine is doing work or hung. | Always label the spinner with the specific step being performed. |
| **Raw Exit Codes in UI** | `Error: exit status 1` provides zero actionable guidance to the user. | Parse output into the 4-question error dialog format. |
| **Uncancellable Operations** | User is trapped waiting for a hung mirror or DNS failure. | Pass `context.WithCancel` to every process executor. |
| **Modal Spam for Success** | Interrupts workflow after every routine command. | Deliver quiet dock-adjacent toasts for success. |

---

## 9. Async Operation Checklist

- [ ] Background operations run in `tea.Cmd` outside the `Update` loop.
- [ ] No goroutines mutate model fields directly.
- [ ] Cancellation context (`context.Context`) is propagated to the Application Service.
- [ ] Progress events stream via channels to `OperationProgressMsg`.
- [ ] Spinners are accompanied by clear human-readable phase descriptions.
- [ ] Errors follow the 4-question format (What, Why, Affected, Next Actions).
- [ ] Error dialogs provide recovery options (`[Retry]`, `[Skip]`, `[View Log]`).
- [ ] Success delivers a brief toast (≤4s) rather than a disruptive modal.
- [ ] Activity Sheet (`` ` ``) reflects active tasks and allows cancellation.
