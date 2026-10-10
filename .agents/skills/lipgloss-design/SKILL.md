---
name: lipgloss-design
description: Guides styling, theme token consumption, spatial geometry, panel composition, typography, and visual hierarchy using Lip Gloss in SetupForge.
---

# Lip Gloss Design & Styling Skill

Use this skill when implementing, refactoring, or reviewing visual styling, layout geometry, panel composition, typography, focus states, and themes using Lip Gloss (`github.com/charmbracelet/lipgloss`) in SetupForge.

This skill is the direct implementation counterpart to [`docs/DESIGN-SYSTEM.md`](../../../docs/DESIGN-SYSTEM.md) and [`docs/UI-UX-DESIGN.md`](../../../docs/UI-UX-DESIGN.md).

> [!IMPORTANT]
> **Core Aesthetic Philosophy**:
> *"Structure gives SetupForge usability; decoration gives SetupForge personality."*
> SetupForge is a serious, Fedora-first workbench. It is neither a chaotic collection of arbitrary boxes nor a sterile, featureless terminal. Decoration (embedded title brackets, subtle rail indicator bars, warm status badges) must remain intentional, consistent, and strictly supportive of visual hierarchy.

---

## 1. Source of Truth: Centralized Token System

Screens and components **must never define ad-hoc hex values, arbitrary borders, or custom color palettes**. All styles derive from semantic tokens defined in `docs/DESIGN-SYSTEM.md`.

### 1.1 The Semantic Color Architecture

```text
Theme Definition (Forge / Rosé Pine / Gruvbox)
      ↓ (maps to)
Semantic Token Roles (bg, surface, accent, focus, error, text)
      ↓ (consumed by)
Lip Gloss Styles & Components
      ↓ (adapts to)
Terminal Capability (TrueColor → 256 → 16 → Mono)
```

### 1.2 SetupForge Default Theme: "Forge"

Forge provides warm graphite surfaces, cream foreground, and a warm copper accent:

```go
type Theme struct {
    // Surfaces & Layers
    Bg           lipgloss.TerminalColor // #1b1a18 - Workspace backdrop
    Surface      lipgloss.TerminalColor // #232120 - Pane interior
    SurfaceMuted lipgloss.TerminalColor // #2a2826 - Inputs, insets, table header strip
    Overlay      lipgloss.TerminalColor // #2e2c29 - Dialogs, palette, sheets

    // Typography & Content
    Fg           lipgloss.TerminalColor // #e6e1d8 - Primary text
    FgMuted      lipgloss.TerminalColor // #a8a29a - Secondary text, metadata
    FgFaint      lipgloss.TerminalColor // #6b6660 - Hints, timestamps, disabled

    // Accents & Brand
    Accent       lipgloss.TerminalColor // #d08743 - Copper brand, active tabs
    AccentSec    lipgloss.TerminalColor // #8fbbb7 - Cyan/sage paths, identifiers, data
    Focus        lipgloss.TerminalColor // #d08743 - Focused pane border & indicators

    // Status Semantics
    Success      lipgloss.TerminalColor // #a0b56a - Healthy, installed, completed
    Warning      lipgloss.TerminalColor // #e0a84f - Updates available, pending
    Error        lipgloss.TerminalColor // #d05c5c - Destructive, failed
    Info         lipgloss.TerminalColor // #7f9cbc - Informational glyphs, notes

    // Structural Borders
    Border       lipgloss.TerminalColor // #3a3733 - Default unfocused pane borders
    BorderMuted  lipgloss.TerminalColor // #2c2a27 - Dividers, table rules, inactive rails

    // Interactive States
    Selection    lipgloss.Style         // Reverse video: #e6e1d8 on #4a4238
    Disabled     lipgloss.TerminalColor // #6b6660 (paired with dim)
}
```

### 1.3 Alternate Themes as Token Mappings
Rosé Pine and Gruvbox Dark are **pure token mappings**, never separate layout engines or custom style forks:
- Changing the theme swaps color bindings; layout math, border characters, and spacing scales remain 100% identical.
- In Rosé Pine, `Success` maps to `#31748f` ("Pine"); paired with `✓`, semantic clarity is preserved.
- In Gruvbox Dark, `Accent` maps to `#fe8019`, `Fg` to `#ebdbb2`.

---

## 2. Spatial System & Geometry: The Modular Bench

SetupForge must not look like an arbitrary collage of mismatched boxes.

### 2.1 The M-Unit Grid (Macro Geometry)

All stage geometry derives from the **Module (M)**:
- **1M Width** = ¼ of the available stage width (`(terminalWidth - railWidth) / 4`).
- **1M Height** = ¼ of the available stage height (`(terminalHeight - header - dock) / 4`).

Stage layouts are composed strictly from these standardized unit multiples:

