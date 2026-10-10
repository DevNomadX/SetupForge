# Contributing to SetupForge

Welcome to SetupForge! We welcome contributions to make SetupForge a calm, reliable, and safe post-installation terminal control center for Fedora Linux.

---

## 1. Safety & Architecture Principles

SetupForge modifies real operating systems. All contributions must respect these non-negotiable invariants:

1. **Strict Layering Boundary**: Presentation (TUI) code must **never** invoke system commands, package managers, or `sudo` directly:
   ```text
   Presentation (UI / Bubble Tea)
         ↓
   Application Services / Use Cases
         ↓
   Domain Contracts & Invariants
         ↓
   Infrastructure Adapters (DNF5, systemd, sudo)
   ```
2. **System Safety**:
   - Zero silent privilege escalation: non-interactive `sudo -n` preflight only; never solicit passwords in the TUI.
   - Discrete argument vectors: strictly use typed arguments (`exec.CommandContext`); never concatenate shell strings (`sh -c`).
   - Immutability of reviewed operations: `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT`.
   - Never allow ordinary tests to alter the developer's live system, package database, or `/etc`.
3. **Terminal First**:
   - Keyboard-first interaction model with 12 core keys.
   - Responsive layouts accommodating dimensions from **80×24** minimum to **160×45+**.

---

## 2. Getting Started

### Prerequisites
- **Operating System**: Fedora Linux 44 Workstation (or modern Fedora Workstation with `dnf5` version 5.4+).
- **Go Toolchain**: Go `1.26+` (as declared in [`go.mod`](go.mod); verified on Go 1.26.8).
- **Terminal**: ANSI/xterm-compatible terminal supporting minimum dimensions of 80×24.

### Building from Source
```bash
# Compile the standalone binary to bin/
go build -o bin/setupforge ./cmd/setupforge

# Run SetupForge locally
./bin/setupforge
```

---

## 3. Branch Strategy & Contributor Workflow

SetupForge uses a two-tier branch model:

- **`develop`**: The primary integration branch for ongoing active development. All feature branches, bugfix branches, and refactors must branch from and target `develop`.
- **`main`**: The stable, reviewed, release-ready branch. Changes are merged into `main` from `develop` via reviewed release pull requests.

```text
feature/foo ──┐
              ▼
develop ────────────► [Integration & Testing] ──────► main [Release-Ready]
              ▲
bugfix/bar ───┘
```

### Branch Naming
Create branches from `develop` using clear prefixes:
- `feat/<short-description>` — new capabilities
- `fix/<short-description>` — bug fixes
- `refactor/<short-description>` — code restructuring without behavior changes
- `test/<short-description>` — test additions or improvements
- `docs/<short-description>` — documentation updates
- `chore/<short-description>` — maintenance, CI, or tooling tasks

### Commit Message Convention
We follow concise, intention-revealing commit messages:
```text
<type>: <short-description>
```
Examples:
- `feat: add package search service`
- `fix: handle cancelled dnf transaction`
- `refactor: isolate Fedora package adapter`
- `test: cover package validation`
- `docs: document system operation safety`

Keep commits logically focused. Do not mix unrelated refactorings or cosmetic cleanups into feature commits.

### Pull Request Targets
- Target **`develop`** for all standard contributions, features, and fixes.
- Pull requests targeting **`main`** are reserved for release promotions from `develop`.

---

## 4. Verification Before Submitting

Run all local verification checks before creating a pull request:

```bash
# 1. Verify code formatting (must produce zero unformatted files)
gofmt -l .

# 2. Run static analysis
go vet ./...

# 3. Run isolated test suite with race detection
go test -v -race -count=1 ./...

# 4. Confirm clean executable build
go build -o bin/setupforge ./cmd/setupforge
```

For UI changes:
- Verify layout rendering across key dimensions: `80×24`, `100×30`, and `120×35`.
- Check all interaction states: empty, loading, success, error, and confirmation.

> [!NOTE]
> **CI Scope & Limitations**:
> Automated GitHub Actions CI checks formatting, static analysis (`go vet`), in-memory unit tests, and build compilation in an unprivileged Linux environment.
> A passing CI run confirms code logic and interface contracts, but **CI alone does not prove live Fedora privileged transaction safety or crash recovery**. Operations involving live host DNF5 databases or elevated execution must be tested in dedicated local Fedora environments.

---

## 5. Pull Request Guidelines

When opening a Pull Request, please provide:
1. **Summary**: What problem is being solved and why.
2. **Architecture Impact**: Which layers (`internal/ui`, `app`, `domain`, `infra`) are touched.
3. **Safety Impact**: Whether elevated privileges, command arguments, or filesystem mutations are involved.
4. **Testing**: Output or summary of unit tests and integration tests run.
5. **Manual Verification**: Fedora environment, terminal emulator, and dimensions tested.
6. **Known Limitations**: Any edge cases or pending follow-ups.

---

## 6. Reporting Issues

- **Bug Reports**: Open an issue describing the observed behavior, expected behavior, Fedora release, Go version, terminal dimensions, and reproduction steps.
- **Feature Requests**: Describe the problem or workflow SetupForge should improve, why it belongs in post-installation setup, and safety implications.
- **Security Vulnerabilities**: **Do not file public issues with exploit details.** Follow the disclosure instructions in [`SECURITY.md`](SECURITY.md).

---

## 7. Deep Architecture References

For comprehensive technical specifications:
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — Architecture rules and dependency direction
- [`docs/SECURITY.md`](docs/SECURITY.md) — Security threat model and privilege controls
- [`docs/UI-UX-DESIGN.md`](docs/UI-UX-DESIGN.md) — Screen contracts, flows, and navigation
- [`docs/DESIGN-SYSTEM.md`](docs/DESIGN-SYSTEM.md) — Lip Gloss tokens and spatial layout rules
- [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) — Extended architectural contributor reference
- [`AGENTS.md`](AGENTS.md) — Pair-programming contract for AI coding assistants
