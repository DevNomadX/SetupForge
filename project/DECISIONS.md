# SetupForge — Architecture Decision Records (ADRs)

This document is the canonical record of foundational architectural decisions made for SetupForge.

---

## ADR-001: Fedora-First Platform Strategy

- **Status**: Accepted
- **Context**: SetupForge aims to configure Linux systems. Linux distributions diverge significantly in package managers (DNF, APT, Pacman, Zypper), init systems, repository formats, and default tooling. Attempting to support all distributions simultaneously from day one leads to bloated abstractions, premature generalization, and shallow functionality.
- **Decision**: SetupForge will target Fedora Linux exclusively in its initial release. Platform capabilities will be defined as capability interfaces (`PackageManager`, `ServiceManager`, `OSDetector`), but only Fedora adapters will be implemented initially.
- **Consequences**:
  - Fedora users receive a deep, robust, polished post-install experience.
  - No speculative stubs or fake adapters for Debian/Arch.
  - Adding future distributions in later phases requires implementing new platform adapters without touching UI presentation code.

---

## ADR-002: Strict Isolation Between UI and System Commands

- **Status**: Accepted
- **Context**: Terminal UIs that directly invoke `sudo`, `exec.Command("dnf", ...)`, or bash scripts inside view models become impossible to unit-test, introduce severe security risks (unvalidated input interpolation), and suffer from UI freezing during blocking calls.
- **Decision**: Bubble Tea UI models are strictly prohibited from executing operating system commands or mutating filesystem state. All interactions with the host system must pass through Application Services and Domain capability interfaces via asynchronous `tea.Cmd` closures.
- **Consequences**:
  - The UI layer can be 100% unit-tested in pure memory without root privileges or a real terminal.
  - Background operations stream progress events over channels and never freeze the Bubble Tea event loop.
  - System safety invariants (explicit confirmation, validation, cancellation) are strictly enforced at the service boundary.

---

## ADR-003: Pure Elm Architecture Lifecycle

- **Status**: Accepted
- **Context**: Complex interactive TUIs risk race conditions and unpredictable visual glitches if background threads mutate view state directly or if views perform side effects.
- **Decision**: Bubble Tea's Elm Architecture (`Init`, `Update`, `View`) is strictly enforced:
  - `Update` is a pure state machine responding to typed `tea.Msg` events.
  - Goroutines never modify model fields directly; they communicate solely by returning `tea.Msg` through `tea.Cmd`.
  - `View` is a pure function of current state with zero side effects.
- **Consequences**:
  - Deterministic state transitions and effortless unit testing.
  - Zero concurrency data races.

---

## ADR-004: Centralized Token-Only Design System

- **Status**: Accepted
- **Context**: Hardcoding hex color values, arbitrary borders, or custom padding across individual screens leads to visual incoherence, broken light-terminal rendering, and inability to support themes.
- **Decision**: Screens consume only semantic tokens defined in `docs/DESIGN-SYSTEM.md` (`color.bg`, `color.surface`, `color.accent`, `color.focus`, `space.sm`, etc.). Alternate themes (Rosé Pine, Gruvbox) are pure token mappings.
- **Consequences**:
  - SetupForge maintains a coherent visual identity ("The Modular Bench").
  - Theme swapping is instantaneous and cannot break layout geometry.
  - Clean fallbacks for TrueColor, 256-color, 16-color, and monochrome (`NO_COLOR`).

---

## ADR-005: Reusable Privileged Operation Boundary

- **Status**: Accepted
- **Context**: SetupForge is a Fedora system-management control center. Core operations (package inspection, updates, repository setup, service management) frequently interact with system resources owned by `root`. On Fedora, DNF5 state tracking (`/usr/lib/sysimage/libdnf5/packages.toml`) is permission-restricted (`0640 root:root`), causing even read-only `dnf5 check-upgrade --json` to fail when executed by an unprivileged user. However, calling `sudo` or privileged tools naively inside a TUI risks severe failure modes: hanging background event loops on hidden `/dev/tty` password prompts, shell injection vulnerabilities from string interpolation, and coupling UI presentation to host privilege implementations.
- **Decision**: Introduce a dedicated infrastructure abstraction (`internal/infra/privilege`) exposing `PrivilegedRunner` with a production `SudoRunner` adapter:
  - **Zero Shell Invocation**: Direct argument vectors executed via `exec.CommandContext`. Shell interpreters (`sh`, `bash`, `sh -c`) are strictly prohibited.
  - **Strict Architectural Layering**: Presentation and application layers consume domain capability interfaces only; they possess zero awareness of `sudo`, `pkexec`, or privilege escalation mechanics.
  - **Non-Interactive Sudo by Default**: Subprocesses pass `-n` to prevent blocking the Bubble Tea event loop or corrupting terminal display if credentials are required. If authorization is absent, `sudo` exits immediately (code 1), mapping cleanly to typed domain errors.
  - **EUID Awareness**: If the process is already elevated (`euid == 0`), `SudoRunner` bypasses `sudo` and invokes the binary directly.