| Pane Unit | Grid Dimensions | Composition | Used For |
|---|---|---|---|
| **Whole** | 4M × 4M | Full stage | Single-surface pages (Journal, Development, Settings forms) |
| **Half** | 2M × 4M | Vertical half | Master–detail split (Software catalog + Inspector) |
| **Wide** | 4M × 2M | Horizontal half | Overview stat band, confirmation previews |
| **Quad** | 2M × 2M | Quarter tile | Overview metric tiles at XL (160×45+) |

```text
  Stage = 4M × 4M:
  ┌───────────────────┬───────────────────┐  2M × 4M Half Panes
  │ Master Browser    │ Detail Inspector  │  Shared border, zero gutter
  │ (2M × 4M)         │ (2M × 4M)         │
  └───────────────────┴───────────────────┘
```

> [!CAUTION]
> **Anti-Pattern (Unbalanced Geometry)**:
> Never build screens with a massive empty panel on top and a cramped, unreadable panel beneath. Maintain visual rhythm and proportion using the M-unit system.

### 2.2 Shared Borders & Zero Gutter

Adjacent stage panes share single-line border edges. Never place empty padding columns or double frames between adjacent panes.

---

## 3. Border Signature & Pane Container

SetupForge features a distinct single-line border signature with embedded header slots:

```text
┌─ Packages ─ 142 available ────────── 12 of 142 ─┐
│                                                 │
│  Row content                                    │
│                                                 │
└─────────────────────────────────────────────────┘
```

### 3.1 Header Slot Layout
- **Title**: Left-aligned immediately after `┌─ ` in bold text.
- **Metadata** (Optional): Center-left, separated by ` ─ ` in `type.meta`.
- **Position / Count** (Optional): Right-aligned before ` ─┐` in `type.data`.

### 3.2 Border Tokens & Rules
- **Border Family**: Strictly single-line (`┌─┐│└┘├┤┬┴┼`).
- **Never use**: Double borders (`╔═╗`), rounded corners (`╭─╮`), heavy borders, or dashed lines.
- **Unfocused Pane**: Border rendered in `color.border`.
- **Focused Pane**: Border rendered in `color.focus` (accent) with bold title. Exactly **one** pane per screen carries the focused accent border.

```go
func RenderPane(title, meta, pos, content string, width, height int, focused bool, theme Theme) string {
    borderColor := theme.Border
    titleStyle := theme.TypeTitle
    if focused {
        borderColor = theme.Focus
        titleStyle = theme.TypeTitle.Bold(true).Foreground(theme.Accent)
    }

    // Build header line: ┌─ Title ─ meta ────── pos ─┐
    leftHeader := fmt.Sprintf("┌─ %s", titleStyle.Render(title))
    if meta != "" {
        leftHeader += fmt.Sprintf(" ─ %s", theme.TypeMeta.Render(meta))
    }

    rightHeader := ""
    if pos != "" {
        rightHeader = fmt.Sprintf(" %s ─┐", theme.TypeData.Render(pos))
    } else {
        rightHeader = "─┐"
    }

    fillCount := width - lipgloss.Width(leftHeader) - lipgloss.Width(rightHeader)
    if fillCount < 1 {
        fillCount = 1
    }
    topLine := leftHeader + strings.Repeat("─", fillCount) + rightHeader
    topLine = lipgloss.NewStyle().Foreground(borderColor).Render(topLine)

    // Inner content area: padding 1 row top/bottom, 2 cols left/right
    innerStyle := lipgloss.NewStyle().
        Width(width - 2).
        Height(height - 2).
        Padding(1, 2).
        Background(theme.Surface)

    // Render sides and bottom border...
    return lipgloss.JoinVertical(lipgloss.Left, topLine, innerContent, bottomLine)
}
```

---

## 4. Typography Hierarchy & Contrast

Terminals offer limited typographic levers: weight (bold/normal/faint), color, and case.

| Token | Weight | Color Role | Usage |
|---|---|---|---|
| `type.brand` | Bold | `color.accent` | Wordmark: "SetupForge" in header |
| `type.title` | Bold | `color.fg` | Pane and dialog titles embedded in borders |
| `type.heading` | Bold | `color.fg` | Section headings inside panels |
| `type.body` | Normal | `color.fg` | Table text, general descriptions, body copy |
| `type.data` | Normal | `color.accent-secondary` | File paths, package versions, IP addresses |
| `type.meta` | Normal | `color.fg-muted` | Item counts, timestamps, column headers |
| `type.hint` | Normal | `color.fg-faint` | Keyboard hints in dock, empty-state guidance |
| `type.key` | Bold | `color.accent` | Key names (`Ctrl+K`, `Enter`, `Tab`) |
| `type.status` | Bold | Semantic status | `✓ Success`, `⚠ Warning`, `✗ Error` |
| `type.disabled` | Dim | `color.disabled` | Non-interactive controls |
| `type.inverted` | Bold | `color.bg` on `color.accent` | Primary dialog action buttons, active search hit |

