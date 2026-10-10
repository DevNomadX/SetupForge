# SetupForge

> **A terminal control center for safely configuring a freshly installed Linux system.**
> Written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## 1. Overview

SetupForge is an open-source, Fedora-first Linux post-installation control center designed to provide a calm, responsive, and keyboard-first terminal workbench. It bridges the gap between raw shell scripts and graphical control panels by offering structured, reviewable, and safety-gated system management directly in the terminal.

### Core Principles
- **Keyboard-First & Mouse-Friendly**: 12 core keys, intuitive navigation, and optional mouse cell support.
- **Strict Architectural Boundary**: Presentation models never execute operating system commands directly (`UI → Application Services → Domain Contracts → Infrastructure Adapters`).
- **Cryptographic & Structural Safety**: Enforces `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT`. The system update or package bundle reviewed by the user is cryptographically verified prior to elevated execution.
- **Zero Silent Privilege Escalation**: No password handling in the TUI; non-interactive privilege preflight (`sudo -n true`); no arbitrary shell string interpolation.
- **Conservative Recovery**: Power cuts or interrupted transactions fail closed. Uncertain transactions are **never** automatically replayed.
- **Fedora-First**: Engineered specifically for modern Fedora Linux (`dnf5`, `systemd`, RPM 4.20+) without premature multi-distro abstractions.

---

## 2. Project Status & Maturity

> [!WARNING]
> **Development Status: Early Alpha / Developer Preview**
> SetupForge is under active development (Phase 2B). It is **not yet production-ready**. While all non-destructive planning and dry-run tests are verified, the project should be evaluated in non-production environments.

### Implementation Summary

| Milestone | Status | Description |
|---|---|---|
| **Phase 1: Foundation** | **Completed** | Terminal frame, 12-core navigation, responsive layout engine (80×24 to 160×45+), design tokens (`Forge`, `Rosé Pine`, `Gruvbox Dark`), host detection. |
| **Phase 2A: Update Engine** | **Completed** | Unprivileged DNF5 check-upgrade polling, structured JSON parsing, dynamic header status badges. |
| **Phase 2B.1–2B.3: Stored Planning** | **Completed** | Unprivileged DNF5 transaction planning (`--store`), Merkle root SHA-256 sealing, single-engine kernel lock (`flock(2)` on `engine.lock`), crash-consistent atomic journaling (`active-transaction.json`). |
| **Phase 2B.4 Slice 1: Privileged Boundary** | **Completed** | Check B exact artifact revalidation, non-interactive sudo authorization, process group containment (`Setpgid: true`), bounded signal escalation (`SIGINT` → `SIGTERM` → `SIGKILL`). |
| **Phase 2B.4 Slice 2A: Evidence & History** | **Completed** | Unprivileged DNF5 SQLite transaction history inspection (`/usr/lib/sysimage/libdnf5/transaction_history.sqlite`), OperationID correlation (`setupforge:<opID>`), and corroborating consistency validation. |
| **Phase 2B.4 Slice 2B: Reconciliation Engine** | *In Progress* | Automatic outcome reconciliation (`RECOVERY_REQUIRED → COMPLETED / FAILED / MANUAL_RECOVERY`) and truthful stdout progress streaming. |
| **Phase 3: Package Management** | *Planned* | DNF package catalog search, inspection, batch selection, and installation/removal workflows. |
| **Phase 4: Applications & Toolchains** | *Planned* | Flatpak application manager, font previews, and development environment bundles. |
| **Phase 5: Polish & History** | *Planned* | System modification audit log, configuration settings, and global command palette (`Ctrl+K`). |

---

## 3. Requirements

### Verified Host Environment
- **Operating System**: Fedora Linux 44 Workstation (or recent Fedora Workstation releases with `dnf5`).
- **Package Manager**: DNF5 (`/usr/bin/dnf5` version 5.4+).
- **Go Toolchain**: Go `1.26+` (as declared in `go.mod`; verified on Go `1.26.8`).
- **Terminal Emulator**: Any ANSI/xterm-compatible terminal supporting minimum dimensions of **80×24** (recommended: **100×30** or wider).

