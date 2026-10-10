---
name: tui-responsive-layout
description: Guides implementation and verification of responsive TUI layouts across 80×24, 100×30, 120×35, and 160×45+ terminal dimensions in SetupForge.
---

# TUI Responsive Layout Skill

Use this skill when implementing, refactoring, or verifying terminal layouts, window-size handling, and breakpoint transformations in SetupForge.

SetupForge is built for real Linux terminals. A user might run SetupForge in an 80×24 virtual console, a 100×30 tiled window, a 120×35 standard desktop terminal, or a 160×45+ ultrawide display.

> [!IMPORTANT]
> **The Core Responsive Principle**:
> *Do not merely squeeze or shrink the same layout. Transform composition modes.*
> Squeezing a two-pane layout into an 80-column terminal results in unreadable, clipped columns and broken wrapping. Instead, switch the interaction mode: collapse sidebars into overlays, drop non-essential table columns, and transform dual-pane splits into sequential drill-downs.

---

## 1. The Four Breakpoint Classes

SetupForge defines four normative breakpoint classes (`docs/UI-UX-DESIGN.md` §9):

| Class | Minimum Grid | Rail Mode | Stage Composition | Key Behavior Changes |
|---|---|---|---|---|
| **S** | **80×24** | Collapsed / Overlay (`Ctrl+B`) | **Single Whole Pane** | Dual-pane transforms into sequential drill-down (`Enter` drills in, `Esc` returns). Drop header clock & badge. Tables keep top-3 columns. Dock shows ≤4 hints. |
| **M** | **100×30** | Pinned (14 cols) | **Half+Half** (55/45 split) or Whole | Tables keep top-4 priority columns. Clock dropped from header; update badge visible. Command palette 80% width. |
| **L** | **120×35** | Pinned (18 cols) | **Half+Half** (2M×4M) | Baseline standard. Master browser + Inspector open by default. Full dock hints. Distro badge and clock visible. |
| **XL** | **160×45+** | Pinned (18 cols) | **Quad Tiles** or Half+Half | Overview stage shows 4 Quad tiles (2M×2M). Tables display full column set (≥6 cols). Persistent rich inspector. |

---

## 2. Dynamic Transformation Rules

When terminal size decreases, apply transformations in this strict order:

```text
Full Desktop View (XL / L)
      ↓ (Terminal width drops below 120 cols)
1. Drop optional header chrome (Clock first, then secondary badges)
2. Reduce Rail width from 18 to 14 cols
3. Drop low-priority table columns (e.g. Size, Architecture)
      ↓ (Terminal drops below 100 cols / S Class: 80×24)
4. Collapse Rail completely into an on-demand sheet (toggle with Ctrl+B)
5. Transform dual-pane (Browser + Inspector) into sequential drill-down:
   - Screen 1: Master Browser (Whole pane, 80 cols)
   - Press Enter → Screen 2: Inspector Detail (Whole pane, 80 cols)
   - Press Esc → Returns to Screen 1
6. Shrink dialogs to 76×11 with safe 2-col margins
7. Dock key hints capped at maximum 4 primary actions
```

---

## 3. Window Size Message Handling (`tea.WindowSizeMsg`)

The root `AppModel` receives `tea.WindowSizeMsg` and calculates the spatial budget for each region:

```go
type ResponsiveBreakpoints struct {
    Width       int
    Height      int
    IsSmall     bool // < 100 cols or < 30 rows
    IsMedium    bool // 100–119 cols
    IsLarge     bool // 120–159 cols
    IsExtraLarge bool // >= 160 cols and >= 45 rows
    RailWidth   int
    StageWidth  int
    StageHeight int
}

func CalculateLayout(msg tea.WindowSizeMsg, railCollapsed bool) ResponsiveBreakpoints {
    bp := ResponsiveBreakpoints{
        Width:  msg.Width,
        Height: msg.Height,
    }

    // Determine breakpoint tier
    if msg.Width < 100 || msg.Height < 30 {
        bp.IsSmall = true
        bp.RailWidth = 0 // In small mode, rail is an overlay, takes 0 stage width
    } else if msg.Width < 120 {
        bp.IsMedium = true
        bp.RailWidth = 14
    } else if msg.Width < 160 || msg.Height < 45 {
        bp.IsLarge = true
        bp.RailWidth = 18
    } else {
        bp.IsExtraLarge = true
        bp.RailWidth = 18
    }

    if railCollapsed && !bp.IsSmall {
        bp.RailWidth = 0
    }

    // Header consumes 1 row, Dock consumes 1 row
    headerRows := 1
    dockRows := 1
    bp.StageHeight = msg.Height - headerRows - dockRows
    if bp.StageHeight < 0 {
        bp.StageHeight = 0
    }

    bp.StageWidth = msg.Width - bp.RailWidth
    if bp.StageWidth < 0 {
        bp.StageWidth = 0
    }

    return bp
}
```

---

## 4. Stage & M-Unit Grid Calculations

Once `StageWidth` and `StageHeight` are known, derive the M-unit values (`docs/DESIGN-SYSTEM.md` §8):

```go
// 1M width and height
unitW := bp.StageWidth / 4
unitH := bp.StageHeight / 4

// Standard pane dimensions
wholePaneWidth  := bp.StageWidth
wholePaneHeight := bp.StageHeight

halfPaneWidth   := bp.StageWidth / 2
halfPaneHeight  := bp.StageHeight

quadPaneWidth   := bp.StageWidth / 2
quadPaneHeight  := bp.StageHeight / 2
```

