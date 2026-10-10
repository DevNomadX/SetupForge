# SetupForge — Design System

> Version 1.0 · Companion to [`UI-UX-DESIGN.md`](./UI-UX-DESIGN.md).
> This document is the token and component contract. Screens reference tokens;
> they never contain raw hex values, spacing literals, or ad-hoc styles.

---

## 1. Design Tokens — Architecture

```
Theme file (data) ──maps──► Semantic tokens (roles) ──render──► Terminal (16 / 256 / truecolor / mono)
```

- **Semantic tokens** are the only thing screens consume: `accent`, `surface`,
  `border-muted`, `text-body`, `state-error`, …
- **Theme files** (TOML) bind tokens to hex values per theme. Adding a theme
  never touches screen code.
- **Capability detection** at startup maps each token to truecolor → 256 → 16 →
  mono. `NO_COLOR` and `TERM=dumb` force the mono profile.
- Token names are `category.role[.state]`. All tokens are listed here; if a
  screen needs a style not in this file, the token is added here first.

---

## 2. Color Tokens — default theme "Forge"

Forge is SetupForge's own identity: warm graphite surfaces, cream foreground,
copper accent. Warmth from Gruvbox's discipline (never pure white), calm
structure from Rosé Pine (muted borders, layered surfaces) — an original
palette, not a copy of either.

| Token | Hex | 256 | 16 | Usage |
|-------|-----|-----|----|-------|
| `color.bg` | `#1b1a18` | 234 | black | Workspace background |
| `color.surface` | `#232120` | 235 | black | Pane interior |
| `color.surface-muted` | `#2a2826` | 236 | black | Insets, input fields, table header strip |
| `color.overlay` | `#2e2c29` | 237 | bright black | Dialogs, palette, sheets |
| `color.fg` | `#e6e1d8` | 188 | white | Primary text |
| `color.fg-muted` | `#a8a29a` | 145 | bright black | Secondary text, metadata |
| `color.fg-faint` | `#6b6660` | 241 | bright black (dim) | Hints, timestamps, disabled text |
| `color.accent` | `#d08743` | 173 | yellow | Focused pane border, selection accents, brand |
| `color.accent-secondary` | `#8fbbb7` | 109 | cyan | Links, paths, data values |
| `color.success` | `#a0b56a` | 143 | green | `✓`, healthy state |
| `color.warning` | `#e0a84f` | 179 | yellow | `⚠`, pending, update badge |
| `color.error` | `#d05c5c` | 167 | red | `✗`, destructive actions |
| `color.info` | `#7f9cbc` | 110 | blue | Neutral highlights, info glyph |
| `color.border` | `#3a3733` | 237 | bright black | Pane/dialog borders (unfocused) |
| `color.border-muted` | `#2c2a27` | 236 | black | Inactive rails, inner rules, table dividers |
| `color.focus` | `#d08743` | 173 | yellow | Focus ring (same hue as accent, border context) |
| `color.selection` | `#e6e1d8` on `#4a4238` | fg 188 / bg 238 | white on black (reverse) | Selected row: reverse video |
| `color.marked` | `#d08743` | 173 | yellow | Batch-mark glyph `●` |
| `color.disabled` | `#6b6660` | 241 | bright black | Disabled controls (with dim) |
| `color.backdrop` | `#1b1a18` + dim | 234 dim | black dim | Dialog dim layer |

### Light-terminal adaptation

SetupForge targets dark terminals but must not break in light ones. In a light
profile the *roles* stay identical and the values invert luminance: bg `#f4f1ec`,
surface `#eae6de`, fg `#2a2724`, accent `#9a5b1e`, borders `#c8c2b8`, selection
reverse video unchanged. All contrast minimums (§10) hold in both.

---

## 3. Theme Strategy — Rose Pine & Gruvbox as token mappings

Decision (recorded in UI-UX-DESIGN.md §18): neither ships as the default;
both ship as bundled alternate themes because they are popular and because the
token architecture makes them nearly free. They are **mappings**, not skins.