- **Consequences**:
  - Establishes a safe, reusable contract for subsequent privileged operations in future phases.
  - Strictly preserves the 4-layer dependency model (UI → App → Domain ← Infra).

---

## ADR-006: Least Privilege System Inspection with Privileged Fallback

- **Status**: Accepted
- **Context**: In Phase 2A runtime diagnostics, executing read-only `dnf5 check-upgrade` through `sudo -n` failed with `sudo: a password is required` on standard Fedora installations where the invoking user lacks passwordless sudo or an active TTY credential cache. Furthermore, `.agents/rules/system-safety.md` mandates that system inspection should remain unprivileged whenever possible. Investigation of libdnf5 revealed that the RPM database (`/var/lib/rpm`) and repository metadata (`/var/cache/libdnf5`) are world-readable; unprivileged `dnf5 check-upgrade` failed only because libdnf5 defaulted to loading transactional state from `system_state_dir` (`/usr/lib/sysimage/libdnf5/packages.toml`, mode `0640`).
- **Decision**: Implement least-privilege system inspection in `DNF5UpdateChecker`:
  - When running unprivileged (`euid != 0`), DNF5 queries execute unprivileged with `--setopt=system_state_dir=/var/empty` (falling back defensively to `os.TempDir()` if missing).
  - When running as root (`euid == 0`), DNF5 queries execute directly without modification.
  - If unprivileged execution fails, `DNF5UpdateChecker` falls back to `PrivilegedRunner` in case privileged credentials are available.
- **Consequences**:
  - Background TUI checks never prompt for passwords, never fail due to sudo credential timeouts, and never pollute root audit logs.
  - SetupForge displays live, accurate update availability immediately upon startup.
  - `PrivilegedRunner` remains available for genuinely privileged state-mutating operations in future phases.

---

## ADR-007: Strict Transaction Replay Invariant (Reviewed == Executed)

- **Status**: Accepted
- **Context**: In Phase 2B planning, an initial draft proposed generating an update plan via `dnf5 upgrade --store=DIR` for user review, but then executing `dnf5 upgrade -y`. This created a critical safety vulnerability: network changes, mirror desynchronization, or upstream repository updates between review and execution could result in packages or dependencies being installed that were never reviewed or confirmed by the user. Furthermore, DNF5 provides replay options (`--ignore-installed`, `--ignore-extras`, `--skip-unavailable`, `--skip-broken`) that weaken exactness by ignoring package mismatches, dropping conflicting packages, or resulting in empty transactions.
- **Decision**: SetupForge strictly mandates the `Reviewed == Executed` invariant:
  - Planning executes unprivileged `dnf5 upgrade --store=DIR -y --setopt=system_state_dir=/var/empty`, storing `transaction.json` and downloading RPMs into `packages/`.
  - The presentation model (`UpdatePlan`) is constructed directly from `transaction.json`.
  - User confirmation binds cryptographically to the exact artifact digest.
  - Execution strictly replays the confirmed artifact using `sudo -n /usr/bin/dnf5 -y replay <artifact_dir>`.
  - Replay relaxation flags (`--ignore-installed`, `--ignore-extras`, `--skip-unavailable`, `--skip-broken`) are strictly prohibited.
  - Zero silent re-resolution: SetupForge will never discard a confirmed artifact to re-resolve or run a fresh `dnf5 upgrade`. If replay fails, execution terminates safely with an explicit error.
- **Consequences**:
  - SetupForge guarantees that the exact packages, versions, and scripts reviewed by the user are the only ones executed on the system.
  - If any system package drifts between review and execution, `dnf5 replay` safely aborts without mutating the system.

---

## ADR-008: Deterministic Multi-Component Transaction Artifact Fingerprinting

