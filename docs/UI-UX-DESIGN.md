# SetupForge — UI/UX Design

> Version 1.0 · Design phase only — no implementation decisions are made here.
> Companion document: [`DESIGN-SYSTEM.md`](./DESIGN-SYSTEM.md) (tokens and component rules).
> Target: Fedora Linux · Go · Bubble Tea + Bubbles + Lip Gloss · min grid 80×24.

---

## 1. Design Philosophy

**SetupForge is a workbench, not a wizard.**

A wizard walks you through steps and forgets you. A menu asks you to pick a number.
A workbench stays where you left it: tools within reach, the current job in front of you,
state visible at a glance, and nothing shouting for attention.

The design language is built on four commitments:

### 1.1 One workspace, everywhere

Every screen is the same workspace with different content in it. The frame
(header, rail, dock) never changes shape. Only the **stage** — the working
surface — changes content. A user navigating Packages → Fonts → Services should
never experience a "different application"; only a different set of tools on the
same bench.

### 1.2 Calm by default, dense on demand

The interface is quiet: muted borders, one accent color, generous alignment.
Information density comes from **structure** (tables, aligned columns, grouped
panels), never from clutter. Details are progressively disclosed — a package
row shows name/version/state; its details appear on demand in the inspector,
not permanently on screen.

### 1.3 The terminal is not a browser

No gradients, no rounded "cards with shadows," no web-dashboard metaphors.
SetupForge uses the native materials of the terminal: characters, weight, color,
box drawing, and space. It should be unmistakably a terminal application —
and unmistakably *this* terminal application.

### 1.4 Operations are first-class citizens

SetupForge modifies the operating system. Every operation therefore has a
designed lifecycle (Ready → Confirm → Prepare → Execute → Result), a visible
home (the Activity system), and honest states (cancellable or not, privileged
or not, partial success included). The UI never leaves the user guessing
whether the machine is busy or hung.

---

## 2. Design Principles

Eight principles, in priority order. When two principles conflict, the earlier one wins.

| # | Principle | Meaning |
|---|-----------|---------|
| P1 | **Spatial constancy** | The frame, rail, dock, and pane geometry are identical on every screen. Only stage content changes. |
| P2 | **Keyboard completeness** | Every action is reachable by keyboard. Mouse is additive. |
| P3 | **One focus, always visible** | Exactly one element has focus. Its location is never ambiguous. |
| P4 | **Color is meaning** | Color encodes state (accent, success, warning, error, data). It is never decoration. |
| P5 | **Progressive disclosure** | Show what the task needs now; reveal depth on demand. |
| P6 | **Honest operations** | Every system operation declares what it does, whether it needs root, and whether it can be cancelled — before it runs. |
| P7 | **Graceful degradation** | The design must survive 80×24, 16 colors, no mouse, and `NO_COLOR`. TrueColor and large grids are enhancements. |
| P8 | **Restraint** | If an element cannot justify its presence, it is removed. Whitespace is a feature. |

---

## 3. Reference Analysis

The two reference archives (`awesome-tui-design`, `tui-design-skill`) were inspected
in full before any design decision was made. Summary of the audit:

### 3.1 What the referenced TUIs do well — and what SetupForge borrows

| Source | Effective pattern | How SetupForge uses it |
|--------|-------------------|------------------------|
| lazygit | Border color communicates pane focus; titles/counts/position embedded in the border; keybinding bar always visible; `portraitMode` responsive stacking | Panes get a single accent border on focus; border headers show title, count, position; dock always shows contextual keys; breakpoint strategy stacks instead of squeezing |
| k9s | Breadcrumb context bar (`Context: prod > NS: default > Pods`); full-width sortable tables; selected row as high-contrast reverse video | Header shows a location breadcrumb; tables are the primary data instrument; selection is reverse video, not a colored row |
| btop | Inverted title brackets `┐title┌` that embed the title in the border; per-panel identity through restrained accent use | Titles live *in* the top border (never above it), saving a row; this is SetupForge's signature border detail |
| Rosé Pine | Muted borders that "frame, not compete"; semantic layering (base/surface/overlay/highlight); warm muted calm | Adopted as the structural model for surfaces and the calm baseline mood |
| Gruvbox | Warm cream foreground (never pure white); single-line structured borders; warm secondary palette for data values | Adopted the warm-foreground discipline and single-line border family |
| minimal | Neutral scale, bold-only emphasis, ≤4 visible colors | The density/contrast discipline; SetupForge caps simultaneous accent colors |
| tui-design-skill | Focus rules (one, visible, restored on return); Esc always works; destructive-confirm defaults to Cancel; spinner only when progress is unknowable; NO_COLOR / TERM=dumb fallbacks; wide-char width discipline | These become hard interaction rules (§5, §12) and token fallback requirements (DESIGN-SYSTEM.md) |

### 3.2 Weaknesses the references exhibit — and what SetupForge avoids

| Source | Weakness | SetupForge's rule |
|--------|----------|-------------------|
| k9s | Saturated neon palette (aqua/fuchsia/orange on black); high stimulation, poor for long sessions | One accent hue family only; muted surfaces; status colors appear only where state lives |
| btop | TrueColor gradient ramps as a core feature; decorative identity competes with data | No gradients in core UI; graphs (if any) use one ramp and degrade to blocks |
| lazygit (default) | 6–7 simultaneous panels; expert-oriented cognitive load | Max 2 panes in the stage at ≤120 cols; 4 only at XL; every panel closable |
| AI agent CLIs (claude-code, codex, gemini-cli) | Dashed/whimsical borders, ASCII art, per-message color coding | A *system tool* is not a chat log; no dashed borders, no figlet headers in the workspace |
| Generic Bubble Tea demos | One layout per screen; borders around everything | Borders carry structure only (§8); identical frame on every screen |
| "Retro"/"Cyberpunk" schemes | Theme as costume; double-line/heavy borders everywhere | Single border family; theme changes palette, never geometry |

### 3.3 Extracted principles (not copied screens)

1. **Borders are a language, not a fence.** One border family, muted by default,
   accented only on the focused pane.
2. **Tables are the workhorse of a sysadmin tool** — but lists, trees, and
   key/value layouts have specific jobs (§15, DESIGN-SYSTEM.md §7).
3. **Selection = reverse video.** Works at 16 colors, survives `NO_COLOR`
   (degrades to bold), readable for color-blind users.
4. **All chrome is earned.** Every row of header, dock, and border must justify
   its existence every second it is on screen.
5. **Never a dead screen.** Any wait longer than ~200 ms gets a designed
   loading state in place of the content it replaces.

---

## 4. Information Architecture

### 4.1 Reorganizing the candidate list

