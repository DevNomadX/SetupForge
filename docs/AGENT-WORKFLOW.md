# SetupForge AI Agent Workflow

This document is the procedural map of the instruction system: how the layers
fit together, which skill fires when, and what "done" means. It applies to
Google Antigravity coding agents working in this repository.

For the contract itself read `AGENTS.md`. For what the system *is* read
`docs/ARCHITECTURE.md`. This file describes the *process*.

## Instruction hierarchy

```text
AGENTS.md                     — always-on contract: identity, boundaries, invariants
  │
  ├── .agents/rules/          — scoped constraints ("what must be true")
  │     ├── architecture.md
  │     ├── ui.md
  │     ├── go-bubbletea.md
  │     ├── system-safety.md
  │     ├── testing.md
  │     └── git.md
  │
  ├── .agents/skills/         — procedural workflows ("how to do this work")
  │     ├── feature-planning
  │     ├── ui-ux-design
  │     ├── tui-architecture
  │     ├── bubbletea-development
  │     ├── bubbles-development
  │     ├── lipgloss-design
  │     ├── tui-responsive-layout
  │     ├── tui-input-navigation
  │     ├── tui-async-operations
  │     ├── tui-testing
  │     ├── tui-visual-review
  │     ├── fedora-system-integration
  │     ├── code-review
  │     └── self-review
  │
  ├── docs/                   — reference knowledge ("what the system is")
  │     ├── ARCHITECTURE.md
  │     ├── DESIGN-SYSTEM.md
  │     ├── UI-UX-DESIGN.md
  │     ├── SECURITY.md       (rationale; rules/system-safety.md is enforceable)
  │     ├── PACKAGE-UI.md
  │     └── CONTRIBUTING.md
  │
  └── project/                — project state, roadmap, and decisions
        ├── STATUS.md
        ├── ROADMAP.md
        ├── DECISIONS.md
        └── CURRENT-WORK.md
```

Antigravity discovers workspace rules and skills from `.agents`. Rules are
persistent constraints; skills are focused workflows, automatically selected
or explicitly invoked.

## Normal feature workflow

### Phase 1 — Understand

1. Inspect the repository.
2. Read the rules relevant to the task (`AGENTS.md` §11 routes them).
3. Search for existing implementations, components, and services.
4. Identify architectural boundaries the change will cross.
5. Identify safety, privilege, and failure implications.

### Phase 2 — Plan

Use `feature-planning`. For UI features, also use `ui-ux-design`.

Do not write application code until the plan is coherent. Check the plan
against `project/ROADMAP.md` for scope — roadmap items are not automatically
in scope for the current task.

### Phase 3 — Design

For a major screen, produce before implementation:

1. information architecture;
2. navigation;
3. ASCII wireframe;
4. spatial/layout composition (must follow `docs/DESIGN-SYSTEM.md`);
5. responsive behavior (80×24, 100×30, 120×35, 160×45+);
6. component mapping;
7. state model;
8. interaction model.

### Phase 4 — Implement

- `tui-architecture` to enforce layer boundaries and state ownership;
- `bubbletea-development` for Bubble Tea model lifecycle, composition, and commands;
- `bubbles-development` for selecting, configuring, and styling Bubbles components;
- `lipgloss-design` for visual styling and M-unit spatial grids (`docs/DESIGN-SYSTEM.md`);
- `tui-responsive-layout` for mode transformations (80×24, 100×30, 120×35, 160×45+);
- `tui-input-navigation` for the 12 core keys, focus traps, and palette integration;
- `tui-async-operations` for background execution, progress streaming, and cancellation;
- `fedora-system-integration` for safe host-system adapters.

Reuse existing components. Preserve the architecture boundary
(UI → service → domain → infrastructure).

### Phase 5 — Verify

- formatting (`gofmt`) and vetting (`go vet ./...`);
- `tui-testing` for unit tests, model state transitions, commands, and layout bounds;
- service tests with fakes (never mutate the developer's real system);
- relevant integration tests in controlled environments;
- `tui-visual-review` for auditing the 10 visual inspection gates;
- terminal-size checks at 80×24 / 100×30 / 120×35 / 160×45+;
- keyboard path (mouse path where exposed);
- safety/failure-path checks (permission denied, cancellation, network failure).

### Phase 6 — Self-review (mandatory for non-trivial work)

Use `self-review` on your own diff before declaring completion. Fix any
Critical or High finding first.

### Phase 7 — Review

Use `code-review` for the final diff audit — required before merge/PR and
recommended for any large change. Review the actual diff, not your memory
of it.

### Phase 8 — Document

Update the appropriate docs (architecture, design system, security,
troubleshooting) and changelog/release notes. If you made a meaningful
architectural or product decision, record it in `project/DECISIONS.md`.

## When to use which skill

| Task | Skill |
|---|---|
| New feature spanning layers | `feature-planning` |
| New screen, UX flow, or redesign | `ui-ux-design` |
| Bubble Tea implementation or model work | `bubbletea-development` |
| Visual styling, layout, panels, themes | `lipgloss-design` |
| DNF/systemd/filesystem/downloads/repos | `fedora-system-integration` |
| Your own change, before declaring done | `self-review` |
| Diff/PR audit (yours or someone else's) | `code-review` |

## Explicit skill invocation

Antigravity skills can be invoked by slash-command names when available:

```text
/feature-planning
/ui-ux-design
/bubbletea-development
/lipgloss-design
/fedora-system-integration
/self-review
/code-review
```

Use automatic discovery for normal work; explicit invocation is useful when a
particular workflow must be enforced.

## Never claim unverified behavior

If a system-level behavior could not be tested in the available environment,
say so. Use precise language:

- "Implemented and unit tested."
- "Implemented; Fedora integration not run."
- "UI manually checked at 120×35."
- "Not verified on Fedora 42."

Do not replace uncertainty with confidence.
