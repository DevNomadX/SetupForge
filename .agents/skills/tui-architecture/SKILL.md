---
name: tui-architecture
description: Enforces the mandatory architectural layering, boundary isolation, state ownership, and service contracts between Bubble Tea TUI and the Fedora system.
---

# TUI Architecture & Layering Skill

Use this skill when designing, implementing, or reviewing the boundaries between Bubble Tea presentation code, application services, domain models, and Linux/Fedora system operations in SetupForge.

This skill is the authoritative implementation guide for [`.agents/rules/architecture.md`](../../rules/architecture.md) and [`AGENTS.md`](../../../AGENTS.md) §2.

---

## 1. The Core Dependency Graph

SetupForge enforces strict inward-pointing dependencies:

```text
┌────────────────────────────────────────────────────────┐
│               Bubble Tea / Presentation               │
│         (Models, Views, Messages, Focus, Input)        │
└──────────────────────────┬─────────────────────────────┘
                           │ calls use cases
                           ▼
┌────────────────────────────────────────────────────────┐
│                  Application Services                  │
│   (Orchestration, Confirmation Policy, Progress, Err)  │
└──────────────────────────┬─────────────────────────────┘
                           │ depends on
                           ▼
┌────────────────────────────────────────────────────────┐
│               Domain Interfaces & Models               │
│   (PackageManager, ServiceManager, Package, Result)    │
└──────────────────────────▲─────────────────────────────┘
                           │ implemented by
┌──────────────────────────┴─────────────────────────────┐
│                 Infrastructure Layer                   │
│       (DNF Runner, Systemd Adapter, OS Detector)       │
└──────────────────────────┬─────────────────────────────┘
                           │ executes safe commands
                           ▼
┌────────────────────────────────────────────────────────┐
│                   Fedora Host System                   │
└────────────────────────────────────────────────────────┘
```

> [!CAUTION]
> **Non-Negotiable Architecture Invariant**:
> Never allow UI code to execute operating-system commands directly.
> A Bubble Tea model or view must NEVER import `os/exec`, run `sudo`, execute `dnf`, query `systemctl`, parse `/etc/os-release`, or mutate files on disk.

---

## 2. Layer Responsibilities & Constraints

### 2.1 Presentation Layer (Bubble Tea, Bubbles, Lip Gloss)

**Responsible for:**
- Bubble Tea models (`tea.Model`), lifecycle (`Init`, `Update`, `View`), and message routing.
- Terminal focus management, key maps, and mouse clicks.
- Rendering layout using Lip Gloss and Design System tokens (`docs/DESIGN-SYSTEM.md`).
- Presenting user dialogs, toasts, activity sheets, and confirmation prompts.
- Translating user actions into Application Service invocations via `tea.Cmd`.

**Forbidden from:**
- Invoking `exec.Command` or shell scripts.
- Checking system privileges (`os.Getuid()`).
- Parsing raw CLI command output for business rules.
- Writing to `/etc`, `~/.config`, or system directories.
- Owning package-manager policy (e.g. deciding whether a reboot is required).

### 2.2 Application Service Layer (Use Cases)

**Responsible for:**
- Orchestrating use cases (e.g. "InstallSelectedPackages", "RefreshRepositories", "EnableWorkstationServices").
- Enforcing confirmation policies (e.g. flagging destructive or privileged operations).
- Managing transaction lifecycles and step-by-step preparation (Ready → Confirm → Prepare → Execute → Result).
- Propagating `context.Context` for cancellation and timeouts.
- Emitting structured progress events to channels consumed by Bubble Tea.
- Mapping low-level infrastructure failures into user-friendly application errors (the 4-question error format).

**Must remain independent of:**
- Bubble Tea types (`tea.Model`, `tea.Msg`, `tea.Cmd`).
- Lip Gloss styles.
- Terminal geometry or rendering concepts.

### 2.3 Domain Layer (Interfaces & Stable Entities)

**Responsible for:**
- Core business entities: `Package`, `Repository`, `ServiceUnit`, `SystemInfo`, `TransactionPlan`.
- Capability-based domain interfaces (e.g., `PackageManager`, `ServiceManager`, `RepositoryManager`, `OSDetector`).
- Typed domain errors: `ErrPackageNotFound`, `ErrTransactionConflict`, `ErrPrivilegeDenied`.
- Domain validation rules (validating package names, repo URLs, service unit names).

**Design Rule**: Keep domain interfaces minimal and capability-focused. Do not create a monolithic `Distribution` interface.

### 2.4 Infrastructure Layer (Adapters & Platform Integration)