The brief's candidate areas (Dashboard, Packages, Applications, Fonts,
Development, Shell, Desktop, Services, Repositories, System, Logs, Settings)
are organized by *system concept*. Users are organized by *intent*. Re-grouping
by intent, frequency, and task relationship gives **seven destinations** instead
of twelve — shallower navigation, less cognitive load, and each destination maps
to a real post-install job:

| Rail destination | Contains (tabs) | User intent |
|------------------|-----------------|-------------|
| **Overview** | (none) | "Where am I, what needs attention, what's next" |
| **Software** | Packages · Applications · Repositories | "Get the software I need, from the right sources" |
| **Workstation** | Desktop · Fonts · Shell | "Make this machine feel like mine" |
| **Development** | (single surface) | "Set up my toolchain in one pass" |
| **System** | Updates & Services · System Info | "Keep the machine healthy; understand it" |
| **Journal** | (single surface, filterable) | "What has SetupForge done; what went wrong" |
| **Settings** | General · Appearance · Operations | "How SetupForge itself behaves" |

Rationale for the merges:

- **Repositories moved under Software.** A repository is meaningless outside
  package operations; users think "install X," not "configure repos." Repos get
  full-tab prominence *inside* the place they affect, with inline warnings when
  a selected package is unavailable.
- **Updates moved under System, promoted to first tab** — with a global update
  badge in the header so it never hides. Updates are a health task, not a
  browsing task.
- **Fonts grouped with Desktop/Shell as Workstation.** Font installation is a
  personalization task; grouping them shares the same "preview → apply" pattern.
- **Logs renamed Journal.** One flat log list is a debugging surface, not a
  destination hierarchy; it lives as a filterable stream with severity facets.
- **Dashboard renamed Overview.** It answers three questions only: system
  state, pending actions, recent activity. It is a launchpad, not a control
  room — no dense metrics grid (btop exists for that).

### 4.2 Navigation tiers

```
┌─ Primary ──────── Rail destinations 1–7          (always one hop away)
├─ Secondary ────── Tabs within a destination      (Tab key cycles)
├─ Contextual ───── Inspector, filters, row actions (Enter / on selection)
├─ Command ──────── Command palette (Ctrl+K)       (any action, anywhere)
└─ Global ───────── Help ?, Quit Ctrl+Q, palette Ctrl+K, rail toggle Ctrl+B
```

Rules:

- Maximum depth: Rail → Tab → Row → Inspector. No screen requires more than
  three levels of drilling, and `Esc` unwinds one level per press.
- Anything reachable at depth 3 must also be reachable from the command
  palette at depth 1.
- Settings contains only SetupForge's own behavior. Anything that modifies the
  OS lives in the destination for that system area.

---

## 5. Navigation Architecture

### 5.1 Focus model

- Exactly **one** focus target at all times: a pane (rail, stage pane, inspector),
  a dialog, the palette, or a form field.
- Focus is visible by **reverse video on the selection** plus **accent border**
  on the focused pane. Never color-only.
- `Tab` / `Shift+Tab` cycles panes in reading order (rail → main pane →
  inspector → rail). Within a pane, `j/k` (or arrows) move the selection.
- Focus is **restored on return**: leaving Packages for Fonts and coming back
  returns to the same row, same filter, same scroll position.
- Dialogs and the palette are focus traps: `Esc` always exits; there is no state
  the user can enter that they cannot leave.

### 5.2 Key map (complete; nothing else is required of the user)

| Keys | Context | Action |
|------|---------|--------|
| `1`–`7` | global | Jump to rail destination |
| `Tab` / `Shift+Tab` | global | Cycle pane focus |
| `j/k` or `↑/↓` | lists, tables | Move selection |
| `h/l` or `←/→` | trees, breadcrumbs | Collapse/expand, back/forward |
| `Enter` | global | Activate selection / drill in |
| `Esc` | global | Unwind one level / close overlay (never quits) |
| `/` | browsable surfaces | Focus filter |
| `Space` | tables, trees | Mark/unmark (batch selection) |
| `a` / `A` | tables | Mark all / unmark all |
| `i` | tables, lists | Toggle inspector |
| `r` | global | Refresh current surface |
| `Ctrl+K` | global | Command palette |
| `Ctrl+B` | global | Toggle rail (small terminals) |
| `g`/`G` or `Home`/`End` | lists | First / last item |
| `?` | global | Contextual help |
| `Ctrl+Q` | global | Quit (with confirmation if operations run) |

Design intent: **12 memorable keys cover 95% of usage.** Everything else lives
behind the palette, which is the discoverability layer — a user should never
need to memorize a shortcut that isn't shown in the dock.

### 5.3 State vocabulary

- **Focused** — the pane that receives input. One per screen. Accent border.
- **Selected** — the item within a list/table that input acts on. Reverse video.
- **Marked** — items chosen for a batch operation. `●` prefix, accent color.
- **Active** — a running operation or an enabled service. Spinner/running glyph.
- **Hover** (mouse) — never carries unique information; mirrors focus visuals.

### 5.4 Mouse interaction

Mouse enhances, never replaces: rail items, tabs, rows, buttons, dock hints,
palette entries, and scroll regions are clickable; the wheel scrolls the pane
under the cursor. Every click target has a keyboard equivalent. Clicking a pane
focuses it (same as `Tab`).

---

## 6. Workspace Model — precise definitions

The brief asks for exact definitions. These are normative:

| Term | Definition | Lifetime | Geometry |
|------|-----------|----------|----------|
| **Workspace** | The entire application: frame + stage. One per process. | Persistent for the session | Terminal size |
| **Frame** | Header + rail + dock. Identical on every screen. | Persistent | Fixed rows/cols per breakpoint |
| **Page** | A rail destination (Overview, Software, …). What the user "is on." | Until user navigates | One page visible at a time |
| **Pane** | A standard container in the stage holding one task surface. | While its page is visible | Standard pane units (§8) |
| **Surface** | The content kind inside a pane: table, form, tree, detail, log stream, dashboard. | While pane exists | Fills its pane |
| **Inspector** | A detail pane bound to the selection of another pane. | Open/closed per user; state remembered per page | Half pane (or sheet when small) |
| **Sheet** | A temporary edge-attached panel (filter, activity, help) that slides over the stage without a modal backdrop. | Task-scoped | Partial width, full stage height |
| **Dialog** | A centered modal with backdrop dim. Captures focus; `Esc` cancels. | Until dismissed | Standard dialog sizes |
| **Palette** | The command surface (Ctrl+K). A dialog variant with search. | While open | Top-centered overlay |
| **Toast** | A transient, non-interactive confirmation line. Does not capture focus. | ≤4 s | Dock-adjacent, right-aligned |