---

## 4. Building & Testing

### Build from Source
```bash
# Compile the standalone setupforge binary to bin/
go build -o bin/setupforge ./cmd/setupforge

# Run the compiled binary
./bin/setupforge
```

### Safe Test Execution
SetupForge maintains strict test isolation. Unit tests execute using in-memory fakes and temporary files. Ordinary tests **never** alter the host package database or mutate `/etc`:

```bash
# Run all unit tests with data race detection
go test -race -count=1 ./...

# Verify code formatting and static analysis
gofmt -l .
go vet ./...
```

---

## 5. Architectural Overview

SetupForge strictly enforces inward-pointing dependencies to isolate the terminal interface from host operating system commands:

```text
┌────────────────────────────────────────────────────────┐
│             Presentation Layer (TUI)                   │
│   Bubble Tea Models, Bubbles Widgets, Lip Gloss        │
└──────────────────────────┬─────────────────────────────┘
                           │ Dispatches tea.Cmd
                           ▼
┌────────────────────────────────────────────────────────┐
│               Application Service Layer                │
│   Use Case Orchestration, Safety Policies, Evidence    │
└──────────────────────────┬─────────────────────────────┘
                           │ Invokes capability interfaces
                           ▼
┌────────────────────────────────────────────────────────┐
│               Domain Layer (Core & Models)             │
│   Interfaces: UpdatePlanner, ArtifactStore, Journal    │
│   Entities: TransactionArtifact, ProcessResult, Plan   │
└──────────────────────────▲─────────────────────────────┘
                           │ Implemented by
┌──────────────────────────┴─────────────────────────────┐
│             Infrastructure / Platform Layer            │
│   SudoRunner, DNF5 History Reader, Local Store, flock  │
└──────────────────────────┬─────────────────────────────┘
                           │ Structured argument execution
                           ▼
┌────────────────────────────────────────────────────────┐
│                  Fedora Linux System                   │
└────────────────────────────────────────────────────────┘
```

---

## 6. System Safety Commitments

1. **Non-Interactive Privilege Verification**: Elevated commands run strictly via non-interactive sudo (`sudo -n`). Plaintext root passwords are never requested, stored, or accepted in application memory.
2. **Discrete Argument Vectors**: Commands are constructed using typed parameter slices (`exec.CommandContext`), completely avoiding shell string concatenation (`sh -c`) and command injection vulnerabilities.
3. **Immutability of Reviewed Artifacts**: Transaction replay uses sealed, pre-downloaded package bundles without relaxation flags (`--ignore-installed` and `--skip-broken` are prohibited).
4. **Crash-Consistent Journaling**: All state transitions are flushed to disk before privileged subprocesses spawn, ensuring complete forensic traceability if power is cut.

---

## 7. Documentation Index

- **Contributor Guidelines**: [`CONTRIBUTING.md`](CONTRIBUTING.md) (and detailed architectural reference in [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md))
- **Security Policy & Reporting**: [`SECURITY.md`](SECURITY.md)
- **Security Threat Model**: [`docs/SECURITY.md`](docs/SECURITY.md)
- **Architecture Rules & System Design**: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- **Design Tokens & Spatial System**: [`docs/DESIGN-SYSTEM.md`](docs/DESIGN-SYSTEM.md)
- **UI/UX Design Specification**: [`docs/UI-UX-DESIGN.md`](docs/UI-UX-DESIGN.md)
- **Architectural Decision Records**: [`project/DECISIONS.md`](project/DECISIONS.md)
- **Detailed Roadmap Checklist**: [`project/ROADMAP.md`](project/ROADMAP.md)
- **Agent Pair-Programming Contract**: [`AGENTS.md`](AGENTS.md)

---

## 8. License

SetupForge is open-source software licensed under the **[Apache License 2.0](LICENSE)**.

Copyright 2026 SetupForge contributors.