- **Status**: Accepted
- **Context**: A transaction artifact produced by `dnf5 upgrade --store=DIR` contains multiple components: the metadata file (`transaction.json`) and the actual binary payload (`packages/*.rpm`, comps metadata). Hashing only `transaction.json` leaves the binary RPM payloads vulnerable to post-review tampering or corruption. Furthermore, standard directory traversal (`filepath.WalkDir`) enumerates files in arbitrary filesystem order (`readdir`), which could cause identical artifact bundles to produce divergent digests.
- **Decision**: Define a canonical, deterministic Merkle-style streaming SHA-256 digest algorithm for `TransactionArtifact` (Case A: All Regular Files):
  - Recursively discover all regular files inside the artifact directory via `IsReplayRelevantArtifactFile(d fs.DirEntry) bool { return d.Type().IsRegular() }`.
  - Non-regular filesystem objects (symlinks, sockets, FIFOs, character/block devices) are excluded.
  - Zero filename-based heuristic exclusions: Post-implementation audit of DNF5 5.4.6 verified that DNF5 `--store` creates strictly regular files (`transaction.json` mode `0640`, `packages/*.rpm` mode `0640`) and never writes `.lock` or `.tmp` files into the store directory. Speculative exclusions for `.lock` or `.tmp` were rejected (Case A selected over Case B/C) because ignoring unexpected regular files creates a security vulnerability where unreviewed or tampered content could escape validation prior to `dnf5 replay`.
  - Normalize all relative file paths using forward slashes (`/`).
  - Sort the relative file path list in strict lexicographical (byte-order) sequence.
  - Stream each file into a single SHA-256 hasher framed with length-prefixed path and size headers:
    `PATH:<len(path)>:<path>\nSIZE:<sizeBytes>\n<content_bytes>\nEND\n`.
  - Compute a 64-character lowercase hexadecimal digest.
  - Validate the artifact against strict rules: missing `transaction.json`, missing RPM files, bad permissions, or any digest drift immediately marks the artifact corrupted and prevents execution.
- **Consequences**:
  - 100% deterministic digest independent of filesystem enumeration order.
  - Low memory footprint by streaming multi-gigabyte RPM payloads in 64 KiB chunks rather than buffering into RAM.
  - Prevents length-extension, boundary collision, and binary-substitution tampering.
  - Strict adherence to `Reviewed == Executed`: any regular file added or modified in the artifact directory, regardless of filename or extension, invalidates the digest and halts execution.

---

## ADR-009: Update Plan Presentation & Cryptographic User Confirmation Boundary

- **Status**: Accepted
- **Context**: In Phase 2B.2, the user reviews the resolved `UpdatePlan`. Previous system tools often conflate user review and confirmation with immediate privilege escalation and execution. In SetupForge, user confirmation must strictly record the user's explicit intent without performing any privilege escalation, command execution, or package mutation. Furthermore, confirmation must be bound cryptographically to the exact reviewed transaction artifact (`OperationID`, `ArtifactDirectory`, `ArtifactFingerprint`) to satisfy the `Reviewed == Executed` security invariant.
- **Decision**:
  - Implement `UpdateReviewModel` following the pure Elm architecture (`ReviewState`: `Idle`, `Loading`, `Ready`, `Confirmed`, `Cancelled`, `Error`, `Empty`).
  - Introduce explicit interactive controls (`ControlTable`, `ControlCancelButton`, `ControlConfirmButton`).
  - Safety Invariant: `Enter` on the table navigates but NEVER confirms. Destructive/mutating actions are never preselected. Confirmation requires explicit user navigation via `Tab` to `[ Confirm Update ]` followed by `Enter`.
  - Introduce `domain.ConfirmationDecision` (`Status`, `OperationID`, `ArtifactDirectory`, `ArtifactFingerprint`, `DecidedAt`).
  - Strict Boundary: Confirmation records the `ConfirmationDecision` and halts. Zero execution, zero `sudo`, zero `dnf5 replay`, zero package mutation. Phase 2B.3 (`AuthorizationBoundary`) remains strictly separate and not started.
- **Consequences**:
  - Complete separation between user decision recording and privileged execution.
  - Fully testable in memory across responsive terminal sizes (80×24 to 200×50).
  - Clear user visibility with responsive Bubbles table layout and prominent warnings for high-impact updates (kernel, systemd).

---

## ADR-010: Lean Transaction Engine, ArtifactStore & Crash Recovery