What is **persistent between screens**: frame, rail selection, dock, focus memory
per page, filter/scroll/inspector state per page, theme, and window geometry.

What changes **dynamically**: stage content (per page/tab), pane splits
(breakpoint-driven), inspector visibility, dock hints (per focus), overlays.

**What SetupForge deliberately does not have:** floating/overlapping windows,
free resizing, a tiling WM layout engine, per-screen custom chrome. A system
tool must be legible; geometry is architected, not improvised.

---

## 7. Global Application Frame

```
 Row 0   HEADER  — identity · breadcrumb · system badge · clock
 Rows…   RAIL    │  STAGE (panes on the macro-grid)
         │        │
 Last    DOCK    — context keys · operation status · filter state
```

### 7.1 Header (1 row, earns its row)

```
 SetupForge  Software › Packages                       Fedora 43 · 12 updates ●
```

- `SetupForge` — wordmark, accent, bold. Identity without ASCII art.
- Breadcrumb — current page (and tab). Location is the second-strongest signal
  on screen (after the selection).
- Right cluster — distro/release, then **update badge** (count, warning color
  when >0, hidden at <100 cols), then clock only at ≥120 cols.
- Everything in the header is ambient: none of it is interactive.

### 7.2 Rail (primary navigation)

```
 1 Overview
 2 Software
 3 Workstation
 4 Development
 5 System
 6 Journal
 7 Settings
```

- Vertical, left edge, numbered. Numbers are the shortcuts (`1`–`7`) and double
  as ordinal affordance for mouse users.
- Active destination: accent bar `▌` + bold. Inactive: muted.
- Widths: 18 (≥120 cols) → 14 (100–119) → collapsed to a single context
  indicator + `Ctrl+B` reveal (<100).
- At <100 cols the rail becomes an overlay (sheet), not a squeezed column.

### 7.3 Dock (1 row, contextual)

```
 ● 3 marked   j/k move · Space mark · Enter install · i inspect · / filter · Ctrl+K commands
```

- Left: operation/marking state (spinner or `● n marked` or filter state).
- Right: key hints **for the current focus only**, `key: action` format, capped
  at what fits; the rest lives in `?` help and the palette.
- The dock is the discoverability engine: no hidden bindings (anti-pattern A1).

### 7.4 Frame budget

| Size class | Chrome rows | Rail cols |
|------------|-------------|-----------|
| XL ≥150×42 | 2 | 18 |
| L 120×35   | 2 | 18 |
| M 100×30   | 2 | 14 |
| S 80×24    | 2 | 0 (overlay) |

---

## 8. Spatial System — the Modular Bench

### 8.1 The unit grid

All stage geometry derives from one base cell, the **module (M)**:

- **1M width = ¼ of the stage width** (stage = terminal width − rail − 1 gutter).
- **1M height = ¼ of the stage height**.

Panes are composed only of these unit multiples — never arbitrary sizes:

| Pane unit | Composition | Used for |
|-----------|-------------|----------|
| **Whole** | 4M × 4M | Single-surface pages (Development, Settings forms, Journal) |
| **Half** | 2M × 4M | Master–detail default (browser + inspector) |
| **Wide** | 4M × 2M | Short wide panes: stat band on Overview, confirmations |
| **Quad** | 2M × 2M | Overview tiles (XL only) |

Why a unit grid: at any breakpoint, panes are instances of the *same* unit
family — they align to shared edges, share borders (zero gutter), and read as
parts of one interface. This is the deliberate spatial consistency the brief
asks for: not identical dimensions everywhere, but one recognizable geometry.

```
  Stage = 4M × 4M, panes snap to the grid:

  ┌───────┬───────┐  M M M M   ← shared border, zero gutter
  │       │       │
  │ Half  │ Half  │  Master + Inspector (default L/XL)
  │ 2M×4M │ 2M×4M │
  │       │       │
  └───────┴───────┘

  ┌───────┬───────┐
  │ Quad  │ Quad  │  Overview XL: four equal tiles
  ├───────┼───────┤  — instances of ONE unit, not four
  │ Quad  │ Quad  │    individually-sized boxes
  └───────┴───────┘
```

### 8.2 Border signature

Single-line borders (`┌─┐│└┘`), muted by default. Titles embedded **in** the
top border with inverted brackets — borrowed from btop, adopted as SetupForge's
signature:

```
┌─ Packages ─ 142 available ────────── 12 of 142 ─┐
```

- Title left after `┌─`, count after title, position right-aligned before `─┐`.
- All three metadata slots are optional; a pane shows only what it can justify.
- **Focused pane**: border + title turn accent, title gains bold. Exactly one
  accent-bordered pane per screen (P4).

### 8.3 What never varies

Alignment edges (panes share borders), title placement, padding (1 row / 2 cols
inside every pane), selection rendering, dock position. What may vary: pane
count per breakpoint, inspector open/closed, column sets in tables.

---

## 9. Responsive Layout System

### 9.1 Size classes and transformations

The design does not shrink — it **transforms**:

| Class | Grid | Rail | Stage composition | Behavior changes |
|-------|------|------|-------------------|------------------|
| **XL** ≥150×42 | 4M×4M | 18, pinned | Half+Half, or Quad tiles | Inspector persistent; Overview shows 4 quads; tables show ≥6 columns |
| **L** 120×35 | 4M×4M | 18, pinned | Half+Half | Default target. Inspector default-open on first use, remembered after |
| **M** 100×30 | 4M×4M | 14, pinned | Half+Half (narrower) or Whole | Tables drop to 4 priority columns; table → inspector ratio 55/45; clock removed; palette 80% width |
| **S** 80×24 | 2M wide / 4M tall | 0, overlay (`Ctrl+B`) | One pane at a time | Master and inspector are *separate views* (Enter drills, `Esc` returns); dock shows ≤4 hints; dialogs nearly full width; Overview becomes a single Whole pane with vertical stat list |

Transformation rules (apply in order as size decreases):

1. **Drop optional chrome** first (clock, header extras, table columns by priority).
2. **Collapse width** of rail, then overlay it.
3. **Merge panes into a drill-down** (Half+Half → sequential Whole views) —
   never shrink a two-pane layout into illegibility.
4. **Shrink dialog/palette** to safe margins (never edge-to-edge; ≥2 col margin).
5. **Truncate, then wrap, then elide** — with `…` and full value in inspector.

### 9.2 Per-element breakpoint contract

