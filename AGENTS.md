# SetupForge — AI Development Contract

SetupForge is a serious, open-source, Fedora-first Linux post-installation TUI
written in Go with Bubble Tea, Bubbles, and Lip Gloss.

This file is the project-wide contract for AI coding agents. It is intentionally
concise. It states invariants and points to deeper documents; it does not
duplicate them.

## 1. Mission

Build a polished, GUI-like terminal control center for safely configuring a
freshly installed Linux system.

The product must be:

- keyboard-first and mouse-friendly;
- responsive to terminal size (80×24 minimum usable);
- visually distinctive, coherent, and terminal-safe;
- modular and testable;
- idempotent where practical;
- explicit about privileged or destructive actions;
- Fedora-first without prematurely implementing multi-distro behavior.

Do not implement the entire roadmap at once. Build coherent vertical slices.

## 2. Mandatory architectural boundary

Never let UI code execute operating-system commands directly.

    UI → application/service layer → domain interfaces → infrastructure/platform

UI models may orchestrate messages, state transitions, and presentation, but
must not contain shell commands, `sudo`, DNF invocation, filesystem mutation,
or distro detection. Fedora-specific behavior must be isolated behind
interfaces so future distro support can be added without rewriting UI behavior.

Full layering rules: `.agents/rules/architecture.md`

## 3. System safety (non-negotiable)

SetupForge modifies real Linux systems. All system-mutating behavior is
governed by `.agents/rules/system-safety.md`, which is authoritative. The
invariants in brief:

- no silent privilege escalation;
- explicit, specific confirmation for destructive/irreversible actions;
- structured argument execution, never shell interpolation of untrusted input;
- validation at both UI and service/infrastructure boundaries;
- cancellation, typed errors, dry-run/preflight, and recovery where practical;
- no arbitrary "run any shell command" feature, ever.

## 4. Work before coding

1. Inspect the repository structure and relevant existing code/tests.
2. Search for existing components/services before creating new ones.
3. Identify the smallest coherent change.
4. For non-trivial UI work, design information architecture, UX flow, and an
   ASCII wireframe first (see `.agents/skills/ui-ux-design/SKILL.md`).
5. Identify safety, privilege, cancellation, and failure behavior before
   implementing system-changing features.

Do not invent abstractions merely because they look elegant. Prefer existing
project patterns when they are sound.

## 5. UI contract

UI/UX is a first-class product concern. SetupForge must be both **highly
consistent** (shared spatial system, shared components, shared design tokens)
and **visually distinctive** (intentional decoration, recognizable identity
across themes — Rose Pine and Gruvbox are the theme candidates).

Never copy web/mobile patterns literally into the terminal. Never create
one-off colors, borders, spacing, or status semantics for individual screens.

| Concern | Authoritative source |
|---|---|
| UX flows, navigation, states, accessibility | `docs/UI-UX-DESIGN.md` |
| Tokens, themes, spatial system, decoration | `docs/DESIGN-SYSTEM.md` |
| Always-on TUI constraints | `.agents/rules/ui.md` |
| Lip Gloss implementation | `.agents/skills/lipgloss-design/SKILL.md` |

## 6. AI-agent failure modes (explicitly forbidden)

- Creating new abstractions without checking for existing ones.
- Rewriting large areas of the project unnecessarily.
- Implementing before understanding the architecture.
- Inventing APIs, commands, or package names that were not verified.
- Silently changing system state (including the developer's machine during tests).
- Claiming tests passed without running them, or claiming UI works without
  checking it at the required terminal sizes.
- Mixing unrelated refactors into feature work.
- Following stale instructions (`SETUPFORGE-ALL-INSTRUCTIONS.md` or any doc)
  when newer source files disagree — the code and the files listed in §8 win.
- Adding dependencies for trivial problems.
- Building fake multi-distro implementations for architectural completeness.

## 7. Verification

Run the narrowest relevant checks first, then the broader suite: `gofmt`,
`go vet ./...`, project tests, integration tests when applicable. Never claim a
feature works unless verified. Distinguish explicitly: implemented / tested /
manually verified / not verified / blocked by environment. Ordinary tests must
never mutate the developer's real system (see `.agents/rules/testing.md`).

## 8. Self-review (mandatory for non-trivial work)

Before declaring a task complete, run `.agents/skills/self-review/SKILL.md`
against your change. It covers architecture, UI/design-system conformance,
safety, testing, and scope. Skipping it is a contract violation.

## 9. Change discipline

Keep changes focused. Do not reformat unrelated files, rename unrelated APIs,
upgrade dependencies without reason, or hide cleanup inside feature commits.
Explain meaningful architectural decisions in the change summary. Do not commit
unless explicitly asked (`.agents/rules/git.md`).

## 10. Source-of-truth hierarchy

1. Explicit user/task requirements.
2. This contract.
3. `.agents/rules/` scoped constraints.
4. `.agents/skills/` procedural workflows.
5. `docs/` reference documentation.
6. Existing code conventions — unless they violate the above, in which case
   flag the conflict instead of silently following stale code.

When documents conflict, say so; do not pick the convenient interpretation.

## 11. Required references by task type

| Task | Read first |
|---|---|
| Architecture | `.agents/rules/architecture.md`, `.agents/skills/tui-architecture/SKILL.md` |
| UI implementation | `.agents/rules/ui.md`, `.agents/rules/go-bubbletea.md`, `.agents/skills/bubbletea-development/SKILL.md`, `.agents/skills/bubbles-development/SKILL.md`, `docs/UI-UX-DESIGN.md`, `docs/DESIGN-SYSTEM.md` |
| New screen / UX design | `.agents/skills/ui-ux-design/SKILL.md` (+ the UI row above at implementation time) |
| Styling & layout | `.agents/skills/lipgloss-design/SKILL.md`, `docs/DESIGN-SYSTEM.md` |
| Responsive layout | `.agents/skills/tui-responsive-layout/SKILL.md`, `docs/UI-UX-DESIGN.md` §9 |
| Input & navigation | `.agents/skills/tui-input-navigation/SKILL.md`, `docs/UI-UX-DESIGN.md` §5 |
| Async system operations | `.agents/skills/tui-async-operations/SKILL.md`, `.agents/rules/system-safety.md` |
| Fedora/system work | `.agents/rules/system-safety.md`, `.agents/skills/fedora-system-integration/SKILL.md` |
| Feature planning | `.agents/skills/feature-planning/SKILL.md` |
| Testing | `.agents/rules/testing.md`, `.agents/skills/tui-testing/SKILL.md` |
| Visual QA & review | `.agents/skills/tui-visual-review/SKILL.md`, `docs/DESIGN-SYSTEM.md` §11 |
| Review (own or others') | `.agents/skills/code-review/SKILL.md`, `.agents/skills/self-review/SKILL.md` |
| Contributing/Git | `.agents/rules/git.md`, `docs/CONTRIBUTING.md` |

References are guidance, never permission to bypass §2, §3, or §7.
