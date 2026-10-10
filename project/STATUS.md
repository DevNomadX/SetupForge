# SetupForge — Project Status

## Current Phase
Phase 2B.4 Controlled Vertical Slice 1 Complete (Privilege Boundary, Artifact Revalidation, Durable EXECUTING & Sudo Replay Dispatch) · Ready for Slice 2

## Status Summary
Phase 1 Foundation, Phase 2A System Update Status Engine, Phase 2B.1 Transaction Artifact Model & DNF5 Update Planner, Phase 2B.2 Update Plan Presentation & User Confirmation, Phase 2B.3 Transaction Engine & Artifact Storage Lifecycle, and Phase 2B.4 Controlled Vertical Slice 1 are fully implemented and verified on the real Fedora host. SetupForge safely resolves system updates into isolated, persistent disk transaction artifacts under `/var/tmp/setupforge-$UID/transactions/` using unprivileged DNF5, enforces admission capacity preflight (`statfs`), hardens against symlinks and non-regular objects (`Lstat`), calculates a deterministic multi-component SHA-256 fingerprint, persists authoritative active transaction state to `/var/tmp/setupforge-$UID/journal/active-transaction.json` using crash-consistent atomic publication (`f.Sync` -> `Rename` -> `parentDir.Sync`), enforces single-engine ownership via kernel advisory `flock(2)` on `engine.lock`, presents the reviewable plan in a responsive Bubbles table, records cryptographic confirmation, revalidates the exact reviewed artifact before execution, verifies non-interactive authorization (`sudo -n true`), publishes durable `EXECUTING` state before process invocation, dispatches privileged DNF5 replay (`dnf5 --comment="setupforge:<opID>" -y replay <artifactDir>`) with process group containment and bounded escalation, and captures raw process execution evidence while keeping the transaction in `EXECUTING` state (exit code 0 != COMPLETED).

## Implemented & Working
- **Canonical Project State** (`project/`):
  - `STATUS.md`: Authoritative live project status.
  - `ROADMAP.md`: Phase-by-phase implementation checklist and release strategy.
  - `DECISIONS.md`: Canonical Architecture Decision Records (ADR-001 through ADR-012).
  - `CURRENT-WORK.md`: Current sprint task log and verification notes.
- **Domain Layer** (`internal/domain/`):
  - Pure domain models (`SystemInfo`, `OSRelease`, `UpdateStatus`, `PackageUpdate`, `UpdateState`, `TransactionArtifact`, `UpdatePlan`, `PlannedPackage`, `PackageAction`, `ConfirmationDecision`, `ConfirmationStatus`, `ArtifactState`, `TransactionState`, `Descriptor`, `ActiveTransaction`, `PrivilegedCommand`, `ProcessResult`).
  - Domain capability contracts (`OSDetector`, `UpdateChecker`, `UpdatePlanner`, `ArtifactStore`, `TransactionJournal`, `EngineLock`, `AuthorizationProvider`, `PrivilegedRunner`).
  - Domain typed errors (`ErrUnsupportedDistribution`, `ErrOSReleaseNotFound`, `ErrUpdateCheckFailed`, `ErrDNF5NotFound`, `ErrMalformedUpdateOutput`, `ErrInvalidArtifact`, `ErrIncompleteArtifact`, `ErrUnreadableArtifact`, `ErrTransactionArtifactCorrupted`, `ErrPlanningFailed`, `ErrEmptyTransaction`, `ErrUnknownAction`, `ErrPlanUnavailable`, `ErrStalePlan`, `ErrInsufficientStorage`, `ErrEngineLocked`, `ErrInvalidArtifactState`, `ErrInvalidTransactionState`, `ErrRecoveryRequired`, `ErrInterruptedTransaction`, `ErrAuthorizationFailed`, `ErrArtifactMismatch`).