| Token | Forge (default) | Rosé Pine | Gruvbox Dark |
|-------|-----------------|-----------|--------------|
| `color.bg` | `#1b1a18` | `#191724` | `#282828` |
| `color.surface` | `#232120` | `#1f1d2e` | `#3c3836` |
| `color.surface-muted` | `#2a2826` | `#26233a` | `#32302f` |
| `color.overlay` | `#2e2c29` | `#26233a` | `#3c3836` |
| `color.fg` | `#e6e1d8` | `#e0def4` | `#ebdbb2` |
| `color.fg-muted` | `#a8a29a` | `#908caa` | `#a89984` |
| `color.fg-faint` | `#6b6660` | `#6e6a86` | `#928374` |
| `color.accent` | `#d08743` | `#ebbcba` | `#fe8019` |
| `color.accent-secondary` | `#8fbbb7` | `#9ccfd8` | `#8ec07c` |
| `color.success` | `#a0b56a` | `#31748f` → rendered as teal-status | `#b8bb26` |
| `color.warning` | `#e0a84f` | `#f6c177` | `#fabd2f` |
| `color.error` | `#d05c5c` | `#eb6f92` | `#fb4934` |
| `color.info` | `#7f9cbc` | `#c4a7e7` | `#83a598` |
| `color.border` | `#3a3733` | `#403d52` | `#504945` |
| `color.border-muted` | `#2c2a27` | `#26233a` | `#3c3836` |

Geometry never changes across themes — themes are palette swaps, so a theme
can never break layout or legibility. (Note: in Rosé Pine the `success` token
maps to a teal that reads as "pine" — paired with `✓` it stays semantically
clear, which is exactly why glyphs always accompany color.)

---

## 4. Typography Tokens

Terminal-safe only: weight, color, and case. No font dependencies, no italics
(poor support), no figlet art in the workspace.

| Token | Style | Used for |
|-------|-------|----------|
| `type.brand` | bold, `color.accent` | "SetupForge" wordmark |
| `type.title` | bold, `color.fg`, embedded in border | Pane/dialog titles |
| `type.heading` | bold, `color.fg` | Section headings inside panes |
| `type.body` | normal, `color.fg` | Content, table values |
| `type.data` | normal, `color.accent-secondary` | Paths, versions, identifiers |
| `type.meta` | normal, `color.fg-muted` | Counts, sizes, descriptions |
| `type.hint` | normal, `color.fg-faint` | Help text, empty-state guidance |
| `type.key` | bold, `color.accent` | Key names in dock and hints |
| `type.status` | bold, status color + glyph | `✓ success`, `⚠ warning`, `✗ error` |
| `type.disabled` | `color.disabled` + dim | Unavailable controls |
| `type.inverted` | `color.bg` on `color.accent` | Primary dialog button, palette match |

Case rules: sentence case everywhere. No ALL CAPS (reads as shouting at
terminal scale), no lowercase branding.

---

## 5. Spacing, Borders, Padding

### Spacing scale (1 cell = 1 char × 1 row)

| Token | Cells | Usage |
|-------|-------|-------|
| `space.xs` | 1 | Inner rule margins, glyph padding |
| `space.sm` | 2 | Default inner padding (left/right of pane content) |
| `space.md` | 4 | Between form fields, groups |
| `space.lg` | 6 | Between major sections within a pane |
| `space.xl` | 12 | Reserved — rarely justified in a terminal |

Vertical padding inside panes: 1 row (`space.xs`) above and below content.

### Border tokens

| Token | Characters | Color | Usage |
|-------|-----------|-------|-------|
| `border.frame` | `┌─┐│└┘├┤┬┴┼` | `color.border` | All panes, dialogs, palette, sheets |
| `border.frame-focused` | same | `color.focus` + bold | Focused pane only |
| `border.rule` | `─` | `color.border-muted` | Table header dividers, section breaks |
| `border.fallback` | `+-|'` | `color.border` | ASCII fallback terminals — identical layout math |

One border family, ever. No double, heavy, rounded, or dashed borders.
Title format: `┌─ Title ─ metadata ─ position ─┐` (slots optional).

### Emphasis levels

1. reverse video (selection) → 2. bold + accent (titles, keys) →
3. bold (headings) → 4. normal → 5. muted → 6. faint/dim.

---

## 6. Elevation & Overlay Tokens

Terminals have no real z-depth; layering is communicated by surface + dim:

