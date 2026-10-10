---
name: bubbles-development
description: Guides selection, composition, customization, and review of Charmbracelet Bubbles components (lists, tables, viewports, text inputs, spinners, progress bars, paginators) in SetupForge.
---

# Bubbles Component Development Skill

Use this skill when selecting, composing, customizing, or reviewing Charmbracelet Bubbles (`github.com/charmbracelet/bubbles`) components in SetupForge.

Bubbles provides reusable, battle-tested terminal UI widgets designed for Bubble Tea. In SetupForge, Bubbles accelerates feature delivery, but every component must conform strictly to our design system (`docs/DESIGN-SYSTEM.md`) and spatial architecture (`docs/UI-UX-DESIGN.md`).

---

## 1. Core Philosophy: Search First, Don't Blindly Build

### The Component Selection Decision Matrix

```text
Requirement arises
      ↓
Is there an existing Bubbles component that fits?
      ├── YES → Can it be styled and configured via its public API?
      │           ├── YES → Use Bubbles component with Design System tokens.
      │           └── NO  → Evaluate if slight customization or a simple Lip Gloss view suffices.
      └── NO  → Is a full interactive model truly needed?
                  ├── YES → Build a custom child tea.Model.
                  └── NO  → Build a pure Lip Gloss layout function (stat cards, breadcrumbs).
```

### When to Use Bubbles vs. Custom Components

| UI Need | Recommended Component | Rationale |
|---|---|---|
| Multi-attribute tabular data (Packages, Services) | `bubbles/table` | Built-in scrolling, column widths, header styles, row selection. |
| 1D selectable collections with fuzzy filter | `bubbles/list` | Built-in search, pagination, status messages, delegate system. |
| Long scrollable read-only text (Logs, Details) | `bubbles/viewport` | Viewport clipping, memory efficiency, pager key navigation. |
| Single-line search / form input | `bubbles/textinput` | Cursor management, character limits, placeholder styling. |
| Multi-line text input | `bubbles/textarea` | Line wrapping, scrolling, multi-line cursor navigation. |
| Indeterminate background work | `bubbles/spinner` | Smooth frame ticking, standard terminal-safe animations. |
| Determinate progress (Downloads, Transactions) | `bubbles/progress` | Percentage calculations, filled/unfilled rendering. |
| Key hint dock / help modal | `bubbles/help` & `bubbles/key` | Standardized key binding declarations and formatted help bars. |
| **Global Rail Navigation** | **Custom Component** | High spatial specificity (numbered destinations, accent bar `▌`). |
| **Application Frame Header** | **Custom Component** | Wordmark, breadcrumbs, ambient badges, clock. |
| **Stat Cards / Key-Value Grids** | **Custom Component** | Lightweight Lip Gloss layout without model overhead. |

> [!IMPORTANT]
> **Anti-Pattern A4**: Do NOT box every list item or card in a separate rectangular border. Use alignment and whitespace (`space.sm`, `space.md`) for grouping.

---

## 2. Component Guidelines & SetupForge Customization

### 2.1 Table (`bubbles/table`)

The primary data grid for packages, services, and repositories.

```go
import "github.com/charmbracelet/bubbles/table"

func NewPackageTable(theme Theme, width, height int) table.Model {
    columns := []table.Column{
        {Title: "Name", Width: 26},
        {Title: "Version", Width: 18},
        {Title: "State", Width: 14},
        {Title: "Size", Width: 10},
    }

    t := table.New(
        table.WithColumns(columns),
        table.WithFocused(true),
        table.WithHeight(height),
    )

    // Customize styles to match SetupForge Design System
    s := table.DefaultStyles()
    s.Header = theme.TypeMeta.Bold(true).BorderBottom(true).BorderStyle(theme.BorderRule)
    s.Selected = theme.ColorSelection.Bold(true) // Reverse video selection
    s.Cell = theme.TypeBody
    t.SetStyles(s)

    return t
}
```