- **Infrastructure Layer** (`internal/infra/`):
  - `detector`: `FedoraDetector` parsing `/etc/os-release` and kernel release without shell execution.
  - `privilege`: `SudoAuthorizationProvider` providing non-interactive `sudo -n true` preflight and root bypass.
  - `privilege`: `SudoRunner` implementing `ExecuteWithProcessGroup` with process group isolation (`Setpgid: true`), sanitized environment, and bounded signal escalation (`SIGINT` 30s -> `SIGTERM` 10s -> `SIGKILL`).
  - `update`: `BuildDNF5ReplayCommand` constructing discrete argument vector `/usr/bin/dnf5 --comment="setupforge:<opID>" -y replay <artifactDir>` rejecting relaxation flags.
  - `update`: `DNF5UpdateChecker` querying `/usr/bin/dnf5 check-upgrade --json` with least privilege (`--setopt=system_state_dir=/var/empty`) and `PrivilegedRunner` fallback.
  - `update`: Structured JSON parser `ParseDNF5CheckUpgradeJSON` handling up-to-date and upgrade states.
  - `update`: `DNF5UpdatePlanner` executing unprivileged `dnf5 upgrade --store=PATH --setopt=system_state_dir=/var/empty` delegating staging, sealing, and validation to `ArtifactStore`.
  - `storage`: `LocalFilesystemStore` managing physical transaction bundles in `/var/tmp/setupforge-$UID/transactions/` (0700), capacity preflight via `statfs`, explicit symlink rejection via `Lstat`, streaming SHA-256 Merkle root sealing, physical purging, and ownership-aware garbage collection.
  - `journal`: `FileTransactionJournal` managing `/var/tmp/setupforge-$UID/journal/active-transaction.json` using crash-consistent atomic temporary-file publication (`f.Sync` -> `os.Rename` -> `parentDir.Sync`).
  - `journal`: `FlockedEngineLock` acquiring Linux kernel advisory lock (`flock(2)`) on `/var/tmp/setupforge-$UID/engine.lock` ensuring single-engine ownership.
- **Application Service Layer** (`internal/app/`):
  - `SystemService` providing thread-safe cached host inspection and compatibility verification.
  - `UpdateService` orchestrating `UpdateChecker`, `UpdatePlanner`, `ArtifactStore`, `TransactionJournal`, `EngineLock`, `AuthorizationProvider`, and `PrivilegedRunner`.
  - `ExecuteActiveTransaction` enforcing engine lock, confirmed decision validation, `CONFIRMED` state precondition, Check B exact artifact revalidation (`OperationID`, directory, fingerprint, Merkle digest), non-interactive authorization, durable `EXECUTING` state publication prior to execution, and capturing process execution evidence.
- **Presentation Layer** (`internal/ui/`):
  - Design system tokens and themes (`Forge`, `Rosé Pine`, `Gruvbox Dark`).
  - Responsive layout engine with M-unit macro grid (80×24, 100×30, 120×35, 160×45+).
  - Signature single-line pane borders with embedded bracket headers (`┌─ Title ─ count ─ pos ─┐`).
  - Global application frame (1-row Header, 7-destination Rail, 1-row Dock, Stage).
  - Overview workbench page with live Fedora detection, update status reporting, interactive launchpad, and `[u] Review plan` shortcut.
  - Header dynamic update badge (`X updates ●`) on M/L/XL breakpoints.
  - Root `AppModel` handling 12 core keys, focus cycling, Keyboard Help modal overlay, and simultaneous refresh (`r`).
  - Update Review modal overlay (`UpdateReviewModel`) with responsive Bubbles table, package details, kernel/systemd warnings, focus trap, explicit `[ Confirm Update ]` vs `[ Cancel ]` activation, and `ConfirmationDecision` generation.
  - Confirmation records to durable journal; cancellation executes immediate artifact purge and journal clear.
- **Executable Entrypoint** (`cmd/setupforge/main.go`):
  - Compiles to `bin/setupforge`.
  - Wires `LocalFilesystemStore`, `FileTransactionJournal`, `FlockedEngineLock`, `SudoAuthorizationProvider`, and `SudoRunner` into `UpdateService`.
  - Executes startup crash recovery and garbage collection reconciliation.
  - Supports `--version` and `--help` CLI flags.
  - Alternate screen buffer and mouse support.

## In Progress
- Phase 2B.4 Controlled Vertical Slice 1 Complete.
- Advancing to Phase 2B.4 Controlled Vertical Slice 2: DNF5 Transaction History Inspection & Post-Execution Reconciliation.

## Verification Gates (Passing)
- `gofmt -l .`: 0 formatting violations.
- `go vet ./...`: 0 static analysis issues.
- `go test -v -count=1 ./...`: 100% pass across all packages (domain, detector, privilege, update, storage, journal, app, ui, components, layout, theme).
- Visual QA: Verified responsive dimensions (80×24, 100×30, 120×35, 160×45, 180×50, 200×50) with 0 stray borders, 0 line overflows, and full text display.
- Real Fedora Integration (`TestRealFedora_ControlledExecutionSlice1`): Verified on Fedora Linux 44 Workstation (`dnf5 5.4.6.0`). Evaluated full planning -> confirmation -> exact artifact revalidation -> authorization boundary -> durable EXECUTING -> runner dispatch. `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT` verified end-to-end. Zero host packages mutated.
- Binary compilation: `bin/setupforge` recompiled cleanly.

## Blocked
None.

## Known Problems
None.

## Last Verified
2026-10-03
