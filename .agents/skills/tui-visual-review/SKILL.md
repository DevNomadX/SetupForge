---
name: tui-visual-review
description: Guides visual quality assurance, layout geometry, typography contrast, focus visibility, responsive inspection, and design system compliance before shipping TUI changes.
---

# TUI Visual Review & QA Skill

Use this skill when performing visual quality assurance, reviewing pull requests, inspecting rendered screens, or conducting pre-completion reviews of SetupForge terminal user interfaces.

> [!CAUTION]
> **Core Quality Invariant**:
> *An agent must never consider a TUI task finished merely because `go test ./...` passes.*
> Tests can pass on code that renders overlapping text, garbled borders, missing spinners, unreadable colors, or broken responsive layouts. Every visual change must be audited against the visual physics and design system of SetupForge.

This skill is the authoritative audit guide for comparing terminal output against [`docs/DESIGN-SYSTEM.md`](../../../docs/DESIGN-SYSTEM.md) and [`docs/UI-UX-DESIGN.md`](../../../docs/UI-UX-DESIGN.md).

---

## 1. Visual Review Rubric: The 10 Inspection Gates

Before declaring any TUI work complete, evaluate your screen against these ten visual criteria:

### Gate 1: Spatial Geometry & Panel Balance
- [ ] **M-Unit Grid Adherence**: Does the stage geometry align to the 4M×4M module system (`docs/DESIGN-SYSTEM.md` §8)?
- [ ] **Balanced Proportions**: Are panel heights and widths balanced?
  - **Fail**: A giant, cavernous empty panel on top paired with a tiny, cramped, unreadable list below.
  - **Pass**: Master browser and Inspector occupy proportional halves (2M×4M each) with shared borders and zero gutter.

### Gate 2: Border Signature & Containers
- [ ] **Single-Line Family**: Are all borders rendered exclusively using standard single-line box characters (`┌─┐│└┘├┤┬┴┼`)?
  - **Fail**: Rounded corners (`╭─╮`), double borders (`╔═╗`), or dashed lines.
- [ ] **Signature Header Slots**: Does the top border follow SetupForge's signature format?
  ```text
  ┌─ Title ─ metadata ─ position ─┐
  ```
- [ ] **Focused Pane Isolation**: Is the focused pane the **ONLY** pane with an accent border (`color.focus`) and bold title?

### Gate 3: Visual Hierarchy & Emphasis Order
- [ ] Does the visual weight strictly follow the ranked hierarchy?
  1. **Selection**: Reverse video on active row (`color.selection` + `❯`).
  2. **Focused Pane Border**: Accent color + bold title.
  3. **Active Operation**: Accent spinner + live dock counter.
  4. **Primary Data**: Cream foreground text (`color.fg`).
  5. **Headings**: Bold foreground text.
  6. **Metadata**: Muted secondary text (`color.fg-muted`).
  7. **Key Hints**: Accent key + muted action.
  8. **Structural Rules**: Dimmest border line (`color.border-muted`).

### Gate 4: Typography, Case, and Spacing
- [ ] **Sentence Case**: Is all text written in sentence case (e.g., "Install selected packages")?
  - **Fail**: ALL CAPS shouting ("PACKAGES", "ACTIONS") or lowercase branding ("setupforge").
- [ ] **Padding Scale**: Does pane content have exactly 1 row vertical padding and 2 columns horizontal padding?
- [ ] **Text Truncation**: Does long text truncate cleanly with `…` without wrapping awkwardly mid-word onto the next row?

### Gate 5: Focus & Selection Visibility
- [ ] **Unambiguous Target**: Can you immediately identify which element will respond to the next keystroke within 50ms of looking at the screen?
- [ ] **Selection vs. Marking**:
  - Selected row is highlighted with reverse video and `❯`.
  - Batch marked items display the persistent `●` glyph in `color.marked`.

### Gate 6: Color Restraint & Accessibility
- [ ] **No Color Confetti**: Are there ≤4 distinct color hues visible simultaneously on the screen?
- [ ] **Glyph Reinforcement**: Does every status color have an explicit glyph twin?
  - `✓` for `Success`
  - `⚠` for `Warning`
  - `✗` for `Error`
  - `ℹ` for `Info`