**Table Rules:**
- Row selection must render with **reverse video** and prefix `❯` (`color.selection`).
- Column priority: Define column drop order when screen transitions between L (120×35), M (100×30), and S (80×24).
- Pagination: Show position in the pane border header (`┌─ Packages ─ 4 of 142 ─┐`), never as a cluttered footer row.
- Truncation: Truncate long text with `…`. Show full text in the side Inspector.

### 2.2 List (`bubbles/list`)

Used for 1D selections such as category browsing or repository pickers.

```go
import "github.com/charmbracelet/bubbles/list"

// Define custom item delegate to enforce Design System row appearance
type PackageItem struct {
    name, summary string
}

func (i PackageItem) Title() string       { return i.name }
func (i PackageItem) Description() string { return i.summary }
func (i PackageItem) FilterValue() string { return i.name }

func NewCategoryList(items []list.Item, width, height int, theme Theme) list.Model {
    delegate := list.NewDefaultDelegate()
    delegate.Styles.SelectedTitle = theme.ColorSelection.Bold(true)
    delegate.Styles.SelectedDesc = theme.ColorSelection.Faint(true)

    l := list.New(items, delegate, width, height)
    l.Title = "Categories"
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(true)
    return l
}
```

**List Rules:**
- Use lists for 1D navigation; prefer tables for comparing multi-column attributes.
- Use `❯` prefix and reverse video for active selection.
- Hide default title chrome if the pane border header already displays the title.

### 2.3 Viewport (`bubbles/viewport`)

Used for inspecting package details, reading logs, and markdown help.

```go
import "github.com/charmbracelet/bubbles/viewport"

func NewLogViewport(width, height int, theme Theme) viewport.Model {
    vp := viewport.New(width, height)
    vp.Style = theme.Surface
    vp.MouseWheelEnabled = true
    return vp
}
```

**Viewport Rules:**
- Keep viewport dimensions synchronized with parent pane size on `tea.WindowSizeMsg`.
- Support standard pager keys (`j/k`, `d/u` for half-page, `g/G` for top/bottom).
- Use `vp.SetContent()` when selection changes in a sibling table or list.

### 2.4 Text Input (`bubbles/textinput`) & Text Area (`bubbles/textarea`)

Used for search filters (`/`), command palette search (`Ctrl+K`), and form inputs.

```go
import "github.com/charmbracelet/bubbles/textinput"

func NewFilterInput(theme Theme) textinput.Model {
    ti := textinput.New()
    ti.Placeholder = "Filter packages... (/)"
    ti.Prompt = "❯ "
    ti.PromptStyle = theme.Accent
    ti.TextStyle = theme.TypeBody
    ti.PlaceholderStyle = theme.TypeHint
    ti.CharLimit = 64
    return ti
}
```

**Input Rules:**
- Focused input fields must feature an accent left bar indicator `▌` (`color.focus`).
- Never wrap inputs in full box borders — keep inputs clean and inline.
- In forms, display validation error text (`✗ …`) directly beneath the field in `color.error`.

### 2.5 Spinner (`bubbles/spinner`)

Used for indeterminate loading operations (querying repos, resolving dependencies).

```go
import "github.com/charmbracelet/bubbles/spinner"

func NewBrailleSpinner(theme Theme) spinner.Model {
    s := spinner.New()
    s.Spinner = spinner.Spinner{
        Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
        FPS:    time.Second / 12, // ~80ms frame rate
    }
    s.Style = theme.Accent
    return s
}
```

> [!CAUTION]
> **Anti-Pattern A7 (Bare Spinners)**: NEVER render a bare spinner by itself. A spinner must ALWAYS be paired with a human-readable label naming what is happening (e.g., `⠸ Resolving dependencies (38/41)…`).
> Respect reduced-motion preferences: display a static glyph (`⠿`) when reduced motion is requested.

### 2.6 Progress (`bubbles/progress`)

