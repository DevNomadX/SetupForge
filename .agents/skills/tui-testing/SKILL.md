---
name: tui-testing
description: Guides unit testing, state transition testing, command verification, terminal dimension testing, and fake service mocking for SetupForge TUI.
---

# TUI Testing Skill

Use this skill when writing, maintaining, or reviewing automated tests for Bubble Tea models, components, layout math, keyboard navigation, and asynchronous UI workflows in SetupForge.

This skill is the authoritative implementation guide for [`.agents/rules/testing.md`](../../rules/testing.md).

> [!CAUTION]
> **Zero Host Mutation Invariant**:
> Normal unit and UI tests must NEVER execute real `dnf`, `flatpak`, `systemctl`, `sudo`, or mutate the developer's real Fedora installation. All system operations must be tested using in-memory fake services and adapters.

---

## 1. The Four TUI Testing Layers

```text
┌────────────────────────────────────────────────────────┐
│ 1. Pure Model State Tests                              │
│    - Test Update(msg) transitions in memory            │
│    - Assert updated model fields and selection indices │
├────────────────────────────────────────────────────────┤
│ 2. Command & Async Effect Tests                        │
│    - Execute cmd() synchronously in unit test          │
│    - Assert the emitted typed tea.Msg                  │
├────────────────────────────────────────────────────────┤
│ 3. Keyboard & Navigation Flow Tests                    │
│    - Feed sequences of tea.KeyMsg (j, k, Tab, Enter)   │
│    - Verify focus routing, modal traps, and unwinding  │
├────────────────────────────────────────────────────────┤
│ 4. Responsive Layout & Geometry Tests                  │
│    - Send tea.WindowSizeMsg (80×24, 100×30, 120×35)    │
│    - Verify lipgloss.Width(view) <= window.Width       │
│    - Verify lipgloss.Height(view) <= window.Height     │
└────────────────────────────────────────────────────────┘
```

---

## 2. Testing `Update()` State Transitions

Because Bubble Tea's `Update` method is a pure function `(msg) -> (Model, Cmd)`, testing state transitions requires no mock TTY, no terminal emulator, and no external dependencies:

```go
package ui_test

import (
    "testing"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestPackageTable_Navigation(t *testing.T) {
    fakeSvc := &fakePackageService{
        packages: []domain.Package{
            {Name: "git", Version: "2.49.0"},
            {Name: "ripgrep", Version: "14.1.0"},
            {Name: "tmux", Version: "3.5a"},
        },
    }

    m := ui.NewPackagePage(fakeSvc)
    m.Init()

    // Simulate pressing 'j' (Down arrow)
    downKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
    updatedModel, cmd := m.Update(downKey)
    pkgModel := updatedModel.(ui.PackagePageModel)

    assert.Nil(t, cmd)
    assert.Equal(t, 1, pkgModel.SelectedIndex(), "Cursor should move to row 1 (ripgrep)")

    // Simulate pressing 'Space' (Batch mark)
    spaceKey := tea.KeyMsg{Type: tea.KeySpace}
    updatedModel, _ = pkgModel.Update(spaceKey)
    pkgModel = updatedModel.(ui.PackagePageModel)

    assert.True(t, pkgModel.IsMarked("ripgrep"), "ripgrep should be marked")
}
```

---

## 3. Testing `tea.Cmd` and Async Workflows

Commands are simply functions returning a `tea.Msg` (`func() tea.Msg`). You can test them directly by calling them synchronously:

```go
func TestLoadPackagesCmd_Success(t *testing.T) {
    fakeSvc := &fakePackageService{
        packages: []domain.Package{
            {Name: "git", Version: "2.49.0"},
        },
    }

    cmd := ui.LoadPackagesCmd(fakeSvc)
    require.NotNil(t, cmd)

    // Execute the command synchronously in test
    msg := cmd()

    loadedMsg, ok := msg.(ui.PackagesLoadedMsg)
    require.True(t, ok, "Command should emit PackagesLoadedMsg")
    assert.Len(t, loadedMsg.Packages, 1)
    assert.Equal(t, "git", loadedMsg.Packages[0].Name)
}

func TestLoadPackagesCmd_Failure(t *testing.T) {
    fakeSvc := &fakePackageService{
        err: errors.New("network unreachable"),
    }

    cmd := ui.LoadPackagesCmd(fakeSvc)
    msg := cmd()

    errMsg, ok := msg.(ui.OperationErrorMsg)
    require.True(t, ok, "Command should emit OperationErrorMsg on error")
    assert.Contains(t, errMsg.Err.Error(), "network unreachable")
}
```

---

## 4. Testing Responsive Layouts & Bounds

Layout tests ensure that views never overflow their allocated terminal bounds and that responsive mode switches trigger correctly:

```go
func TestResponsiveBreakpoints_LayoutMath(t *testing.T) {
    testCases := []struct {
        name             string
        width            int
        height           int
        expectSmall      bool
        expectRailWidth  int
        expectedColumns  int
    }{
        {"Compact Console", 80, 24, true, 0, 3},     // S Breakpoint
        {"Tiled Window", 100, 30, false, 14, 4},      // M Breakpoint
        {"Standard Desktop", 120, 35, false, 18, 5},  // L Breakpoint
        {"Ultrawide High-DPI", 160, 45, false, 18, 6},// XL Breakpoint
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            fakeSvc := &fakePackageService{}
            m := ui.NewAppModel(fakeSvc)

            // Send window size message
            sizeMsg := tea.WindowSizeMsg{Width: tc.width, Height: tc.height}
            updatedModel, _ := m.Update(sizeMsg)
            app := updatedModel.(ui.AppModel)

            rendered := app.View()

            // Verify visual boundaries
            actualWidth := lipgloss.Width(rendered)
            actualHeight := lipgloss.Height(rendered)

            assert.LessOrEqual(t, actualWidth, tc.width, "Rendered view width must not exceed terminal width")
            assert.LessOrEqual(t, actualHeight, tc.height, "Rendered view height must not exceed terminal height")

            // Verify mode flags
            assert.Equal(t, tc.expectSmall, app.Layout().IsSmall)
            assert.Equal(t, tc.expectRailWidth, app.Layout().RailWidth)
        })
    }
}
```

---

## 5. Testing Focus Traps & Overlays

Verify that dialogs capture input and that `Esc` cancels safely:

```go
func TestConfirmationDialog_FocusTrap(t *testing.T) {
    fakeSvc := &fakePackageService{}
    m := ui.NewAppModel(fakeSvc)

    // Open confirmation dialog
    m, _ = m.Update(ui.PromptConfirmMsg{Title: "Remove git?"})
    assert.True(t, m.HasActiveDialog(), "Dialog should be open")

    // Press Tab: should cycle to [Confirm] button
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
    assert.Equal(t, ui.DialogButtonConfirm, m.ActiveDialogButton())

    // Press Esc: should dismiss dialog without executing
    m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
    assert.False(t, m.HasActiveDialog(), "Dialog should be dismissed")
    assert.Nil(t, cmd, "No system action should be scheduled on Esc")
}
```

---

## 6. Fake Service Mocking Pattern

Keep fake implementations clean, deterministic, and free of external side effects:

```go
type fakePackageService struct {
    packages   []domain.Package
    err        error
    installErr error
    installed  []string
}

func (f *fakePackageService) ListInstalled(ctx context.Context) ([]domain.Package, error) {
    if f.err != nil {
        return nil, f.err
    }
    return f.packages, nil
}

func (f *fakePackageService) Install(ctx context.Context, pkgs []string, ch chan<- domain.ProgressStep) error {
    if f.installErr != nil {
        return f.installErr
    }
    f.installed = append(f.installed, pkgs...)
    return nil
}
```

---

## 7. Anti-Patterns to Prevent

| Anti-Pattern | Consequence | Prevention |
|---|---|---|
| **Direct OS Commands in Tests** | Tests modify the developer's workstation or require root. | Mock domain interfaces with in-memory fakes. |
| **Snapshot-Only UI Tests** | Fragile tests break whenever a single border character or space changes. | Assert semantic state transitions, model fields, and dimension bounds. |
| **Testing Concatenated Shell Strings** | Ignores argument injection risks and misrepresents how structured commands execute. | Test argument vectors `[]string{"dnf5", "install", "-y"}` directly. |
| **Unchecked Window Resizing** | View functions silently panic on small dimensions or clip when resized. | Include tests for all 4 benchmark sizes (80×24, 100×30, 120×35, 160×45+). |
| **Sleeping in Tests** | `time.Sleep()` makes test suites slow, flaky, and non-deterministic. | Execute commands synchronously or use buffered channels. |

---

## 8. TUI Testing Checklist

- [ ] All UI model tests run purely in memory without terminal emulation.
- [ ] No test runs `exec.Command` or invokes real `dnf`/`flatpak`.
- [ ] State transitions tested for all primary keybindings (`j/k`, `Tab`, `Enter`, `Space`, `Esc`).
- [ ] Commands tested by executing `cmd()` synchronously and asserting typed result messages.
- [ ] Layout dimensions verified at 80×24, 100×30, 120×35, and 160×45+.
- [ ] Rendered views verified with `lipgloss.Width()` and `lipgloss.Height()`.
- [ ] Dialog focus traps and `Esc` cancellation paths tested.
- [ ] Error states tested using the 4-question error format.
- [ ] `go test ./...` passes cleanly and deterministically.