| Layer | Token | Treatment |
|-------|-------|-----------|
| Workspace | `color.bg` | — |
| Pane | `color.surface` | — |
| Inset / input | `color.surface-muted` | — |
| Sheet (edge panel) | `color.overlay` | attached to edge, no backdrop |
| Dialog / palette | `color.overlay` + `color.backdrop` dim | centered, backdrop dims the stage |
| Toast | none (transparent bg) | dock-adjacent line, no box |

---

## 7. Component Rules

### 7.1 Pane (the universal container)

- Geometry: M-unit multiples only (UI-UX-DESIGN.md §8). Padding 1 row / 2 cols.
- Border header slots: title · count · position. Show only slots that earn space.
- States: `default` (muted border) → `focused` (accent border, bold title).
  Exactly one focused pane per screen.
- Nested panes: one level maximum; nested uses `color.border-muted` and no title.

### 7.2 Table

```
  Name                    Version        State         Size
  ────────────────────────────────────────────────────────────
❯ git                     2.49.0-1       installed ●   108 kB
  gruvbox-theme-gtk       1.0.0-3        available     2.4 MB
```

- Header: `type.meta` + rule below. Sort column: `▲/▼` after header.
- Rows: `type.body`; active row reverse video with `❯`; marked rows `●`.
- Alignment: text left, numbers right, status center-right.
- Truncation: `…`, full value in inspector. Never wrap mid-word in tables.
- Pagination: `4 of 142` in the border. No footer row.
- Column priority list per surface defines drop order at M/S breakpoints.

### 7.3 List

- Selection `❯` + reverse video on the item; indentation for hierarchy (2 cols/level).
- One-dimensional data only (see §15 of UI-UX-DESIGN.md).

### 7.4 Tree

- `▸` collapsed / `▾` expanded; guide lines at `color.border-muted`; `h/l` or `←/→`.

### 7.5 Cards (application browser)

- Single-row card: name (`type.body`) · source (`type.meta`) · size (`type.data`)
  · action aligned right. No box per card — whitespace and alignment group them;
  a border per card is clutter (anti-pattern A4).

### 7.6 Key/value (detail surfaces)

- Key column fixed width, `type.meta`, padded to the longest key; values
  `type.body` / `type.data`. Max 2 column pairs side by side at ≥120 cols.

### 7.7 Forms

- Field label `type.heading` above or left; inputs on `color.surface-muted`
  with 1-row height, `space.sm` between fields.
- Focused field: `color.focus` left bar `▌` + cursor visible. Never a full
  border change per field — motion without meaning.
- Validation: error text `type.status` (`✗ …`) directly under the field;
  the field keeps its left bar in `color.error`. Errors never appear as popups.
- Toggles/checkboxes: `●/○` or `[x]/[ ]` + label; toggle on `Space`/`Enter`.
- Selects: current value + `▾`; options render in an overlay sheet; `Esc`
  closes without change.
- Actions bottom-right: `[Cancel] [Primary]`; primary uses `type.inverted`;
  destructive uses `type.status` error + `!` prefix and is never preselected.
- Destructive settings (e.g., "replace rc files") require typed confirmation
  or an explicit `!` confirm dialog — no single-press destruction.

### 7.8 Dialog

- Sizes: L 88×15 · M 80×13 · S 76×11, centered, ≥2 col margin, backdrop dim.
- Layout: title in border (`type.status` color when destructive) → body
  (what/why/scope/root/cancellable) → action row → optional expandable Details.
- Buttons: `Tab` cycles, `Enter` activates, `Esc` = Cancel always.
- Dialogs are for irreversible, privileged, or long operations only. If the
  information is reference, use the inspector; if transient, use a toast.

### 7.9 Notifications

- **Toast**: single line, right-aligned above dock, `type.status` + glyph,
  ≤4 s, never captures focus, max 1 visible (queue collapses to the latest).
- **Badge**: update count in header (`type.warning` when >0).
- **Inline status**: filter state, marking counts — left of dock, persistent
  while the state exists.
- No notification drawer; history lives in Journal.

### 7.10 Progress

- Bar variant A (known progress): `▕██████░░░░░░▏ 58%` — `█` accent-filled,
  `░` `color.border`; percentage `type.data`; package/step count and ETA right.
- Bar variant B (indeterminate): labeled braille spinner + the *name of what*
  is being waited on.
- Completion: bar turns `color.success` for one frame beat, then a toast.

### 7.11 Spinners