| Element | XL/L | M | S (80×24) |
|---------|------|---|-----------|
| Navigation | pinned rail | pinned rail | overlay rail; breadcrumb carries location |
| Panel count | 2 (4 on Overview) | 2 | 1 + drill |
| Table columns | full priority list | top-4 priorities | top-3, single-pane |
| Text truncation | column `…` at 60% pane width | column `…` at 45% | name-only + inspector for values |
| Scrolling | per-pane, synced inspector | per-pane | full-surface |
| Sidebar (inspector) | persistent half pane | persistent, 45% | drill-down view |
| Status/dock | full hint set | drop weakest hints | 4 hints max, grouped |
| Modal size | 88×15 | 80×13 | 76×11 |
| Palette | 60% width top | 80% width | 100%−4 cols |
| Detail views | right pane | right pane | full-screen drill |

### 9.3 Minimum viable interface

At 80×24 nothing is broken: one focused surface, breadcrumb in header, four
dock hints, palette for everything else. This is enforced as a design gate, not
an aspiration — every wireframe below exists in an S variant (§16).

---

## 10. Visual Hierarchy

Ranked. The hierarchy must survive with color removed (mono/bold/dim only):

| Rank | Element | Treatment |
|------|---------|-----------|
| 1 | Selection (focused item) | Reverse video — strongest signal on screen |
| 2 | Focused pane border | Accent + bold border + title |
| 3 | Active operation | Accent spinner + dock status |
| 4 | Primary data (table values, form values) | Body foreground |
| 5 | Section/pane titles | Bold, embedded in border |
| 6 | Secondary data (metadata, counts) | Muted |
| 7 | Key hints | Accent key + muted label (dock) |
| 8 | Decorative structure (borders, rules) | Border-muted, dimmest layer |

Never invert this order for effect. Two accents never compete on one screen:
if a dialog is open, the stage border loses its accent.

---

## 11. Command Palette

`Ctrl+K` opens the palette — the universal fallback for anything not worth a
shortcut. Design goals: instant, legible, honest about danger.

```
┌─ Commands ───────────────────────────────────────────────┐
│ ❯ upg                                                      │
├────────────────────────────────────────────────────────────┤
│ Update system                              System  Ctrl+U  │
│ Upgrade all packages                       Software        │
│ Manage system services                     System          │
│ Open update settings                       Settings        │
├────────────────────────────────────────────────────────────┤
│ ▸ Update system                                            │
│   Downloads 245 packages (1.2 GiB) · requires root · 4 min │
└────────────────────────────────────────────────────────────┘
```

Rules:

- **Groups**: *Go to…* (destinations), *Actions* (contextual), *Recent*.
  Contextual actions for the current selection appear first.
- **Fuzzy match** on name and keywords (`upg` matches "Upgrade"); ranking
  prefers exact-prefix, then frequency.
- **Preview line**: the selected command's consequence is always shown —
  what it changes, whether it needs root, estimated duration when known.
- **Destructive commands** carry a `!` prefix in red and open a confirmation
  dialog on execution — the palette never performs a destructive act directly.
- Keyboard: type, `j/k` move, `Enter` run, `Esc` close.
- Mouse: click to run; palette is keyboard-complete.

---

## 12. System-Operation UX

### 12.1 Operation lifecycle (designed, not incidental)

```
Ready → Confirm → Prepare → Execute → Result
                (queued)  (progress)  (success · partial · failure)
```

- **Ready** — action visible (button, row action, palette). Dock hints show it.
- **Confirm** — dialog for anything destructive, privileged, or >30 s. Dialog
  states: what happens, scope, root requirement, cancellability. Default
  button is always the safe one (Cancel); destructive option is red and never
  preselected.
- **Prepare** — dependency resolution, transaction build. Honest waiting with
  measurable steps ("Resolving dependencies… 38/41"). If it can be cancelled,
  the dock shows it.
- **Execute** — appears in the **Activity Sheet** (§12.2) with real progress
  (DNF percentages); spinner only where progress is genuinely unknowable.
- **Result** — minimal, precise feedback: `✓ git installed successfully` as a
  toast; nothing more for trivial ops. Partial success lists what failed and
  offers *View in Journal*. Failure follows §12.4.

### 12.2 The Activity Sheet

Operations run in a bottom sheet (not a modal — the user keeps working):

```
 Journal ────────────────────────────── filter: severity ≥ warn ──
 12:04:03  ✓ dnf  Install git-2.49.0-1.fc43.x86_64         14s
 12:03:41  ⚠ dnf  Repo 'terra' unreachable (retry 2/3)      —      running ▶
 12:01:12  ✗ svc  Enable docker.socket                     failed
```

- Toggle with `` ` `` (backtick); opens automatically when an operation starts.
- Running operations show spinner + cancellability (`c` cancel where supported).
- Every entry deep-links to its full log in Journal.

### 12.3 Loading states

| Situation | Indicator | Notes |
|-----------|-----------|-------|
| Initial app start | Whole-pane skeleton | Pane borders render immediately; content streams in |
| Package query / repo refresh | In-pane spinner + label | Replaces the table body only, never the whole stage |
| Installation / removal | Activity Sheet progress bar | Percent + package count + ETA |
| Network operation | Spinner + host name | So "waiting on mirrors.fedoraproject.org" is visible |
| Long operation (unknown progress) | Braille spinner | The only spinner-only case |
| Background refresh | Dock dot `●` | Silent; no spinner, no motion |

Hard rule: no blank or frozen pane may persist >200 ms without a labeled
loading state. Spinners are labeled with *what* is happening.

### 12.4 Error states

Errors answer four questions in this order: **What happened · Why (if known) ·
What was affected · What you can do next.**

```
┌─ Installation failed ──────────────────────────────┐
│                                                    │
│  Could not install 3 of 5 packages.                │
│                                                    │
│  libxcrypt-devel conflicts with an existing file   │
│  owned by libxcrypt-compat. This usually means a   │
│  partially completed system upgrade.               │
│                                                    │
│  Affected: libxcrypt-devel, krb5-devel             │
│                                                    │
│  [Retry]  [Skip conflicting]  [View full log]      │
│                                                    │
│  Details: file /usr/lib64/libcrypt.so.1 from       │
│  install of libxcrypt-devel-4.4.36-10.fc43...      │
│                                                    │
└────────────────────────────────────────────────────┘
```

- The raw `Error: exit status 1` class of message is never user-facing; it
  lives in the expandable *Details* region and in Journal.
- Errors are recoverable by construction: every error dialog offers at least
  one forward action (Retry / Skip / View log / Undo).

### 12.5 Success states

Quiet by design. Toast format, dock-adjacent, ≤4 s:

```
 ✓ git installed successfully
 ✓ 3 packages updated · reboot recommended