Used for determinate operations with known steps or byte counts (downloading RPMs, installing Flatpaks).

```go
import "github.com/charmbracelet/bubbles/progress"

func NewProgressBar(theme Theme, width int) progress.Model {
    p := progress.New(
        progress.WithDefaultGradient(),
        progress.WithWidth(width),
    )
    p.FullColor = theme.ColorAccentHex
    p.EmptyColor = theme.ColorBorderHex
    return p
}
```

**Progress Rules:**
- Follow Design System variant A format: `▕██████░░░░░░▏ 58%` accompanied by counts and ETA.
- When the operation completes, turn the bar `color.success` for one frame beat before transitioning to a completion toast.

### 2.7 Key Bindings & Help (`bubbles/key` & `bubbles/help`)

```go
import (
    "github.com/charmbracelet/bubbles/help"
    "github.com/charmbracelet/bubbles/key"
)

type KeyMap struct {
    Up       key.Binding
    Down     key.Binding
    Install  key.Binding
    Inspect  key.Binding
    Filter   key.Binding
    Commands key.Binding
}

func DefaultKeyMap() KeyMap {
    return KeyMap{
        Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "move")),
        Down:     key.NewBinding(key.WithKeys("j", "down")),
        Install:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "install")),
        Inspect:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "inspect")),
        Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
        Commands: key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("Ctrl+K", "commands")),
    }
}
```

**Help Rules:**
- Dock displays contextual hints for the **currently focused control only**.
- Format: `key: action` separated by `·` (`space.xs`).
- On small terminals (80×24), dock shows at most 4 priority hints; full help lives in `?`.

---

## 3. Customizing Bubbles Safely

1. **Avoid Forking**: Configure Bubbles models via options, exported fields, styles, and custom delegates.
2. **Inject Design System Tokens**: Never hardcode colors when styling Bubbles components. Always pass the active `Theme` tokens (`theme.ColorSelection`, `theme.TypeMeta`, etc.).
3. **Handle Window Size Propagation**: When the root model receives `tea.WindowSizeMsg`, propagate the updated width and height to each child Bubbles model:
   ```go
   case tea.WindowSizeMsg:
       m.table.SetWidth(paneWidth)
       m.table.SetHeight(paneHeight)
       m.viewport.Width = inspectorWidth
       m.viewport.Height = inspectorHeight
   ```

---

## 4. Anti-Patterns to Prevent

| Anti-Pattern | Consequence | Correct Pattern |
|---|---|---|
| **Custom Table from Scratch** | Wasted effort, broken column scrolling, manual cell padding bugs. | Use `bubbles/table` styled with `docs/DESIGN-SYSTEM.md` tokens. |
| **Bare Spinner** | Violates Anti-Pattern A7; leaves user confused about what is happening. | Pair spinner with an active state label ("Downloading packages…"). |
| **Full Box Per List Item** | Violates Anti-Pattern A4 ("Card boxes"); creates visual clutter and border soup. | Group rows using whitespace and indentation; reserve borders for pane containers. |
| **Unresponsive Component Sizes** | Component clips or overflows when terminal is resized. | Propagate `tea.WindowSizeMsg` to all child Bubbles models. |
| **Color-Only Selection** | Breaks on monochrome or 16-color terminals; fails accessibility standards. | Always combine selection styling with reverse video and `❯` glyph. |

---

## 5. Review Checklist for Bubbles Usage

- [ ] Evaluated existing Bubbles component before writing custom interactive logic.
- [ ] Component styles consume tokens from `docs/DESIGN-SYSTEM.md` (no raw hex codes).
- [ ] Table rows render selection with reverse video and `❯` prefix.
- [ ] Table columns handle responsive dropping at M and S breakpoints.
- [ ] Spinners are paired with descriptive activity labels.
- [ ] `tea.WindowSizeMsg` dynamically updates component width and height.
- [ ] Keybindings are declared with `bubbles/key` and surfaced in the dock or help view.