- **Status**: Accepted
- **Context**: In Phase 2B.2 testing, storing transaction bundles in `$XDG_RUNTIME_DIR` (`/run/user/$UID`) caused tmpfs exhaustion (~380 MiB ceiling on 4 GiB RAM machines). Additionally, process crashes destroyed in-memory transaction states, leading to storage leakage and inability to audit or recover interrupted transactions. Multiple concurrent SetupForge processes risked split-brain transaction states.
- **Decision**: Adopt the Lean Transaction Engine architecture:
  - **Persistent Workspace**: Relocate transaction bundles to `/var/tmp/setupforge-$UID/transactions/` backed by root filesystem persistent storage and protected by private `0700` user permissions.
  - **ArtifactStore Abstraction**: Isolate physical filesystem concerns behind `domain.ArtifactStore` with concrete adapter `LocalFilesystemStore`.
  - **Artifact Physical Lifecycle**: Implement explicit 4-state lifecycle (`STAGING` $\rightarrow$ `SEALED` $\rightarrow$ `RECLAIMABLE` $\rightarrow$ `PURGED`).
  - **Symlink & Non-Regular Object Hardening**: Perform explicit `os.Lstat` scans during sealing and validation, immediately rejecting symlinks, FIFOs, sockets, and character/block devices (`ErrTransactionArtifactCorrupted`).
  - **Capacity Preflight**: Query filesystem statistics via `unix.Statfs` as an admission check before planning.
  - **Crash-Consistent Atomic State Journal**: Persist active transaction state to `/var/tmp/setupforge-$UID/journal/active-transaction.json` using crash-consistent atomic publication (`f.Sync` $\rightarrow$ `os.Rename` $\rightarrow$ `parentDir.Sync`). SQLite and multi-file history journals are explicitly rejected/deferred.
  - **Advisory Kernel Lock**: Enforce single active engine ownership per Linux user session via `flock(2)` on `/var/tmp/setupforge-$UID/engine.lock`.
  - **Conservative Crash Recovery**: If an engine discovers state `EXECUTING` on startup, transition to `RECOVERY_REQUIRED` $\rightarrow$ `INTERRUPTED` unless conclusive execution evidence proves success. Raw RPM DB checks are rejected as proof of completion, and uncertain transactions are **never** automatically replayed.
  - **Preserved SHA-256 Digest**: Retain hardware-accelerated standard library SHA-256 Merkle root hashing. BLAKE3 and CAS are rejected as unjustified dependencies.
- **Consequences**:
  - Full crash resilience and auditability without adding any external dependencies (zero CGo, pure Go).
  - Eliminates tmpfs exhaustion and RAM starvation during multi-hundred-megabyte package updates.
  - Preserves the `REVIEWED == EXECUTED` security invariant across process restarts and system crashes.

---

## ADR-011: Transaction Engine Security & Recovery Hardening

- **Status**: Accepted
- **Context**: Prior to freezing Phase 2B.3, a comprehensive security and recovery audit was performed against the implementation. The audit identified edge cases in inode aliasing, journal write-overwrites, illegal state skips, and orphan artifact GC risks:
  1. Regular files with multiple hardlinks (`Nlink > 1`) could theoretically allow external mutation of package payloads after sealing.
  2. Overwriting a corrupted journal on disk could destroy forensics and mask an interrupted or compromised execution.
  3. Lack of explicit transition validation could allow illegal jumps (e.g., `PLANNED -> EXECUTING` bypassing user confirmation, or `INTERRUPTED -> EXECUTING` automatic replay).
  4. Garbage collection could inadvertently purge valid sealed artifacts if an engine crashed between sealing and journal commitment.
- **Decision**:
  - **Inode Hardlink Verification**: In `LocalFilesystemStore.assertNoSymlinksOrSpecialObjects`, inspect `info.Sys().(*syscall.Stat_t).Nlink` and reject any regular file with `Nlink > 1` with `ErrTransactionArtifactCorrupted`.
  - **Strict State Transition Matrix**: Introduce domain-level `ValidateTransition(from, to TransactionState)` enforced both in domain models and in `FileTransactionJournal.WriteActive`. Specifically disallow `PLANNED -> EXECUTING`, `REVIEWED -> EXECUTING`, `CONFIRMED -> COMPLETED`, and all re-executions from `FAILED`, `INTERRUPTED`, or `RECOVERY_REQUIRED`.
  - **Fail-Closed Corrupt Journal Policy**: `ReadActive` wraps all parse and structural corruption into `ErrCorruptedJournal`. `WriteActive` inspects disk state and refuses to overwrite corrupted journals, raising `ErrRecoveryRequired`.
  - **Orphan Sealed Artifact Protection**: `CollectGarbage` checks `IsSealedArtifact(dir)` before removing unreferenced directories, preserving intact sealed bundles for manual review. If an unreferenced sealed artifact is found during startup reconciliation, `RecoverOrReconcile` returns `ErrRecoveryRequired`.
  - **Confirmation Integrity Assertion**: `ConfirmActiveTransaction` asserts exact equality of `OperationID`, `ArtifactFingerprint`, and `ArtifactDirectory` against the active journal.