```

A reboot recommendation, partial result, or privileged side-effect earns one
extra clause — never a full-screen celebration.

### 12.6 Empty states

Every empty surface answers *why it's empty* and *what to do*:

| Surface | Empty state |
|---------|-------------|
| Package search | `No packages match "gruvbox-theme". Try the description field or enable more repositories.` + `[Browse all]` |
| Journal (filtered) | `No warnings or errors recorded. SetupForge has had a quiet run.` + `[Show everything]` |
| Repositories | `Only the official Fedora repositories are enabled.` + `[Add repository]` |
| Applications | `Nothing extra installed yet. Applications you add appear here.` + `[Browse applications]` |
| Services (filtered) | `No services match "vpn".` + `[Clear filter]` |
| Inspector (no selection) | `Select an item to inspect it.` (muted, centered) |

---

## 13. Design Exploration — three directions, then a synthesis

The brief requires genuinely different approaches before settling. These are
presented as tradeoffs, not scored rankings.

### Direction A — "The Bench" (persistent rail + master–detail)

Structured terminal desktop. Rail pinned left; stage runs Half+Half
browser/inspector everywhere; dock carries context. lazygit's spatial lessons
without its panel sprawl.

- **Strengths**: strongest spatial constancy (P1); inspector makes every list
  self-explanatory; shallow learning curve; scales naturally to S as drill-down.
- **Weaknesses**: at M sizes the half+half squeeze risks cramped tables;
  dock+rail+header is 20+ cols of chrome that must earn itself.
- **Terminal fit**: excellent — box borders + reverse video are native.
- **Implementation**: low complexity; one layout engine, three breakpoints.

### Direction B — "The Deck" (minimal, palette-centric focus)

Single centered Whole pane per page, no persistent inspector, sparse chrome.
Power flows through the palette; keyboard-first to the extreme.

- **Strengths**: calmest possible surface; zero clutter; fastest keyboard
  paths for experts; trivially responsive.
- **Weaknesses**: discoverability collapses for newcomers — if it's not in the
  palette or a hint, it doesn't exist; details require an extra drill for every
  item; risks feeling like "a collection of menus" (explicitly forbidden).
- **Terminal fit**: good, but wastes the terminal's 2-D nature.
- **Implementation**: lowest complexity; too little structure to build identity on.

### Direction C — "The Grid" (modular equal-pane workspace)

True tiled workspace: 2×2 equal Quad panes everywhere, interchangeable, with
context swapping panes in and out. Dashboard-forward.

- **Strengths**: maximal information on XL screens; pane unity is total;
  distinctive.
- **Weaknesses**: violates cognitive-load goals at L and below — four live
  surfaces compete constantly; most pages don't *have* four meaningful views;
  drifts toward a WM clone (explicitly forbidden) and toward btop-style density.
- **Terminal fit**: good at XL, poor at M/S.
- **Implementation**: highest complexity — pane manager, focus routing across
  four targets, per-pane state persistence.

### 13.1 Synthesis — "The Modular Bench" (adopted)

Direction A is the chassis; C contributes the **unit grid** (§8) — geometry
snaps to modules, so even the single-pane S layout is visibly the *same*
system; B contributes **palette-as-first-class** and the discipline that chrome
must earn its space.

Result: a persistent rail + dock frame; a stage of unit-grid panes
(Half+Half default, Whole for form-like pages, Quad tiles only on Overview at
XL); inspector as drill-down when small; palette as the universal fallback.
Architected, not repetitive; calm, not sparse; dense, not cluttered.

Adopted tradeoffs, stated honestly:

- Two panes maximum at L/M costs XL users some simultaneous context — accepted;
  Overview's Quad tiles and the inspector recover it where space is real.
- The dock+rail chrome costs ~21 cols — accepted, because discoverability
  (P2) depends on it, and it collapses gracefully at S.
- Uniform pane geometry occasionally under-fills a page — accepted; a pane
  renders a designed empty state rather than resizing itself ad hoc.

---

## 14. Screen Inventory & ASCII Wireframes

Seventeen screens, each with an ASCII wireframe. All wireframes are design
concepts (not implementations),
drawn at 120×35 unless noted; responsive variants in §16.

### S1 · Overview

The launchpad. Four questions only: *How is the system? What needs attention?
What's running? What did I do recently?* No metric wall — `btop` owns that job.

```
 SetupForge  Overview                              Fedora 43 · 12 updates ●  21:04
 1 Overview        ┌─ Health ────────────┬─ Pending ────────────────────────┐
 2 Software        │ CPU     12%  ▂▃▂▁   │ ● 12 updates available            │
 3 Workstation     │ Memory  6.1 / 15.7G │   security: 3  bugfix: 6          │
 4 Development     │ Disk    41% ▓▓▓▓░░  │ ● Reboot required (kernel)        │
 5 System          │ Net     ▲ 2.1 MB/s  │                                   │
 6 Journal         │ Uptime  6 days      │   [u] Review updates              │
 7 Settings        ├─────────────────────┤                                   │
                   │─ Activity (3 live) ─├─ Recent ──────────────────────────┤
                   │ ▶ dnf refresh repo  │ ✓ git installed           12:04  │
                   │ ▶ flatpak update    │ ✓ fonts: JetBrains Mono 11:58  │
                   │                     │ ⚠ repo 'terra' retrying  11:41  │
                   │                     │ ✓ shell: zsh + starship  11:20  │
                   │                     │                                   │
                   └─────────────────────┴───────────────────────────────────┘
 ● ready  1..7 go to · Tab panes · j/k move · Enter open · Ctrl+K commands
```

### S2 · Package browser (Software › Packages)

Master–detail default. Filter is first-class (`/`). Batch marking is the core
loop: mark → act.

```
 SetupForge  Software › Packages                       Fedora 43 · 12 updates ●
 1 Overview        ┌─ Packages · repo: all ─ 142 shown ── 4 of 142 ──────────┐
 2 Software        │   Name                    Version        State        │
 3 Workstation     │ ❯ git                     2.49.0-1       installed ●  │
 4 Development     │   gruvbox-theme-gtk       1.0.0-3        available    │
 5 System          │   gtk4                    4.18.2-1       installed    │
 6 Journal         │   gvfs                    1.57.2-1       update avail │
 7 Settings        │   iwlwifi-firmware…       20250811       installed    │
                   │   ▲ marked for install                                 │
                   │                                                        │
                   ├─ git 2.49.0-1 · installed ─────────────────────────────┤
                   │  A fast, scalable, distributed revision control        │
                   │  system.                                               │
                   │  Repo   fedora    Size  108 kB    License  GPL-2.0    │
                   │  [r] Remove   [d] Downgrade   [i] Info                 │
                   └────────────────────────────────────────────────────────┘
 ● 1 marked  j/k move · Space mark · Enter act · i inspect · / filter · a all
