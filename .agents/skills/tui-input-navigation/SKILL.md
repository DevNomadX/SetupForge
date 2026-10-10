---
name: tui-input-navigation
description: Guides implementation of keyboard-first navigation, the 12 core keys, focus management, overlays, dialog focus traps, and mouse interaction in SetupForge.
---

# TUI Input & Navigation Skill

Use this skill when designing, implementing, or reviewing keyboard handling, navigation tiers, focus state management, overlays, dialogs, and mouse interaction in SetupForge.

SetupForge is **keyboard-first and mouse-friendly**. A user should be able to navigate the entire system with speed, predictability, and confidence using a small, memorable set of keybindings.

---

## 1. The 12 Core Keys (95% Rule)

SetupForge enforces a unified keymap contract across all screens (`docs/UI-UX-DESIGN.md` §5.2). Twelve keys cover 95% of routine usage:

| Key | Scope | Action | Visual / State Effect |
|---|---|---|---|
| `1`–`7` | Global | Jump directly to rail destination | Switches page instantly; restores previous focus |
| `Tab` / `Shift+Tab` | Global | Cycle pane focus in reading order | Moves accent border to next pane (`Rail → Main → Inspector`) |
| `j / k` or `↑ / ↓` | Stage Panes | Move row selection | Moves reverse video row and `❯` cursor |
| `h / l` or `← / →` | Trees / Tabs | Collapse/expand tree; cycle tabs | Expands `▸` to `▾`; switches sub-tab |
| `Enter` | Contextual | Activate selection / drill into details | Opens inspector, submits form, or confirms |
| `Esc` | Global | Unwind one level / close overlay | Closes modal, exits filter, or returns to parent view |
| `/` | Browsable Panes | Focus search / filter input | Jumps focus to search bar; starts typing |
| `Space` | Tables / Trees | Toggle mark for batch actions | Toggles batch glyph `●` on selected item |
| `a` / `A` | Tables | Mark all / unmark all | Selects or clears all items in active table |
| `i` | Master–Detail | Toggle inspector pane | Opens / closes side inspector; updates stage layout |
| `r` | Global | Refresh current surface | Triggers background query; shows dock indicator |
| `Ctrl+K` | Global | Open Command Palette | Displays centered palette overlay |
| `Ctrl+B` | Global | Toggle rail sidebar (S breakpoint) | Opens / closes overlay rail on small screens |
| `?` | Global | Open contextual help | Displays key help dialog |
| `Ctrl+Q` | Global | Quit SetupForge | Exits app (prompts confirm if operations run) |

> [!IMPORTANT]
> **Anti-Pattern A1 (Hidden Actions)**:
> Never introduce an action that cannot be discovered. If an action has a keybinding, it must appear in the Dock hints, in `?` contextual help, or in the `Ctrl+K` command palette.

---

## 2. Navigation Tiers & Depth

SetupForge structures navigation into five explicit tiers:

```text
┌─ Tier 1: Primary ──────── Rail destinations 1–7 (Overview, Software, ...)
├─ Tier 2: Secondary ────── Tabs within a destination (Installed, Updates, Repos)
├─ Tier 3: Contextual ───── Pane focus (Tab), Row selection (j/k), Inspector (i)
├─ Tier 4: Command ──────── Command Palette (Ctrl+K)
└─ Tier 5: Global Overlays ─ Help (?), Quit (Ctrl+Q), Activity Sheet (`)
```

### Depth Rules
- **Maximum Depth**: `Rail → Tab → Row → Inspector`. No task in SetupForge requires more than three levels of drilling.
- **Single-Hop Unwinding**: Pressing `Esc` unwinds exactly one level per press:
  - If a dialog is open → `Esc` closes the dialog.
  - If a filter is active → `Esc` clears filter text and restores full list.
  - If in S-breakpoint drill-down detail → `Esc` returns to master table.
  - `Esc` **NEVER** quits the application from the main workspace.
- **Palette as Depth Bypass**: Any action reachable at Depth 3 must also be reachable at Depth 1 via the Command Palette (`Ctrl+K`).

---

## 3. The Single-Focus Model

At any given millisecond, **exactly ONE element has focus**:

```text
Focus Candidates:
├── Rail navigation
├── Primary stage pane (Table, Form, Tree)
├── Secondary stage pane (Inspector, Log viewport)
├── Modal Dialog / Command Palette (Focus trap)
└── Text Input field (Filter bar)
```

### 3.1 Focus Indicators
- **Panes**: The focused pane receives `color.focus` (accent) border and bold title. Unfocused panes render with `color.border`. Exactly **one** pane has an accent border per screen.
- **Table / List Rows**: The focused row renders in **reverse video** (`color.selection`) with `❯` prefix.
- **Form Inputs**: The focused input renders an accent left bar indicator `▌` (`color.focus`).

### 3.2 Focus Restoration (Memory on Return)
SetupForge workspaces remember their state. Navigating from `Software` to `System` and back to `Software` must restore:
- The exact table row selected (`SelectedIndex`).
- The scroll position (`ScrollOffset`).
- Active filter text and filtered item set.
- Inspector open/closed state.

```go
type SoftwarePageState struct {
    ActiveTab        int
    SelectedRowIndex int
    FilterText       string
    InspectorOpen    bool
    TableScrollPos   int
}
```

---

## 4. Overlays & Focus Traps

Dialogs, Sheets, and the Command Palette are **focus traps**:

```text
Modal Dialog Open
      │
      ├── Captures ALL keyboard input (j/k, Tab, Enter, Esc)
      ├── Prevents stage panes behind it from reacting to keys
      ├── Dims background stage with color.backdrop
      └── Tab cycles only between dialog buttons: [Cancel] [Confirm]