- Braille `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` @ 80 ms (fallback `\|/-`).
- Always paired with a label ("Resolving dependencies…"). A bare spinner is an
  anti-pattern (A7). Respect reduced-motion: label only, no animation.

---

## 8. Icons & Glyphs

Single-width glyphs only; every glyph has an ASCII fallback and a text twin
(§10). No emoji (width instability), no Nerd Font requirement.

| Purpose | Glyph | ASCII fallback | Color token |
|---------|-------|----------------|-------------|
| Selection | `❯` | `>` | reverse video |
| Marked | `●` | `*` | `color.marked` |
| Success | `✓` | `+` | `color.success` |
| Error | `✗` | `x` | `color.error` |
| Warning | `⚠` | `!` | `color.warning` |
| Info | `ℹ` | `i` | `color.info` |
| Running | `▶` | `>` | `color.accent` |
| Pending/queued | `○` | `o` | `color.fg-muted` |
| Tree expanded/collapsed | `▾`/`▸` | `v`/`>` | `color.fg-muted` |
| Expand affordance | `▾` | `v` | `color.fg-muted` |
| Rail active | `▌` | `|` | `color.accent` |
| Spinner | `⠿` family | `\|/-` | `color.accent` |
| Updates badge | `●` | `*` | `color.warning` |
| Bullet | `·` | `.` | `color.fg-faint` |

---

## 9. Motion

- No animated transitions anywhere. Screens, panes, and dialogs switch
  instantly; state changes read through the focus/selection system.
- Only permitted motion: spinners and progress fills (operation feedback).
- Reduced motion: static glyphs replace all animation; information unchanged.

---

## 10. State Definitions (all components)

| State | Visual rule |
|-------|-------------|
| default | `type.body` / muted border |
| hover (mouse) | mirrors focus visuals; adds nothing unique |
| focused | accent border (panes) · reverse video (items) · `▌` bar (fields) |
| selected | reverse video row; the action target within a focused pane |
| marked | `●` + accent glyph; persists across focus changes |
| active (running) | `▶` + spinner/progress; dock shows live status |
| disabled | `color.disabled` + dim; non-interactive, still legible |
| success | `✓` + `color.success` |
| warning | `⚠` + `color.warning` |
| error | `✗` + `color.error`; never used for decoration |
| info | `ℹ` + `color.info` |

Accessibility invariants: color is always paired with a glyph or weight change
(no color-only meaning); contrast ≥ 4.5:1 for body text, ≥ 3:1 for muted and
status colors; mono profile (bold/dim/reverse) expresses every state above.

---

## 11. Anti-Patterns (normative — review checklist for every screen)

| # | Anti-pattern | Rule |
|---|--------------|------|
| A1 | Hidden actions | If a binding exists, it appears in dock, help, or palette |
| A2 | Modal as default | Confirm only destructive/privileged/long ops; everything else is pane/sheet/toast |
| A3 | Border soup | One border family; borders group, never decorate; nested depth ≤1 |
| A4 | Card boxes | No per-item boxes in lists; alignment and whitespace group instead |
| A5 | Raw error strings | `exit status 1` never shown bare — always the 4-question error (what/why/scope/next) |
| A6 | Dead screens | >200 ms wait ⇒ labeled loading state in place of pending content |
| A7 | Bare spinners | Every spinner has a label naming what is happening |
| A8 | Color confetti | ≤1 accent + status colors only where state lives; never >4 hues on screen |
| A9 | Shortcut overload | Core 12 keys only; anything else lives in palette + `?` |
| A10 | Web-ism | No gradients, shadows, rounded cards, hover-menus, or scrollbars-as-decoration |
| A11 | Layout drift | No screen invents geometry outside the M-unit grid and frame contract |
| A12 | Focus amnesia | Returning to a page restores selection, filter, scroll, inspector state |

---

## 12. Future Theme Strategy

1. **Now**: `forge` (default), `rose-pine`, `gruvbox-dark` as token mappings.
2. **Next**: community themes ship as TOML files in `themes/` — one file per
   theme binding the §2 token set; no code review needed to add one.
3. **Later**: a light `forge-day` variant completes the built-in set.
4. **Contract that never changes**: token names, the M-unit geometry, border
   family, and state semantics. A theme may change *values only* — never
   structure, spacing, or meaning. This guarantees SetupForge looks like
   SetupForge in every theme.