```

### S3 · Package details (inspector, full)

Inspector opened wide (`i` on a row). Same Half pane geometry — details are
content in the same system, not a new screen.

```
 ┌─ gruvbox-theme-gtk 1.0.0-3 ─ repo: fedora ────────────────────────────────┐
 │ Retro groove GTK theme. Warm earthy tones for GTK4 and libadwaita.        │
 │                                                                            │
 │ Version   1.0.0-3            Size     2.4 MB          License  MIT         │
 │ Source    fedora             Installed never          Arch     noarch      │
 │                                                                            │
 │ Requires  gtk4 ≥ 4.14, gnome-themes-extra                                │
 │ Provides  theme(gruvbox), gtk-theme(gruvbox)                             │
 │ Conflicts gruvbox-gtk-theme (obsolete name)                              │
 │                                                                            │
 │ [i] Install   [q] Queue with marked   [o] Open project page              │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S4 · Application browser (Software › Applications)

Applications are Flatpaks/user apps — cards in a flowing list, not a table,
because the decision unit is "what it is + source + size," not sortable data.

```
 ┌─ Applications ─ 24 shown ──────────────────────────── 1 of 24 ────────────┐
 │ ❯ firefox              Flatpak · flathub     98 MB    [install]           │
 │   thunderbird          Flatpak · flathub    121 MB    [install]           │
 │   obsidian             Flatpak · flathub    190 MB    installed ●         │
 │   vlc                  RPM · fedora          62 MB    installed ●         │
 │   krita                Flatpak · flathub    210 MB    [install]           │
 │                                                                           │
 │ Filter: [desktop] [internet] [media] [dev]            / search           │
 └───────────────────────────────────────────────────────────────────────────┘
```

### S5 · Fonts (Workstation › Fonts)

Preview-first: font decisions are visual, so the inspector previews a pangram
in the actual font when installed, specimen text otherwise.

```
 ┌─ Fonts ─ 9 shown ──────────────────────────────── 2 of 9 ─────────────────┐
 │   Family                Styles     State                     │
 │ ❯ JetBrains Mono        8          installed ●             │
 │   Inter                 18         installed ●             │
 │   Fira Code             5          available               │
 │   Noto Sans CJK SC      1          available               │
 │                                                            │
 ├─ JetBrains Mono ─ preview ─────────────────────────────────┤
 │  The quick brown fox 0123456789 →≠≡                        │
 │  int main(void) { return 0; }  // code ligatures: != >=    │
 │  [r] Remove   [v] variants (8 styles)                      │
 └────────────────────────────────────────────────────────────┘
```

### S6 · Development environment

A checklist matrix, not a package list — users think in toolchains. Whole pane
at L, grouped by domain.

```
 ┌─ Development ─ profiles: none active ─────────────────────────────────────┐
 │  Languages      [x] Go 1.24     [x] Rust (rustup)    [ ] Java 21          │
 │                 [x] Python 3.13  [ ] Node 22 (dnf)    [ ] .NET 9          │
 │                                                                            │
 │  Editors/tools  [x] Neovim  [x] VS Code (flatpak)  [ ] Emacs            │
 │                 [x] lazygit  [x] ripgrep  [x] fd  [ ] docker + compose  │
 │                                                                            │
 │  Shell extras   [x] zsh  [x] starship  [ ] fzf  [x] eza                  │
 │                                                                            │
 │  [p] Apply profile · Go CLI dev → installs 14 packages (38 MB)            │
 │  Space toggle · a mark group · apply always confirms first                │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S7 · Shell configuration (Workstation › Shell)

```
 ┌─ Shell ─ current: bash ────────────────────────────────────────────────────┐
 │  Shell     (●) zsh    ( ) bash    ( ) fish                                  │
 │  Prompt    (●) starship    ( ) oh-my-zsh    ( ) plain                       │
 │  Terminal  [x] colors (truecolor)  [x] mouse reporting  [ ] bell           │
 │                                                                            │
 │  rc files  (●) merge: SetupForge writes a managed block you can edit       │
 │            ( ) replace: generate ~/.zshrc from template                    │
 │            ( ) link: symlink into ~/dotfiles                               │
 │                                                                            │
 │  [v] view diff of changes that would be applied to ~/.zshrc                │
 │  [Apply] · takes effect in new terminal sessions                           │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S8 · Desktop configuration (Workstation › Desktop)

GNOME-first (Fedora target), expressed as intent toggles with honest scope.

```
 ┌─ Desktop ─ GNOME 48 ───────────────────────────────────────────────────────┐
 │  Appearance  [x] Dark style   ( ) Light   Theme: (Adwaita) (Gruvbox…)      │
 │  Extensions  [x] appindicator  [ ] dash-to-dock  [ ] blur-my-shell        │
 │              Extensions install from extensions.gnome.org                  │
 │  Workflow    ( ) hot corner  (●) dynamic workspaces   Count: 4            │
 │  Defaults    Browser: [firefox ▾]   Terminal: [ptyxis ▾]                  │
 │                                                                            │
 │  [Apply] · applies via gsettings · fully reversible from this screen      │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S9 · Services (System › Updates & Services · Services tab)

Services get a table (sortable state columns); units are data.

```
 ┌─ Services ─ 34 shown · filter: ──────────────────── 3 of 34 ──────────────┐
 │   Unit                      State      Enabled    Description             │
 │ ❯ NetworkManager.service    active ●   enabled    Network manager         │
 │   bluetooth.service         active ●   enabled    Bluetooth daemon        │
 │   cups.service              inactive   disabled   Print scheduler         │
 │   docker.socket             active ●   enabled    Docker API socket       │
 │   firewalld.service         active ●   enabled    Dynamic firewall daemon │
 │                                                                            │
 │ s start/stop · e enable/disable · l logs · f filter                       │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S10 · Repositories (Software › Repositories)

```
 ┌─ Repositories ─ 6 enabled ─────────────────────────────────────────────────┐
 │   Repository          Source        State                    Priority       │
 │   fedora              official ●   enabled                  99              │
 │   updates             official ●   enabled                  99              │
 │   terra               third-party  enabled ⚠ unreachable  100 (retrying)   │
 │   rpmfusion-free      third-party  disabled                 100            │
 │   copr: atim/starship third-party  enabled                  100            │
 │                                                                            │
 │ a add · e enable/disable · r refresh · ⚠ rows link to Journal            │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S11 · System information

Key/value Whole pane — read-only reference data, never a table of one row.

```
 ┌─ System Info ────────────────────────────────────────────────────────────────┐
 │  Host        fedora-workstation    Kernel    6.17.4-200.fc43.x86_64         │
 │  OS          Fedora Linux 43       Session   Wayland · GNOME 48             │
 │  CPU         Ryzen 7 7840U (16)    Memory    6.1 / 15.7 GiB                │
 │  GPU         Radeon 780M           Disk      512 GB NVMe · 41% used        │
 │  Secure Boot enabled ●             TPM        2.0 ●                        │
 │                                                                            │
 │  [c] copy summary to clipboard (plain text)   [j] open in Journal context  │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S12 · Journal