```

### Focus Trap Rules:
1. `Tab` cycles between interactive elements inside the dialog.
2. `Enter` activates the currently selected dialog action.
3. `Esc` **always exits the modal without saving changes**.
4. The safe button (e.g. `[Cancel]`) is ALWAYS pre-selected by default. Destructive actions (`[Delete]`) are NEVER pre-selected.

---

## 5. Command Palette (`Ctrl+K`)

The Command Palette is the universal discoverability layer for all SetupForge actions.

```text
┌─ Commands ───────────────────────────────────────────────┐
│ ❯ upg                                                      │
├────────────────────────────────────────────────────────────┤
│ Update system                              System  Ctrl+U  │
│ Upgrade all packages                       Software        │
│ Manage system services                     System          │
├────────────────────────────────────────────────────────────┤
│ ▸ Update system                                            │
│   Downloads 245 packages (1.2 GiB) · requires root · 4 min │
└────────────────────────────────────────────────────────────┘
```

### Palette Implementation Requirements:
1. **Fuzzy Search**: Matches commands by title, category, and synonyms (`upg` matches "Upgrade all packages").
2. **Contextual Grouping**: Commands contextual to the currently focused page/item appear at the top.
3. **Consequence Preview Line**: The bottom slot of the palette shows what the command does, whether it requires root, and estimated time.
4. **Destructive Commands**: Destructive actions carry a red `!` prefix. Executing a destructive command from the palette opens a confirmation dialog—it never executes destructive acts directly.

---

## 6. Selection vs. Marking (Batch Operations)

SetupForge distinguishes between the active cursor and batch marks:

| State | Visual Representation | Keybinding | Purpose |
|---|---|---|---|
| **Selected** | Reverse video row + `❯` cursor | `j / k` or `↑ / ↓` | The active target of single-item inspection (`i`) or action (`Enter`). |
| **Marked** | Accent glyph `●` at row start | `Space` (toggle), `a` (all) | Chosen for a batch transaction (e.g., install 5 marked packages). |

- Marking persists when the cursor moves away.
- The Dock status region immediately reflects marked count: `● 5 packages marked`.

---

## 7. Mouse Interaction (Additive Support)

Mouse support complements the keyboard without compromising keyboard speed:

- **Click on Rail**: Switches to that destination (equivalent to pressing `1`–`7`).
- **Click on Tab**: Switches sub-tab.
- **Click on Table Row**: Moves selection to that row; double-click activates (`Enter`).
- **Click on Dialog Button**: Triggers button action.
- **Click on Dock Hint**: Triggers that action.
- **Scroll Wheel**: Scrolls the viewport/table currently under the mouse cursor.
- **Click on Unfocused Pane**: Moves focus to that pane (equivalent to `Tab`).

---

## 8. Anti-Patterns to Prevent

| Anti-Pattern | Consequence | Prevention |
|---|---|---|
| **Ambiguous Focus** | User cannot tell where keystrokes will land; multiple panes look focused. | Exactly one pane has `color.focus` border; exactly one row has reverse video. |
| **Esc Quits Entire App** | User accidentally closes SetupForge when trying to dismiss a popup. | `Esc` unwinds one level; only `Ctrl+Q` quits the application. |
| **Focus Amnesia** | Navigating back to a page resets the list to row 0 and clears filters. | Store and restore page state in the parent workspace model. |
| **Pre-Selected Destructive Button** | User pressing Enter carelessly triggers package removal or file deletion. | Always pre-select `[Cancel]`; require explicit navigation to destructive button. |
| **Overloading Standard Keys** | Binding `j` or `k` to actions other than navigation in browsable lists. | Preserve the standard key contract across all screens. |

---

## 9. Input & Navigation Verification Checklist

- [ ] Core 12 keys work predictably on every page.
- [ ] Exactly one element has visible focus at all times.
- [ ] `Tab` cycles pane focus in logical reading order.
- [ ] `Esc` unwinds one level without quitting the app.
- [ ] Dialogs trap focus; `Esc` cancels; `[Cancel]` is default focused.
- [ ] Navigating away and returning restores cursor, scroll, and filter state.
- [ ] `Ctrl+K` command palette opens cleanly and displays consequence previews.
- [ ] Destructive actions prompt for confirmation before execution.
- [ ] Mouse clicks and wheel scrolling function properly where exposed.