**Responsible for:**
- Executing external binaries using structured arguments (`exec.CommandContext("dnf5", "install", "-y", pkgName)`).
- Capturing and inspecting `stdout`, `stderr`, and exit codes.
- Interacting with DNF, RPM, Flatpak, and systemd.
- Reading host release info from `/etc/os-release`.
- Implementing domain capability interfaces for Fedora.

---

## 3. Concrete Code Boundary Pattern

### Anti-Pattern: Direct System Call in UI Model (FORBIDDEN)
```go
// VIOLATION: Directly executing DNF in a Bubble Tea Update handler
func (m PackageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
        // TERRIBLE: Freezes the UI event loop, bypasses confirmation, un-testable
        cmd := exec.Command("sudo", "dnf", "install", "-y", m.selectedPkg)
        cmd.Run()
    }
    return m, nil
}
```

### Compliant Pattern: Decoupled via Application Service & Command

```go
// 1. Domain Interface (domain/package.go)
type PackageManager interface {
    Install(ctx context.Context, pkgs []string, progress chan<- ProgressStep) error
}

// 2. Application Service (app/package_service.go)
type PackageService struct {
    pkgMgr domain.PackageManager
}

func (s *PackageService) InstallPackages(ctx context.Context, pkgs []string) (<-chan ProgressStep, <-chan error) {
    progressCh := make(chan ProgressStep, 10)
    errCh := make(chan error, 1)

    go func() {
        defer close(progressCh)
        defer close(errCh)
        err := s.pkgMgr.Install(ctx, pkgs, progressCh)
        errCh <- err
    }()

    return progressCh, errCh
}

// 3. Bubble Tea Command (ui/package_page/commands.go)
func installPackagesCmd(svc *app.PackageService, pkgs []string) tea.Cmd {
    return func() tea.Msg {
        progressCh, errCh := svc.InstallPackages(context.Background(), pkgs)
        // Bubble Tea listens to channels and emits typed messages
        return OperationStartedMsg{ProgressChan: progressCh, ErrChan: errCh}
    }
}
```

---

## 4. State Ownership Rules

Each piece of mutable state must have exactly **one clear owner**:

1. **Presentation State** (Cursor index, active tab, pane focus, scroll offset, search filter text, open dialog state) is owned exclusively by the Bubble Tea model.
2. **System State** (Installed packages, running services, system configuration) is owned by Application Services and Infrastructure caches.
3. **No State Duplication**: Do not mirror mutable lists between parent models, child models, and application services. Child models receive read-only slices or indices from the parent.
4. **Single Source of Truth**: When an operation mutates system state, the Application Service finishes the transaction, and the UI re-queries or applies a typed result message to update its view.

---

## 5. Fedora-First, Not Fedora-Everywhere

SetupForge's initial target is **Fedora Linux**.

- Build concrete, robust, production-quality Fedora adapters first (`dnf5`, `systemd`, `flatpak`).
- **Avoid Abstraction Theater**: Do not create fake or stub implementations for Ubuntu, Debian, Arch, or openSUSE merely to show that an interface is extensible.
- When an interface is defined (e.g. `PackageManager`), implement `FedoraPackageManager`. Additional distribution implementations will be added in later roadmap phases without altering UI code.

---

## 6. Testing & Mocks Boundary

Because all system interactions reside behind domain interfaces, the entire UI and Application layer can be tested with 100% determinism:

```go
// Tests use in-memory fake implementations of domain interfaces
type FakePackageManager struct {
    InstalledPackages []string
    InstallError      error
}

func (f *FakePackageManager) Install(ctx context.Context, pkgs []string, ch chan<- ProgressStep) error {
    if f.InstallError != nil {
        return f.InstallError
    }
    f.InstalledPackages = append(f.InstalledPackages, pkgs...)
    return nil
}
```

- Normal unit and UI tests **never touch the developer's real system**.
- Tests execute instantly and can reliably simulate network failures, transaction conflicts, and cancelled operations.

---

## 7. Architectural Review Checklist

When writing or reviewing code, verify:

- [ ] Presentation code has zero imports of `os/exec`, `syscall`, or system files.
- [ ] No `sudo` or shell commands exist in Bubble Tea models, views, or commands.
- [ ] UI models trigger operations by invoking Application Services.
- [ ] Application Services orchestrate use cases and return typed progress/results.
- [ ] Domain interfaces define platform boundaries using minimal capability contracts.
- [ ] Infrastructure adapters isolate Fedora-specific CLI calls using structured argument arrays (`exec.CommandContext`).
- [ ] Presentation state is separate from domain state.
- [ ] Code is testable with in-memory fakes without root permissions.