A log stream with facet filters — flat, chronological, greppable.

```
 ┌─ Journal ─ 148 entries · filter: severity ≥ warn ──────────────────────────┐
 │ 12:04:03  INFO   dnf      Install git-2.49.0-1 → success (14s)            │
 │ 12:03:41  WARN   dnf      repo 'terra' unreachable — retry 2 of 3 (▶)     │
 │ 12:01:12  ERROR  service  enable docker.socket → exit 1: unit masked      │
 │ 11:58:44  INFO   fonts     Install JetBrainsMono → success                 │
 │                                                                            │
 │ facets: [severity ▾] [source ▾] [time: today ▾]      / search · r refresh │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S13 · Settings

```
 ┌─ Settings · General ─────────────────────────────────────────────────────────┐
 │  Behavior    (●) confirm before system changes      [x] dry-run by default  │
 │              ( ) apply immediately (dangerous)                              │
 │  Appearance  Theme: [Forge ▾]   Density: [(●) balanced  ( ) compact]        │
 │              Accent follows theme — no raw color pickers                    │
 │  Operations  Parallel jobs: [2 ▾]   Timeout: [300s ▾]   Journal: 50 MB      │
 │                                                                            │
 │  Changes apply instantly; no restart required                                │
 └────────────────────────────────────────────────────────────────────────────┘
```

### S14 · Command palette — see §11 (full wireframe there).

### S15 · Confirmation dialog

```
                        ┌─ Install 3 packages ────────────────────────┐
                        │                                             │
                        │  git 2.49.0-1 · gruvbox-theme-gtk 1.0.0-3   │
                        │  fira-code-fonts 6.202 — total 14.2 MB      │
                        │                                             │
                        │  Requires root (sudo) · ~40 seconds ·       │
                        │  cancellable until the transaction begins.  │
                        │                                             │
                        │        [Cancel]   [ ! Install ]             │
                        └─────────────────────────────────────────────┘
```

Destructive variant: title in error color, `!` on the action, default focus on
Cancel.

### S16 · Progress operation (Activity Sheet open)

```
 ┌─ Packages · repo: all ─────────────────────────────────────────────────────┐
 │  …browser remains live; the user keeps browsing…                            │
 ├─ Activity ───────────────────────────────────────── running 1 · queued 1 ─┤
 │ ▶ Install 3 packages   ████████████░░░░░░░░  58%   12/21 pkgs  ~18s   [c]  │
 │ ○ flatpak update       queued                                              │
 └────────────────────────────────────────────────────────────────────────────┘
 ● installing…  c cancel · ` activity · Ctrl+K commands
```

### S17 · Error state (workspace-level, when the whole stage is blocked)

```
 ┌─ SetupForge could not reach the package database ────────────────────────────┐
 │                                                                               │
 │  DNF returned an error while refreshing repository metadata.                 │
 │                                                                               │
 │  Most likely cause: network is unavailable or a VPN is blocking DNS.         │
 │                                                                               │
 │  [Retry]   [Work offline]   [View details]   [Open Journal]                  │
 │                                                                               │
 │  Details: Curl error (6): Could not resolve host: mirrors.fedoraproject.org  │
 └───────────────────────────────────────────────────────────────────────────────┘
```

Note: S17 exists at two scopes — inline (errors live in the pane that caused
them) and workspace-level (shown above, only when the whole stage is blocked).

---

## 15. Data-Instrument Rules (table vs list vs tree vs cards)

| Instrument | Use when | Never for |
|------------|----------|-----------|
| **Table** | ≥3 sortable/comparable columns of homogeneous items (packages, services, repos, journal) | Two-column config (use forms); visual choices (use cards) |
| **List** | Single-dimension navigation or actions | Multi-attribute comparison |
| **Tree** | True hierarchy (font variants, package groups, dotfile folders) | Flat data forced into fake hierarchy |
| **Cards** | Decision unit is identity + description (applications) | Dense data |
| **Key/value** | Read-only reference (System Info, package metadata) | Editable config (use forms) |
| **Form** | Mutating settings (Shell, Desktop, Settings) | Read-only display |

Table specifics: header bold; text left, numbers right, status center; sort
marker on the sorted column; pagination `12 of 142` in the border, never a
second footer row; active row reverse video; columns drop by priority list per
breakpoint (§9.2), never squeeze below readable width.

Visual density rules (for every screen): whitespace comes from the spacing
scale, not from empty borders; grouping comes from alignment and proximity
before lines; scanning is served by consistent column positions across screens;
truncation always ends in `…` with the full value one drill away; repetition of
the pane/border/table idioms is what makes density feel calm rather than busy.

---

## 16. Responsive Examples (layout adaptation, not rescaling)

### 16.1 Overview at S (80×24) — Quad tiles become one focused pane

```
 SetupForge  Overview · Fedora 43 · 12 updates ●
 ┌─ Health ────────────────────────────────────────────────────────────────┐
 │ CPU 12% · Mem 6.1/15.7G · Disk 41% · Uptime 6d                          │
 │ ▲ 2.1 MB/s                                                              │
 │                                                                          │
 │ ● 12 updates available (3 security) — [u] review                         │
 │ ● Reboot required                                                        │
 │                                                                          │
 │ Activity: ▶ dnf refresh repo · ▶ flatpak update    [`] open activity    │
 └──────────────────────────────────────────────────────────────────────────┘
 1..7 go to · Enter open · Ctrl+K commands · ? help
```

The same four Overview questions are answered in one pane; tile identity
survives as labeled groups, not as resized boxes.

### 16.2 Packages at S (80×24) — master/detail become drill-down

```
 SetupForge  Software › Packages · 142 shown
 ┌─ Packages ───────────────────────────────────────────── 4 of 142 ────────┐
 │   Name                  Version       State                             │
 │ ❯ git                   2.49.0-1      installed ●                      │
 │   gruvbox-theme-gtk     1.0.0-3       available                        │
 │   gtk4                  4.18.2-1      installed                        │
 │   gvfs                  1.57.2-1      update avail                     │
 │   iwlwifi-firmware…     20250811      installed                        │
 │   ▲ marked for install                                                 │
 │                                                                         │
 │   Space mark · Enter details · / filter · a mark all                   │
 └──────────────────────────────────────────────────────────────────────────┘
 ● 1 marked  Enter details · / filter · Ctrl+K commands
