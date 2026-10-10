# SetupForge Agent Instructions

Antigravity-compatible workspace customization for SetupForge.

## The four layers

| Layer | Location | Answers | Loaded |
|---|---|---|---|
| Contract | `AGENTS.md` (repo root) | What is always true? Project identity, boundaries, invariants. | Always |
| Rules | `.agents/rules/*.md` | What must be true for this kind of work? Scoped constraints. | By trigger (`always_on`, glob `**/*.go`, model decision) |
| Skills | `.agents/skills/*/SKILL.md` | How do I perform this kind of work? Procedural workflows. | When the task matches the skill |
| Reference | `docs/*.md` | What is the system / design / plan? Human-readable knowledge. | On demand, per `AGENTS.md` §11 |

**One rule per concern.** If two files say the same thing, the more specific /
newer one wins and the other becomes a pointer. Do not fork instructions.

- Rules answer: "What must always be true?"
- Skills answer: "How should the agent perform this work?"
- Docs answer: "What is the system?"

Avoid moving detailed procedures into always-on rules unless they are genuine
project-wide invariants.

## Skills

- `feature-planning` — plan a feature before implementing.
- `ui-ux-design` — design a screen/flow before implementing (information architecture, wireframes).
- `tui-architecture` — enforce presentation/service/domain/infrastructure boundaries and state ownership.
- `bubbletea-development` — implement/review Bubble Tea models, lifecycle, composition, and commands.
- `bubbles-development` — select, configure, and customize Bubbles components (tables, lists, viewports, text inputs).
- `lipgloss-design` — apply the design system to styles, spatial grids (M-unit), and border signatures.
- `tui-responsive-layout` — implement and verify layout transformations across 80×24, 100×30, 120×35, 160×45+.
- `tui-input-navigation` — implement keyboard-first navigation, the 12 core keys, focus traps, and palette.
- `tui-async-operations` — manage async system operations, progress streaming, cancellation, and Activity Sheet.
- `tui-testing` — test models, state transitions, commands, and layout bounds without host mutation.
- `tui-visual-review` — perform visual QA across the 10 inspection gates and design system tokens.
- `fedora-system-integration` — implement host-system integration safely.
- `code-review` — review a diff/PR.
- `self-review` — audit your own non-trivial change before declaring done.

## Reference docs (`docs/`)

- `ARCHITECTURE.md` — what the system is (layers, key types, data flow).
- `DESIGN-SYSTEM.md` — the only design-token source of truth.
- `UI-UX-DESIGN.md` — UX philosophy: information architecture, navigation,
  spatial design, states, accessibility, responsive behavior.
- `AGENT-WORKFLOW.md` — which skill fires when (the procedural map).
- `SECURITY.md` — threat model and safety rationale.
- `CONTRIBUTING.md` — contributor guidelines and safety contracts.
- `PACKAGE-UI.md`, `PACKAGE-AGENT-RULES.md` — package catalog UI and agent rules.

## Project state (`project/`)

- `STATUS.md` — current implementation phase, working features, verification state.
- `ROADMAP.md` — phase-by-phase implementation plan and checklist.
- `DECISIONS.md` — canonical Architecture Decision Records (ADR-001 through ADR-004).
- `CURRENT-WORK.md` — active task log, scope, and verification details.