### Case Rules
- **Sentence case everywhere**: "Install selected packages", "System services", "Update manager".
- **NO ALL CAPS**: Shouting at terminal scale damages calm readability.
- **NO lowercase branding**: Write "SetupForge", never "setupforge".

---

## 5. Spacing Scale

1 cell = 1 character width × 1 terminal row.

| Spacing Token | Cells | Usage |
|---|---|---|
| `space.xs` | 1 | Divider margins, glyph-to-text separation (`✓  Success`) |
| `space.sm` | 2 | Default horizontal inner padding for panes and tables |
| `space.md` | 4 | Spacing between form fields, columns, or stat blocks |
| `space.lg` | 6 | Section breaks within a full-stage surface |
| `space.xl` | 12 | Reserved for large wide terminal layouts (≥150 cols) |

**Vertical Padding Rule**: Standard interior vertical padding for any pane is **1 row** (`space.xs`) above and below content.

---

## 6. Interactive States & Accessibility

State must be instantly recognizable without cognitive effort.

```text
Visual Weight Order:
1. Active Selection   → Reverse video row (Strongest signal)
2. Focused Pane       → Accent border (color.focus) + bold title
3. Active Operation   → Spinning accent glyph + live dock status
4. Primary Data       → Clean foreground body text
5. Section Headings   → Bold foreground text
6. Metadata           → Muted secondary text
7. Key Hints          → Accent key name + muted label
8. Structural Rules   → Muted border (Dimmest layer)
```

### Accessibility Invariants
1. **Never rely on color alone**: Status colors must ALWAYS be accompanied by a unique glyph (`✓`, `⚠`, `✗`, `ℹ`) or text twin.
2. **Reverse Video for Selection**: Selected rows use reverse video (`color.selection`). This ensures perfect legibility on 16-color and monochrome terminals.
3. **High Contrast**: Ensure contrast ratio ≥ 4.5:1 for body text, ≥ 3:1 for metadata and borders.

---

## 7. Layout Math & Safe Rendering

Terminal strings contain ANSI escape sequences. Standard Go `len(str)` counts raw bytes, not visual terminal cells!

### Safe Measuring Rule
Always measure visual width and height using Lip Gloss:
```go
// Correct:
visualWidth := lipgloss.Width(renderedString)
visualHeight := lipgloss.Height(renderedString)

// WRONG:
// visualWidth := len(renderedString) // Breaks completely with ANSI color codes!
```

### Joining Views
Use Lip Gloss compositional helpers rather than manual string concatenation:
- `lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)`
- `lipgloss.JoinVertical(lipgloss.Left, headerView, stageView, dockView)`
- `lipgloss.Place(termWidth, termHeight, lipgloss.Center, lipgloss.Center, dialogBox)` (for centered modals with backdrop dim)

---

## 8. Anti-Patterns to Prevent

| Anti-Pattern | Violation | Prevention |
|---|---|---|
| **Raw Hex in Screens** | `lipgloss.Color("#ff0000")` | Always reference `theme.Error` or semantic token. |
| **Border Soup** | Wrapping every card, row, and label in a box. | Use whitespace and column alignment; reserve borders for panes. |
| **Rounded or Double Borders** | `lipgloss.RoundedBorder()` | Only single-line borders (`┌─┐│└┘`) are permitted. |
| **Color Confetti** | Using 6 different bright colors on one screen. | Max 1 accent color + status colors where state lives. |
| **All-Caps Headers** | "PACKAGES", "ACTIONS" | Use sentence case: "Packages", "Actions". |
| **Web-isms** | Gradients, shadows, card drop-shadows. | Emphasize native terminal materials: character, space, weight. |
| **Sterile Minimalism** | Stripping all personality and visual identity. | Use the signature border header, subtle accent bars, and warm badges. |

---

## 9. Visual Implementation Checklist

- [ ] All colors consumed via semantic tokens from `docs/DESIGN-SYSTEM.md`.
- [ ] Panes conform to the M-unit grid (Whole, Half, Wide, Quad).
- [ ] Adjacent panes share borders with zero gutter.
- [ ] Border header adheres to signature format: `┌─ Title ─ metadata ─ position ─┐`.
- [ ] Exactly one pane has the focused accent border.
- [ ] Selection is highlighted via reverse video and `❯` glyph.
- [ ] Spacing scale followed (1 row / 2 cols inner padding).
- [ ] Text uses sentence case throughout.
- [ ] String dimensions measured with `lipgloss.Width()` and `lipgloss.Height()`.
- [ ] Design verified across dark and light terminal profiles.