```

`Enter` on a row drills to the S3 detail view full-screen; `Esc` returns here
with selection and scroll restored. No two-pane squeeze.

### 16.3 Packages at XL (160×45) — same units, more columns, room to breathe

```
 SetupForge  Software › Packages                                      Fedora 43 · 12 updates ●  21:04
 1 Overview        ┌─ Packages · repo: all ─ 142 shown ─────────────── 4 of 142 ────────────────────────┐
 2 Software        │   Name               Version        State        Size    License   Repo           │
 3 Workstation     │ ❯ git                2.49.0-1       installed ●  108 kB  GPL-2.0   fedora          │
 4 Development     │   gruvbox-theme-gtk  1.0.0-3        available    2.4 MB  MIT       fedora          │
 5 System          │   gtk4               4.18.2-1       installed    26 MB   LGPL-2.1  fedora          │
 6 Journal         │   gvfs               1.57.2-1       update avail 1.1 MB  GPL-3.0   updates         │
 7 Settings        │   iwlwifi-firmware…  20250811       installed    78 MB   redist    updates         │
                   │                                                                                     │
                   │                                                                                     │
                   │                                                                                     │
                   │                                                                                     │
                   │                                                                                     │
                   │                                                                                     │
                   │                                                                                     │
                   ├─ gruvbox-theme-gtk 1.0.0-3 · available ────────────────────────────────────────────┤
                   │  Retro groove GTK theme. Warm earthy tones for GTK4 and libadwaita.                   │
                   │  Size 2.4 MB · License MIT · Requires gtk4 ≥ 4.14 · Installed never                  │
                   │  [i] Install · [q] Queue with marked · [o] Project page                               │
                   └───────────────────────────────────────────────────────────────────────────────────────┘
 ● 1 marked  j/k move · Space mark · Enter act · i inspect · / filter · a mark all
```

Geometry is unchanged (Half+Half, same border signature) — the extra columns
and padding are the *content* adapting, not the structure changing.

---

## 17. Accessibility

- **No color-only meaning**: every state pairs color with a glyph or weight
  (`✓/⚠/✗/●/▸`). Full spec in DESIGN-SYSTEM.md §10.
- **Keyboard-only**: 100% of features operable; focus always visible (reverse
  video + accent border); `Tab` order is reading order; focus restored on return.
- **Contrast**: body text ≥ 4.5:1 against its surface in the default theme;
  muted text ≥ 3:1; status colors never used for body copy.
- **16-color / `NO_COLOR` / `TERM=dumb`**: every token ships ANSI 16 and
  mono fallbacks (bold/dim/reverse only); layout never depends on color.
- **Reduced density**: Settings offers *compact* density (tighter padding,
  zebra rows) for users who want more per screen — an opt-in, never default.
- **Reduced motion**: spinners degrade to static `…`; no animated transitions
  exist anywhere in the design.
- **Wide characters**: all string handling uses display width, never byte or
  rune count; CJK package descriptions truncate by width with `…`.
- **Unicode fallback**: box drawing degrades to ASCII (`+-|`) with identical
  layout math; no glyph carries information that words don't also carry.
- **Screen readers**: TUIs are poorly served by screen readers; SetupForge
  provides `--dump` (structured plain-text output of any surface) as the
  practical assist, documented in `--help`.

---

## 18. Design Decisions & Rationale (the short list that matters)

| Decision | Chosen | Rejected alternative | Why |
|----------|--------|----------------------|-----|
| Primary navigation | Pinned numbered rail | Top tab bar; hamburger; palette-only | Always-visible destinations + self-documenting shortcuts; tabs waste a row and hide count |
| Pane geometry | Unit grid (M modules) | Free layout; fixed 50/50 everywhere; WM tiling | Architected consistency without rigidity; scales across breakpoints |
| Selection | Reverse video | Colored row; arrow-only | Survives 16-color and mono; unambiguous at a glance |
| Focus signal | Accent border + bold title | Background tint; blinking; double border | Exactly one accent on screen; zero motion |
| Borders | Single-line, muted, titled-in-border | Rounded; double; borderless; dashed | One family, one signature detail; dashed = chat-log aesthetics (rejected) |
| Default theme | Original "Forge" warm palette | Ship Rose Pine or Gruvbox as-is | SetupForge needs its own identity; Rose Pine + Gruvbox ship as alternate *mappings of the same semantic tokens* (DESIGN-SYSTEM.md §3) |
| Theme architecture | Semantic tokens → theme files | Raw hex in screens; per-widget palettes | Future themes are data, not code |
| Tables | Priority-column drop per breakpoint | Horizontal squeeze; horizontal scroll | Legibility survives; scroll hides data |
| Inspector | Persistent pane → drill-down at S | Always modal; always inline | Context without modal fatigue; small terminals keep one focus |
| Modals | Rare; centered dialog w/ dim | Inline-everything; modal-everything | Destructive/irreversible confirmations only; everything else is pane or sheet |
| Empty/loading/error | Designed states per surface | Spinners as content; error strings | The app never looks frozen or broken |
| Updates surface | Badge in header + System › Updates | Dedicated Dashboard widget wall | Attention where attention belongs; Overview stays a launchpad |

---

## 19. Final Design Standard — self-check

| Question | Answer |
|----------|--------|
| Does a first-time user see a carefully designed application? | Yes — fixed frame, titled borders, one accent, aligned grid. |
| Does Packages → Fonts → Services → Settings feel like one application? | Yes — identical frame, pane units, border signature; only content and tab labels change. |
| Does every window belong to the same spatial system? | Yes — all panes are M-unit multiples sharing borders and padding. |
| Is the interface coherent at 80×24? | Yes — S variants designed (§16), not shrunk; palette covers the rest. |
| Can a keyboard-only user operate everything? | Yes — 12-key core + palette; mouse strictly additive. |
| Does it feel like a terminal app, not a web app in a terminal? | Yes — no gradients, no shadows, no cards; characters, weight, box drawing. |
| Enough whitespace without wasting space? | Yes — 1row/2col padding, zero-gutter shared borders, earned chrome only. |
| Enough information without overwhelm? | Yes — ≤2 live panes (4 only as Overview tiles at XL), progressive disclosure. |
| Does SetupForge have its own identity? | Yes — the Modular Bench geometry, titled muted borders, Forge palette, dock-driven discoverability. |

All answers pass. The design system and workspace model are coherent;
implementation may begin.
