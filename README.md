# SetupForge

> **A terminal control center for safely configuring a freshly installed Linux system.**
> Written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## 1. Overview & Problem Statement

Setting up a newly installed Linux system typically involves run-away shell scripts, scattered command-line utilities, copy-pasting commands from online tutorials, and disjointed post-installation configuration steps. Users often lack visibility into what commands are being executed, whether dependencies are valid, and what changes are being applied with administrative privileges.

**SetupForge** solves this problem by providing a unified, calm, keyboard-first terminal control center. It combines the speed and accessibility of the terminal with the structure, clarity, and safety of a curated control panel:

- **Structured System Management**: Review configuration tasks, updates, and software packages through clear terminal views before any changes occur.
- **Cryptographic & Operational Safety**: Transactions are staged, verified, and sealed before privilege elevation. SetupForge enforces the invariant: `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT`.
- **Keyboard-First Ergonomics**: Designed for efficient terminal navigation with 12 core keys and full support for responsive terminal dimensions.
- **Fedora-First Engineering**: Specifically tailored for modern Fedora Linux (`dnf5`, `systemd`, RPM) without premature multi-distribution abstractions.

---

## 2. Project Status & Maturity

> [!WARNING]
> **Development Status: Early Developer Preview / Alpha**
> SetupForge is under active development (Phase 2B). It is **not yet production-ready**.
> System modification operations, package planning, and privileged execution interact with live host system components. While extensive architectural safeguards are in place, users should evaluate SetupForge in virtual machines or non-critical test environments.

### Capability Breakdown

| Capability Area | Status | Description |
|---|---|---|
| **Terminal Frame & Layout** | **Implemented** | Bubble Tea application shell, responsive geometry engine (80×24 to 160×45+), Lip Gloss token system (`Forge`, `Rosé Pine`, `Gruvbox Dark`), host detection (`/etc/os-release`). |
| **Keyboard Navigation** | **Implemented** | 12-core navigation model (arrow keys, Tab, Enter, Esc, spacebar, numeric shortcuts). |
| **DNF5 Check-Upgrade** | **Implemented** | Non-privileged DNF5 check-upgrade polling, structured JSON parsing, dynamic header status badges. |
| **Transaction Storing & Sealing** | **Implemented** | Unprivileged DNF5 planning (`dnf5 upgrade --store`), Merkle root SHA-256 sealing (`fingerprint.go`), single-engine advisory lock (`flock(2)` on `engine.lock`), atomic crash-consistent journaling (`active-transaction.json`). |
| **Privileged Execution Boundary** | **Implemented** | Check B exact artifact revalidation, non-interactive sudo verification (`sudo -n`), process group containment (`Setpgid: true`), bounded signal escalation (`SIGINT` → `SIGTERM` → `SIGKILL`). |
| **Evidence & History Correlation** | **Implemented** | Unprivileged DNF5 SQLite transaction history inspection (`/usr/lib/sysimage/libdnf5/transaction_history.sqlite`), OperationID correlation (`setupforge:<opID>`), and corroborating consistency validation. |
| **Transaction Reconciliation** | *In Progress* | Automatic outcome reconciliation (`RECOVERY_REQUIRED → COMPLETED / FAILED / MANUAL_RECOVERY`) and truthful stdout progress streaming. |
| **Package Management** | *Planned (Phase 3)* | DNF package catalog search, inspection, batch selection, and installation/removal workflows. |
| **Applications & Toolchains** | *Planned (Phase 4)* | Flatpak application manager, font previews, and development environment bundles. |
| **Audit Log & Command Palette** | *Planned (Phase 5)* | System modification audit log, configuration settings, and global command palette (`Ctrl+K`). |

---

## 3. Architecture & Component Structure

SetupForge enforces strict inward-pointing dependencies to decouple the terminal presentation layer from operating system commands:

```text
┌────────────────────────────────────────────────────────┐
│             Presentation Layer (internal/ui)           │
│   Bubble Tea Models, Bubbles Widgets, Lip Gloss Themes │
└──────────────────────────┬─────────────────────────────┘
                           │ Dispatches tea.Cmd
                           ▼
┌────────────────────────────────────────────────────────┐
│         Application Service Layer (internal/app)       │
│   SystemService, UpdateService, Orchestration & Policy │
└──────────────────────────┬─────────────────────────────┘
                           │ Invokes capability interfaces
                           ▼
┌────────────────────────────────────────────────────────┐
│           Domain Layer (internal/domain)               │
│   Interfaces: UpdatePlanner, ArtifactStore, Journal    │
│   Entities: TransactionArtifact, ProcessResult, Plan   │
└──────────────────────────▲─────────────────────────────┘
                           │ Implemented by
┌──────────────────────────┴─────────────────────────────┐
│       Infrastructure / Platform (internal/infra)       │
│   SudoRunner, DNF5 History Reader, Local Store, flock  │
└──────────────────────────┬─────────────────────────────┘
                           │ Structured argument execution
                           ▼
┌────────────────────────────────────────────────────────┐
│                  Fedora Linux System                   │
│         /usr/bin/dnf5, systemd, libdnf5 SQLite         │
└────────────────────────────────────────────────────────┘
```