### Composing Responsive Panes in Page Models

```go
func (m SoftwarePageModel) View(bp ResponsiveBreakpoints) string {
    // Mode Transformation at S Breakpoint (80x24)
    if bp.IsSmall {
        if m.drilldownActive {
            // Render full-width Inspector
            return m.inspector.View(bp.StageWidth, bp.StageHeight)
        }
        // Render full-width Master Browser
        return m.browser.View(bp.StageWidth, bp.StageHeight)
    }

    // Standard Dual-Pane Layout (L / XL)
    leftWidth := bp.StageWidth / 2
    rightWidth := bp.StageWidth - leftWidth // Prevent rounding pixel loss

    leftView := m.browser.View(leftWidth, bp.StageHeight)
    rightView := m.inspector.View(rightWidth, bp.StageHeight)

    return lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
}
```

---

## 5. Responsive Table Columns

Tables must explicitly prioritize columns so low-priority data drops gracefully:

```go
func GetPackageColumns(bp ResponsiveBreakpoints) []table.Column {
    // Priority 1: Name (Always visible)
    // Priority 2: State (Always visible)
    // Priority 3: Version (Dropped on extreme squeeze)
    // Priority 4: Size (Visible on M, L, XL)
    // Priority 5: Repository (Visible on L, XL)
    // Priority 6: Architecture (Visible on XL only)

    if bp.IsSmall {
        // Top 3 priority columns only
        return []table.Column{
            {Title: "Name", Width: 32},
            {Title: "Version", Width: 20},
            {Title: "State", Width: 18},
        }
    }

    if bp.IsMedium {
        // Top 4 priority columns
        return []table.Column{
            {Title: "Name", Width: 30},
            {Title: "Version", Width: 20},
            {Title: "State", Width: 18},
            {Title: "Size", Width: 12},
        }
    }

    if bp.IsLarge {
        // Top 5 columns
        return []table.Column{
            {Title: "Name", Width: 30},
            {Title: "Version", Width: 18},
            {Title: "State", Width: 16},
            {Title: "Size", Width: 12},
            {Title: "Repo", Width: 16},
        }
    }

    // XL: Full 6-column set
    return []table.Column{
        {Title: "Name", Width: 34},
        {Title: "Version", Width: 20},
        {Title: "State", Width: 16},
        {Title: "Size", Width: 12},
        {Title: "Repo", Width: 18},
        {Title: "Arch", Width: 12},
    }
}
```

---

## 6. Sub-Minimum Terminal Guard (`< 80×24`)

When a user resizes their terminal below the supported minimum of 80×24, SetupForge must not crash or render garbled, overlapping text. Render a clean warning screen:

```go
func RenderSubMinimumWarning(width, height int, theme Theme) string {
    msg := fmt.Sprintf(
        "Terminal Too Small\n\n"+
            "Current:  %d × %d\n"+
            "Required: 80 × 24\n\n"+
            "Please enlarge your terminal window.",
        width, height,
    )

    box := lipgloss.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(theme.Warning).
        Padding(1, 3).
        Align(lipgloss.Center).
        Render(msg)

    return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
```

---

## 7. Anti-Patterns to Prevent

| Anti-Pattern | Consequence | Prevention |
|---|---|---|
| **Hardcoded Width/Height** | `width := 120` scattered in views clips on small terminals and leaves large terminals empty. | Always consume dimensions from `tea.WindowSizeMsg`. |
| **Squeezing Dual Panes into 80 Cols** | Two 38-column panes create unreadable wrapping, cut off words, and ruin tables. | Switch to sequential drill-down at `< 100` columns. |
| **Silent Text Truncation Without Inspector** | Columns truncate with `…` with no way for the user to view the full value. | Provide the side Inspector (or drill-down) to display full details. |
| **Ignoring Terminal Height** | Table rows overflow below the dock and get pushed out of sight. | Calculate `stageHeight = termHeight - 2` and configure table/viewport height. |
| **Measuring Bytes Instead of Cells** | ANSI color escapes cause `len()` to drastically overestimate string width. | Always use `lipgloss.Width(str)`. |

---

## 8. Responsive Verification Checklist

Test every screen at all four benchmark dimensions:

- [ ] **80×24 (S)**:
  - Rail is collapsed to overlay (`Ctrl+B`).
  - Dual panes transform to single whole pane + drill-down on `Enter`.
  - Tables show top-3 priority columns without horizontal wrapping.
  - Dock key hints capped at ≤4 items.
- [ ] **100×30 (M)**:
  - Rail is pinned at 14 columns.
  - Header shows breadcrumb and update badge (no clock).
  - Tables show top-4 priority columns.
- [ ] **120×35 (L)**:
  - Rail is pinned at 18 columns.
  - Dual-pane master-detail renders cleanly with zero gutter.
  - Header displays wordmark, breadcrumbs, distro badge, and clock.
- [ ] **160×45+ (XL)**:
  - Overview stage leverages quad tiles or full-width data visualization.
  - Tables display full column set (≥6 columns).
- [ ] **< 80×24**:
  - Centered warning screen renders cleanly without panic or clipping.
