# SetupForge — System Architecture

> Version 1.0 · Companion to [`docs/DESIGN-SYSTEM.md`](./DESIGN-SYSTEM.md) and [`docs/UI-UX-DESIGN.md`](./UI-UX-DESIGN.md).
> Authoritative enforceable rules: [`.agents/rules/architecture.md`](../.agents/rules/architecture.md).
> Implementation skill: [`.agents/skills/tui-architecture/SKILL.md`](../.agents/skills/tui-architecture/SKILL.md).

---

## 1. System Mission

SetupForge is a serious, open-source, Fedora-first Linux post-installation TUI control center written in Go with Bubble Tea, Bubbles, and Lip Gloss.

It configures a freshly installed Linux system through a calm, responsive, keyboard-first terminal workbench.

---

## 2. Core Architectural Layers

SetupForge strictly enforces inward-pointing dependencies. Presentation code never interacts directly with low-level operating system binaries or files.

```text
┌────────────────────────────────────────────────────────┐
│             Presentation Layer (TUI)                   │
│   Bubble Tea Models, Bubbles Widgets, Lip Gloss        │
└──────────────────────────┬─────────────────────────────┘
                           │ Dispatches commands (tea.Cmd)
                           ▼
┌────────────────────────────────────────────────────────┐
│               Application Service Layer                │
│   Use Case Orchestration, Safety Policies, Progress    │
└──────────────────────────┬─────────────────────────────┘
                           │ Calls capability contracts
                           ▼
┌────────────────────────────────────────────────────────┐
│               Domain Layer (Core & Models)             │
│   Interfaces: PackageManager, ServiceManager, Detector │
│   Entities: Package, Repository, ServiceUnit, Result   │
└──────────────────────────▲─────────────────────────────┘
                           │ Implemented by
┌──────────────────────────┴─────────────────────────────┐
│             Infrastructure / Platform Layer            │
│   Command Runner, DNF5 Adapter, Systemd, Filesystem    │
└──────────────────────────┬─────────────────────────────┘
                           │ Executes structured commands
                           ▼
┌────────────────────────────────────────────────────────┐
│                  Fedora Host System                    │
└────────────────────────────────────────────────────────┘
```

---

## 3. Layer Responsibilities

### 3.1 Presentation (Bubble Tea)
- Implements `tea.Model` (`Init`, `Update`, `View`).
- Manages local UI state: selection index, cursor, scroll position, filter query, focus order, active dialog.
- Consumes semantic design tokens from `docs/DESIGN-SYSTEM.md`.
- Handles `tea.WindowSizeMsg` and executes responsive layout transformations (`docs/UI-UX-DESIGN.md` §9).
- **Invariant**: Presentation code NEVER imports `os/exec`, never executes `sudo` or `dnf`, and never modifies filesystem files directly.

### 3.2 Application Services
- Orchestrates multi-step transactions (e.g., install packages, enable services, add repositories).
- Evaluates confirmation requirements for destructive or privileged actions.
- Propagates cancellation contexts (`context.Context`) and timeout policies.
- Formats failures into the 4-question error model (What happened, Why, Affected, Next actions).
- Emits progress events over channels to update the UI without blocking the event loop.

### 3.3 Domain
- Defines clean, minimal, capability-based interfaces (`PackageManager`, `ServiceManager`, `OSDetector`).
- Models stable domain entities (`Package`, `Repository`, `ServiceUnit`).
- Enforces domain validation rules (e.g. package naming formats, URL validation).
- Completely decoupled from Bubble Tea, terminal rendering, and OS-specific command syntax.

### 3.4 Infrastructure & Platform Adapters
- Executes system binaries using structured arguments (`exec.CommandContext("dnf5", "install", "-y", pkg)`).
- Captures `stdout`, `stderr`, and exit codes safely without shell interpolation.
- Provides concrete Fedora implementations for DNF5, Flatpak, systemd, and `/etc/os-release`.
- Is testable via in-memory fakes to guarantee that normal test execution never touches the developer's workstation.

---

## 4. Key Architectural Decisions

1. **ADR-001 (Fedora First)**: Focus on concrete, first-class Fedora implementations first. Do not introduce speculative or fake multi-distro abstractions until a real second distribution is targeted.
2. **ADR-002 (UI Cannot Execute Commands)**: All system modifications must go through Application Services and Domain interfaces.
3. **ADR-003 (The Elm Architecture Lifecycle)**: UI updates are pure state transitions driven by typed messages; long-running operations execute asynchronously via `tea.Cmd`.
4. **ADR-004 (Token-Only Styling)**: Screens reference semantic Lip Gloss tokens; raw hex codes and ad-hoc styles are prohibited.
5. **ADR-005 (Reusable Privileged Operation Boundary)**: Safe elevated execution via `PrivilegedRunner` and `SudoRunner` (`sudo -n`, EUID bypass, direct argument vectors).
6. **ADR-006 (Least-Privilege System Inspection)**: Read-only DNF5 inspection runs unprivileged via `--setopt=system_state_dir=/var/empty` with privileged fallback.
7. **ADR-007 (Strict Transaction Replay Invariant)**: The transaction reviewed by the user is the exact transaction executed (`Reviewed == Executed`). Stored transaction artifacts generated via `--store` are replayed strictly via `dnf5 -y replay` without relaxation flags.
8. **ADR-008 (Deterministic Multi-Component Fingerprinting)**: Multi-component artifacts (`transaction.json` + `packages/*.rpm`) are verified with canonical framed streaming SHA-256 digests independent of filesystem enumeration order.

---

## 5. Architectural References

- Layering rules: [`.agents/rules/architecture.md`](../.agents/rules/architecture.md)
- TUI architecture skill: [`.agents/skills/tui-architecture/SKILL.md`](../.agents/skills/tui-architecture/SKILL.md)
- Bubble Tea development: [`.agents/skills/bubbletea-development/SKILL.md`](../.agents/skills/bubbletea-development/SKILL.md)
- System safety rules: [`.agents/rules/system-safety.md`](../.agents/rules/system-safety.md)
- Design tokens & layout: [`docs/DESIGN-SYSTEM.md`](./DESIGN-SYSTEM.md)
- Architectural Decision Records: [`project/DECISIONS.md`](../project/DECISIONS.md)