### Major Packages
- **[`cmd/setupforge`](cmd/setupforge/)**: Main application entry point and TUI initialization.
- **[`internal/ui`](internal/ui/)**: Bubble Tea presentation models, responsive layout engine, components (Dock, Header, Rail, Pane), and theme definitions.
- **[`internal/app`](internal/app/)**: Application use-case services (`SystemService`, `UpdateService`) that orchestrate workflows without knowing Bubble Tea details.
- **[`internal/domain`](internal/domain/)**: Stable domain models (`TransactionArtifact`, `UpdatePlan`, `ProcessResult`), state machines (`TransactionState`), and capability interfaces.
- **[`internal/infra`](internal/infra/)**: Platform-specific adapters:
  - `detector`: Fedora distribution and version detection.
  - `journal`: Atomic, crash-consistent transaction journaling and advisory `engine.lock`.
  - `privilege`: Non-interactive `sudo -n` runner, process isolation, and execution timeouts.
  - `storage`: Filesystem transaction staging in `/var/tmp/setupforge-$UID`.
  - `update`: DNF5 planner, Merkle SHA-256 fingerprint engine, and SQLite transaction history reader.

---

## 4. System Safety Commitments

1. **Non-Interactive Privilege Verification**: Elevated commands run strictly via non-interactive sudo (`sudo -n`). Plaintext root passwords are never requested, stored, or accepted in application memory.
2. **Discrete Argument Vectors**: Commands are constructed using typed parameter slices (`exec.CommandContext`), completely avoiding shell string concatenation (`sh -c`) and command injection vulnerabilities.
3. **Immutability of Reviewed Artifacts**: Transaction replay uses sealed, pre-downloaded package bundles without relaxation flags (`--ignore-installed` and `--skip-broken` are prohibited).
4. **Crash-Consistent Journaling**: All state transitions are flushed to disk before privileged subprocesses spawn, ensuring complete forensic traceability if power is cut.
5. **Conservative Recovery**: Interrupted or power-cut transactions fail closed. Uncertain transactions are **never** automatically replayed.

---

## 5. Requirements & Prerequisites

### Verified Host Environment
- **Operating System**: Fedora Linux 44 Workstation (or recent Fedora Workstation releases with `dnf5`).
- **Package Manager**: DNF5 (`/usr/bin/dnf5` version 5.4+).
- **Go Toolchain**: Go `1.26+` (as declared in [`go.mod`](go.mod); verified on Go 1.26.8).
- **Terminal Emulator**: Any ANSI/xterm-compatible terminal supporting minimum dimensions of **80×24** (recommended: **100×30** or wider).

---

## 6. Building, Testing & Running

### Build from Source
```bash
# Compile the standalone binary into bin/
go build -o bin/setupforge ./cmd/setupforge

# Run the compiled binary
./bin/setupforge
```

### Run Tests and Static Analysis
SetupForge maintains strict test isolation. Unit tests execute using in-memory fakes and temporary directories. Ordinary tests **never** alter the host package database or mutate `/etc`:

```bash
# Run unit and race tests
go test -v -race -count=1 ./...

# Verify code formatting
gofmt -l .

# Run static analysis
go vet ./...
```

---

## 7. Branch Strategy & Development Workflow

SetupForge follows a structured two-tier branch model:

- **`develop`**: The primary integration branch for ongoing development. All feature branches and pull requests target `develop`.
- **`main`**: The stable, release-ready branch. Releases and tagged milestones are promoted into `main` from `develop` via reviewed release pull requests.

```text
feature/foo ──┐
              ▼
develop ────────────► [Integration & CI Testing] ──────► main [Release-Ready]
              ▲
bugfix/bar ───┘
```

### Contributor Guidelines
- Create feature and fix branches off **`develop`** using descriptive prefixes (`feat/`, `fix/`, `docs/`, `refactor/`, `chore/`).
- Follow conventional commit messages: `<type>: <short-description>`.
- Open pull requests targeting **`develop`**. Pull requests targeting **`main`** are reserved for release promotions.
- For complete contributor details, see [`CONTRIBUTING.md`](CONTRIBUTING.md).

---

## 8. Documentation Index

- **Contributor Guidelines**: [`CONTRIBUTING.md`](CONTRIBUTING.md) (and extended architectural reference in [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md))
- **Security Policy & Reporting**: [`SECURITY.md`](SECURITY.md)
- **Security Threat Model**: [`docs/SECURITY.md`](docs/SECURITY.md)
- **Architecture Rules & System Design**: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- **Design Tokens & Spatial System**: [`docs/DESIGN-SYSTEM.md`](docs/DESIGN-SYSTEM.md)
- **UI/UX Design Specification**: [`docs/UI-UX-DESIGN.md`](docs/UI-UX-DESIGN.md)
- **Architectural Decision Records**: [`project/DECISIONS.md`](project/DECISIONS.md)
- **Detailed Roadmap Checklist**: [`project/ROADMAP.md`](project/ROADMAP.md)
- **Agent Pair-Programming Contract**: [`AGENTS.md`](AGENTS.md)

---

## 9. License

SetupForge is open-source software licensed under the **[Apache License 2.0](LICENSE)**.

Copyright 2026 SetupForge contributors.