- **Consequences**:
  - Eliminates post-validation payload mutation via filesystem aliasing.
  - Guarantees fail-closed behavior on state corruption, preventing silent data loss or premature re-planning.
  - Formally prevents automatic or accidental replay of interrupted system updates.

---

## ADR-012: Privileged Transaction Execution, Authorization Boundary & Crash Recovery

- **Status**: Accepted / Architecture Gate Validated
- **Context**: Executing system-level updates requires root privileges. SetupForge must elevate privileges safely without prompting for or storing passwords in the TUI, preserve the `REVIEWED == EXECUTED` security invariant against TOCTOU tampering, contain child processes safely, stream truthful progress without machine-readable JSON replay output, eliminate stale-history false positives, and handle the non-atomic nature of RPM updates with conservative crash recovery.
- **Decision**:
  - **Privileged Execution Mechanism**: Execute via non-interactive sudo: `/usr/bin/sudo -n /usr/bin/dnf5 --comment="setupforge:<operation_id>" -y replay <artifact_directory>`. Shell execution is forbidden. Replay relaxation flags (`--ignore-installed`, `--skip-broken`) are forbidden.
  - **Authorization Boundary**: Sudo preflight (`sudo -n true`) verifies existing credentials. The UI never prompts for or stores passwords. `AUTHORIZED` is an instantaneous pre-execution gate, not a persistent cross-reboot credential.
  - **Pre-Execution Revalidation Gate (Check B)**: Immediately prior to process spawn, `UpdateService` re-scans the sealed artifact directory with `Lstat` (0 symlinks, `Nlink == 1`) and recalculates the full streaming SHA-256 Merkle root digest against `journal.ArtifactFingerprint`. Captures baseline ID, baseline start timestamp, and baseline RPMDB version.
  - **Process Containment & Signal Escalation**: Processes run in an isolated process group (`Setpgid: true`), with sanitized `PATH`, stripped `LD_*`, locale `C.UTF-8`, working directory `/`, and closed `stdin` (`/dev/null`). Signal cancellation targets `-pgid` with bounded escalation (`SIGINT` 30s $\rightarrow$ `SIGTERM` 10s $\rightarrow$ `SIGKILL`). Signal handling does not establish success/failure; outcome is governed strictly by DNF5 transaction evidence.
  - **Authoritative Correlation Semantics**: Primary correlation identity is `OperationID ↔ comment marker` (`h.comment = "setupforge:" + opID`), supported by corroborating consistency checks $(h.\text{id} > \text{base\_id}) \land (h.\text{dt} \ge T_{\text{start}}) \land (h.\text{rpmdb\_begin} = \text{base\_rpmdb})$ to eliminate stale false positives without claiming mathematical infallibility.
  - **Authoritative Outcome Proof**: `ProvenSuccess` requires exit code 0 AND correlated history record with `state_id = 2` ('Ok') and valid end timestamp. `ProvenFailure` requires clean exit code 1 with 0 packages modified OR `state_id = 3` ('Error'). Querying the RPM database alone is rejected as proof of completion.
  - **Corrected Recovery State Machine & Legacy Compatibility**: `RECOVERY_REQUIRED` is an active reconciliation state that evaluates authoritative evidence. If evidence proves success, it promotes to `COMPLETED`; if clean failure, to `FAILED`; if inconclusive, to `MANUAL_RECOVERY`. `MANUAL_RECOVERY` is terminal for automation (zero automatic replay); presents manual command `sudo dnf5 history redo <id>`. While in `MANUAL_RECOVERY` or `INTERRUPTED`, the engine blocks new planning (`PlanUpdate` fails closed with `ErrRecoveryRequired`) to protect forensic evidence and prevent planning against an unverified host state. New transitions into legacy `INTERRUPTED` are rejected, while pre-existing Phase 2B.3 journals with `INTERRUPTED` remain read-compatible and terminal without in-place mutation.
  - **Truthful Progress**: Streaming stdout regex parser extracts determinate `[i/N]` step counters. Active indeterminate spinner is rendered during scriptlets and preflight. No synthetic percentages.
  - **Journal Durability**: `EXECUTING` is flushed to disk before `exec.Cmd.Start()` is invoked.
- **Consequences**:
  - Production-grade execution safety on Fedora with zero external CGo dependencies.
  - Strict adherence to `REVIEWED == EXECUTED`.
  - Completely non-interactive and testable with mock runners.