- [ ] **Degradation Check**: Does the interface remain 100% understandable in monochrome or on 16-color terminals (`NO_COLOR=1`)?

### Gate 7: Loading States & Labeled Spinners
- [ ] **>200ms Rule**: Does any operation taking longer than 200ms present a labeled loading indicator?
- [ ] **Anti-Pattern A7 (Bare Spinners)**:
  - **Fail**: A spinning dot with no explanation.
  - **Pass**: Braille spinner paired with specific text: `⠸ Resolving dependencies (38/41)…`.

### Gate 8: Error Diagnostics & Recovery
- [ ] **4-Question Format**: Do error dialogs clearly explain:
  1. What happened
  2. Why it happened
  3. What was affected
  4. What you can do next
- [ ] **No Raw Exit Codes**: `exit status 1` is relegated to the expandable Details view, never shown bare.
- [ ] **Safe Default Button**: Is the default pre-selected button in a destructive dialog ALWAYS `[Cancel]`?

### Gate 9: Empty States
- [ ] If a table, list, or log surface has zero items, does it clearly explain **why** it is empty and **what** action populates it?

### Gate 10: Responsive Transformation (The 4 Benchmarks)
- [ ] **80×24 (S)**: Rail is collapsed to overlay (`Ctrl+B`); dual panes transform to sequential drill-down; dock shows ≤4 hints.
- [ ] **100×30 (M)**: Rail is 14 cols; tables drop to 4 priority columns; clock is hidden.
- [ ] **120×35 (L)**: Standard desktop layout; Half+Half master-detail default; inspector open.
- [ ] **160×45+ (XL)**: Overview shows 4 Quad tiles; tables display ≥6 columns; rich telemetry visible.

---

## 2. Visual Anti-Patterns Checklist

Verify that your change does not introduce any of the 12 normative anti-patterns (`docs/DESIGN-SYSTEM.md` §11):

| # | Anti-Pattern | Check |
|---|---|---|
| **A1** | **Hidden Actions** | Every binding appears in the Dock, help (`?`), or Command Palette (`Ctrl+K`). |
| **A2** | **Modal as Default** | Modals are reserved for destructive or privileged operations; routine work uses panes or sheets. |
| **A3** | **Border Soup** | Exactly one border family; borders group content, never decorate individual text lines. |
| **A4** | **Card Boxes** | No per-item rectangular boxes in lists; whitespace and column alignment group them. |
| **A5** | **Raw Error Strings** | Raw CLI errors are never shown bare without human context and next steps. |
| **A6** | **Dead Screens** | No blank screen persists >200ms without a labeled loading indicator. |
| **A7** | **Bare Spinners** | Every spinner has an accompanying label naming the active operation. |
| **A8** | **Color Confetti** | Maximum 1 accent color + status colors where state lives. |
| **A9** | **Shortcut Overload** | Core 12 keys cover routine tasks; everything else lives in the palette. |
| **A10** | **Web-isms** | No rounded boxes, simulated drop-shadows, gradients, or hover menus. |
| **A11** | **Layout Drift** | No screen invents ad-hoc geometry outside the M-unit grid. |
| **A12** | **Focus Amnesia** | Navigating away and returning restores selection, scroll, and filters. |

---

## 3. How to Conduct Visual Review

When testing and reviewing TUI views:

1. **Render at All Four Breakpoints**:
   Run tests or manual previews with explicit terminal dimensions:
   - 80 columns × 24 rows
   - 100 columns × 30 rows
   - 120 columns × 35 rows
   - 160 columns × 45 rows
2. **Inspect ASCII Alignment**:
   Look for:
   - Trailing lines wrapping unexpectedly onto the next line.
   - Truncated text breaking box borders.
   - Missing closing border characters (`│`, `┘`).
3. **Audit Color Roles**:
   Verify that theme tokens are used semantically and that no hardcoded hex strings exist in the view code.
4. **Walk the Keyboard Path**:
   Press `Tab`, `j/k`, `Enter`, `Esc`, `Space`—verify that visual focus indicators update smoothly without flickering or lost focus.
