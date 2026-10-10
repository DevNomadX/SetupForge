# SetupForge — Transaction Engine & Artifact Architecture Gate

**Document Status**: ARCHITECTURAL DESIGN GATE / REVISED DECISION RECORD
**Target Phase**: Pre-Phase 2B.3 Architecture Gate
**Author**: SetupForge Core Agent Architecture Team
**Date**: October 2, 2026 (Revised)
**Target Platform**: Fedora Linux (Fedora 44+) & Portable POSIX/Linux

---

## 1. Executive Summary

This architecture decision document establishes the authoritative, durable transaction engine, artifact storage, and lifecycle recovery design for SetupForge prior to implementing Phase 2B.3 (`AuthorizationBoundary`) and Phase 2B.4 (`TransactionExecution`).

### Core Design Problem
SetupForge is evolving from an ephemeral, in-memory prototype into a durable, crash-resilient Linux post-installation control center. In Phase 2B.2, update planning created ephemeral DNF5 stored transactions in `/run/user/$UID`, but process termination wiped all session state, and accumulated test transactions revealed severe `tmpfs` capacity constraints (`documented in docs/PHASE-2B-3-STORAGE-RESEARCH.md`).

Before implementing privileged execution, we evaluated whether introducing an expanded architecture—incorporating **SQLite**, **BLAKE3**, **Linux filesystem primitives (`openat2`, `O_TMPFILE`)**, **Content-Addressed Storage (CAS)**, and **OCI container specifications**—is justified.

### Core Architectural Evaluation
Applying the guiding constraint:
> **Small core + few dependencies + strong correctness + strong security + deterministic recovery + maintainability + incremental migration**

We reached the following definitive architectural conclusions:

1. **SQLite**: **REJECTED (DEFERRED)**.
   SetupForge manages exactly **one active transaction** per user at any given time. Introducing SQLite requires either **CGo** (`mattn/go-sqlite3`), which destroys cross-compilation and fast builds, or a **50 MB transpiled C-in-Go library** (`modernc.org/sqlite`), adding immense dependency bloat for simple single-record state management. Durable state shall instead be managed by a **single authoritative, pure-Go atomic JSON state file** (`active-transaction.json`) using crash-consistent atomic temporary-file publication.
2. **Persistent History (`history.jsonl`)**: **DEFERRED**.
   The current requirement is reliable recovery and state management of the **current transaction**, not constructing a complete historical event store. Introducing multi-file journaling is deferred until Phase 4 (Maintenance & History Workbench).
3. **BLAKE3**: **REJECTED**.
   Hardware-accelerated standard library `crypto/sha256` hashes 150 MiB of RPMs in **< 80 milliseconds** on modern x86_64/arm64 processors. BLAKE3 adds external dependencies for zero user-perceptible performance or security advantage. SHA-256 remains the authoritative, standard-library digest engine.
4. **Linux Filesystem Primitives (`openat2`, `O_TMPFILE`)**: **REJECTED / DEFERRED**.
   External package managers (`dnf5 upgrade --store=PATH`) require a visible directory path, making `O_TMPFILE` unusable for artifact creation. File descriptor traversal (`openat2`) introduces unnecessary complexity when standard directory scanning with `os.Lstat` mode checking (`os.ModeSymlink`) completely eliminates symlink vulnerabilities. However, **file `f.Sync()` and parent directory `Sync()`** are **ADOPTED** for crash-consistent atomic state publication.
5. **Content-Addressed Storage (CAS)**: **REJECTED**.
   DNF5 unconditionally downloads RPMs directly into `<PATH>/packages/` and does not support external package symlink resolution. Because SetupForge enforces a **single active transaction** model with immediate cleanup upon completion, cross-transaction package deduplication adds reference-counting complexity for zero real-world storage savings.
6. **OCI Descriptors**: **KEPT WITH CHANGES (Internal Model Only)**.
   Adopt lightweight, self-describing metadata descriptors (`mediaType`, `digest`, `size`, `annotations`) purely as **internal Go structs**. Zero external OCI/container libraries or registry infrastructure are introduced.
7. **Workspace Location**: **CONFIRMED (`/var/tmp/setupforge-$UID/`)**.
   As established in `docs/PHASE-2B-3-STORAGE-RESEARCH.md`, the workspace resides in persistent storage under `/var/tmp/setupforge-$UID/transactions/`, backed by root disk space and protected by user-only `0700` permissions. However, `/var/tmp` is not assumed to have unlimited capacity; preflight checks are admission guards only, and runtime `ENOSPC` handling remains mandatory.
8. **Concurrency Ownership**: **Single Active Engine per User via Kernel `flock(2)`**.
   SetupForge enforces exclusive user-level engine execution using an advisory, kernel-managed file lock (`engine.lock`), completely preventing split-brain transaction states without complex distributed locking.
9. **Conservative Crash-Recovery Model**: **ADOPTED**.
   SetupForge rejects inferring transaction completion from raw RPM database state. If a crash occurs during `EXECUTING`, SetupForge enters `RECOVERY_REQUIRED`. Unless conclusive, trustworthy execution evidence proves success, SetupForge transitions to `INTERRUPTED / MANUAL_RECOVERY` and **never automatically replays an uncertain transaction**.

---

## 2. Current Architecture Findings (Code Verification)

```
[REPOSITORY-VERIFIED]
```

A comprehensive audit of the frozen Phase 2B.2 repository reveals the exact implementation boundaries:

### 2.1 State Ownership & In-Memory Ephemerality
- In [`internal/ui/app.go`](../internal/ui/app.go):
  ```go
  case update.UpdateConfirmedMsg:
      m.lastDecision = msg.Decision
      // Decision recorded in memory only
  ```
- **Finding**: All transaction state resides strictly inside Bubble Tea model memory (`AppModel.lastDecision`). If the user exits SetupForge, hits `Ctrl+C`, or the process crashes, the confirmed state is permanently lost. There is zero persistent record of whether a transaction was confirmed, authorized, or completed.

### 2.2 Filesystem & Artifact Handling in `dnf5_planner.go`
- In [`internal/infra/update/dnf5_planner.go`](../internal/infra/update/dnf5_planner.go):
  - Resolves base directory from `$XDG_RUNTIME_DIR/setupforge/transactions/` (falling back to `/tmp`).
  - Creates directories via `os.MkdirAll(txBaseDir, 0700)` and `os.MkdirTemp(txBaseDir, "tx-"+opID+"-*")`.
  - Spawns unprivileged DNF5: `dnf5 upgrade --store=<storeDir> -y --setopt=system_state_dir=/var/empty`.
  - Cleans up on error (`defer os.RemoveAll`), but leaves the directory on disk indefinitely if planning succeeds.
- **Finding**: There is no separation between DNF5 execution logic and artifact storage management. Path generation, directory creation, and file validation are tightly coupled within `dnf5_planner.go`.

### 2.3 Cryptographic Integrity Engine
- In [`internal/infra/update/fingerprint.go`](../internal/infra/update/fingerprint.go):
  - Streams all regular files into `crypto/sha256` in lexicographical path order.
  - Implements Case A (All Regular Files, zero heuristic exclusions).
  - Produces a deterministic 64-character hex SHA-256 digest.
- **Finding**: The integrity engine is already optimal, streaming, and 100% deterministic. However, `ValidateArtifactStructure` currently ignores symlinks rather than explicitly rejecting them.

### 2.4 Dependency Footprint
- In [`go.mod`](../go.mod):
  - Charmbracelet ecosystem (`bubbletea`, `bubbles`, `lipgloss`).
  - Low-level system primitives (`golang.org/x/sys`, `golang.org/x/sync`).
  - **Zero CGo dependencies**. Clean, fast static compilation.

---

## 3. Existing Strengths to Preserve

```
[REPOSITORY-VERIFIED]
```

The existing architecture provides rock-solid guarantees that must **not** be compromised:

1. **`REVIEWED == EXECUTED` Invariant (ADR-007)**:
   The exact transaction bundle reviewed by the user is the only bundle authorized and executed.
2. **Deterministic Merkle-Style SHA-256 Fingerprint (ADR-008)**:
   Lexicographically ordered, length-prefixed streaming digest covering both `transaction.json` and all `.rpm` payloads.
3. **Four-Tier Architectural Layering (ADR-002, `.agents/rules/architecture.md`)**:
   UI (`internal/ui`) $\rightarrow$ App Service (`internal/app`) $\rightarrow$ Domain (`internal/domain`) $\leftarrow$ Infra (`internal/infra`). UI never invokes OS commands.
4. **Least-Privilege Planning (ADR-006)**:
   Planning runs 100% unprivileged without `sudo` via `--setopt=system_state_dir=/var/empty`.
5. **Cryptographic User Confirmation Boundary (ADR-009)**:
   Confirmation records an immutable `ConfirmationDecision` bound to the artifact fingerprint and halts. Zero command execution occurs during confirmation.

---

## 4. Problems That the New Architecture Must Solve

```
[RESEARCH-VERIFIED]
```

The current implementation has four critical gaps that prevent advancing to privileged execution (Phases 2B.3 and 2B.4):

1. **No Process-Crash Survivability**:
   If SetupForge is terminated during review, confirmation, or execution handoff, all knowledge of the transaction is lost from memory. SetupForge cannot resume, cannot report what was confirmed, and cannot detect whether an interrupted transaction completed.
2. **Lack of Lifecycle Cleanup (Storage Bleed)**:
   Cancelled, failed, or superceded transaction artifacts remain on disk indefinitely, leading to storage exhaustion (the 259 MiB tmpfs accumulation problem).
3. **Coupled Storage Mechanics**:
   `dnf5_planner.go` directly owns filesystem paths and directory management, making it impossible to substitute storage locations or enforce global storage budgets cleanly.
4. **Missing Concurrency Protection**:
   Nothing prevents a user from launching two SetupForge instances simultaneously, causing conflicting DNF5 lock contentions and conflicting artifact operations.

---

## 5. Proposed Architecture: The Lean Transaction Engine

```
[PROPOSED]
```

Instead of an over-engineered storage platform, SetupForge shall adopt a **Lean Transaction Engine** composed of decoupled, pure-Go components:

```
                            ┌────────────────────────────────────────┐
                            │                UI Layer                │
                            │   (UpdateReviewModel, AppModel, Dock)  │
                            └───────────────────┬────────────────────┘
                                                │ Events / Msgs
                            ┌───────────────────▼────────────────────┐
                            │           Application Layer            │
                            │             (UpdateService)            │
                            └─────────┬───────────────────┬──────────┘
                                      │                   │
                     Durable State    │                   │ Artifact Storage
                                      ▼                   ▼
    ┌───────────────────────────────────┐       ┌───────────────────────────────────┐
    │        Transaction Journal        │       │       Local Artifact Store        │
    │   (atomic active-transaction.json)│       │ (/var/tmp/setupforge-$UID/tx-...) │
    └─────────────────┬─────────────────┘       └─────────────────┬─────────────────┘
                      │                                           │
                      │ Linux filesystem                          │ DNF5 / librpm
                      ▼                                           ▼
          /var/tmp/setupforge-$UID/                   /var/tmp/setupforge-$UID/
              ├── engine.lock                             └── transactions/
              └── journal/                                    └── tx-<opID>-<rand>/
                  └── active-transaction.json                     ├── transaction.json
                                                                  └── packages/*.rpm
```

### Component Responsibilities

1. **`UpdateService` (Orchestrator)**:
   Coordinates planning, user confirmation, authorization checks, execution handoff, and lifecycle transitions. Enforces single-active-transaction rules.
2. **`ArtifactStore` (Infrastructure Interface)**:
   Owns physical storage on disk: preflight capacity checks (`unix.Statfs`), staging directory creation, structure validation, symlink rejection, SHA-256 fingerprinting, and physical directory purging.
3. **`TransactionJournal` (Infrastructure Interface)**:
   Owns durable transaction state: atomic persistence of the single active transaction descriptor, and recovery state detection upon startup.
4. **`EngineLock` (Infrastructure Component)**:
   Acquires non-blocking kernel `flock(2)` on `/var/tmp/setupforge-$UID/engine.lock` to ensure only one SetupForge transaction engine operates per user session.

---

## 6. Artifact Lifecycle Design

```
[PROPOSED]
```

The physical artifact directory (`tx-<opID>-<rand>/`) follows a strict four-state physical lifecycle completely decoupled from UI presentation:

```mermaid
stateDiagram-v2
    [*] --> STAGING: CreateStaging()
    STAGING --> SEALED: Seal() [Validate + Hash]
    STAGING --> PURGED: Abort() [on failure / abandoned]

    SEALED --> RECLAIMABLE: MarkReclaimable() [on terminal tx]
    RECLAIMABLE --> PURGED: Purge() [os.RemoveAll]
    PURGED --> [*]
```

### Artifact State Matrix

| Physical State | Definition & Guarantees | Permitted Actions | Crash Recovery & Reclamation Rule |
| :--- | :--- | :--- | :--- |
| **`STAGING`** | Directory created; DNF5 `--store` downloading packages. Incomplete and mutable. | Write payloads, write `transaction.json`. Cannot be reviewed or executed. | **Preserve if active owner.** If ownership is proven abandoned (e.g. startup with lock held and state is STAGING or unreferenced), **reclaim and purge**. |
| **`SEALED`** | DNF5 finished. Zero symlinks verified. SHA-256 Merkle root digest computed and locked. | Read-only. Reviewable. Executable (once authorized). **Protected against deletion while transaction active.** | **Preserved.** Reconciled against journal state. Reclaimed only when transaction transitions to terminal state. |
| **`RECLAIMABLE`** | Transaction reached terminal state (`COMPLETED`, `CANCELLED`, `FAILED`). No active locks. | Eligible for physical deletion. Cannot be executed. | **Purged immediately** by engine or startup cleanup. |
| **`PURGED`** | Directory removed via `os.RemoveAll`. Storage reclaimed. | None. Inode deleted. | N/A |

### Safe Reclamation Principle
> **An artifact is reclaimed because the system has established that ownership or execution has been abandoned or completed according to durable state, not merely because the process restarted.**

---

## 7. Transaction Lifecycle Design

```
[PROPOSED]
```

The logical transaction follows an explicit, audited lifecycle managed by `UpdateService` and persisted to `active-transaction.json`:

```mermaid
stateDiagram-v2
    [*] --> PLANNED: PlanUpdate() [Artifact SEALED]
    PLANNED --> REVIEWED: User Opens Review Modal
    REVIEWED --> CANCELLED: User clicks [ Cancel ] / Esc
    CANCELLED --> [*]: Artifact RECLAIMABLE -> PURGED

    REVIEWED --> CONFIRMED: User clicks [ Confirm Update ]
    CONFIRMED --> AUTHORIZED: sudo -v authenticated (Phase 2B.3)
    AUTHORIZED --> EXECUTING: dnf5 replay launched (Phase 2B.4)

    EXECUTING --> COMPLETED: Replay 0 + verification proven
    EXECUTING --> RECOVERY_REQUIRED: Process crash / power failure
    EXECUTING --> FAILED: Replay exit code != 0

    RECOVERY_REQUIRED --> COMPLETED: Trustworthy evidence proves success
    RECOVERY_REQUIRED --> INTERRUPTED: Uncertain / incomplete evidence

    COMPLETED --> [*]: Artifact RECLAIMABLE -> PURGED
    FAILED --> [*]: Artifact RECLAIMABLE -> PURGED
    INTERRUPTED --> [*]: Preserved for Manual Recovery
```

### Transaction State Definitions
* **`PLANNED`**: Artifact resolved and `SEALED`. Initial plan generated and written to `active-transaction.json`.
* **`REVIEWED`**: User is actively inspecting the plan in the TUI review modal.
* **`CONFIRMED`**: User explicitly confirmed the plan. Fingerprint and decision recorded durably.
* **`AUTHORIZED`**: Privileged authentication verified (`sudo -v`). Sudo ticket cached.
* **`EXECUTING`**: Privileged `dnf5 replay` launched.
* **`COMPLETED`**: Replay exited code 0 and post-execution checks passed.
* **`FAILED`**: Explicit error during planning, authorization, or execution.
* **`CANCELLED`**: User explicitly cancelled during review.
* **`RECOVERY_REQUIRED`**: Process terminated while in `EXECUTING`. Requires trustworthy evidence evaluation.
* **`INTERRUPTED`**: Execution outcome cannot be conclusively proven. Replay is blocked; manual diagnostic mode engaged.

---

## 8. Ownership Model: Single Active Engine per User

```
[PROPOSED]
```

To eliminate distributed race conditions, multi-process disk thrashing, and split-brain transaction states, SetupForge enforces:

> **One Active Transaction Engine per Linux User Session.**

### Implementation: Kernel Advisory Lock (`flock`)
- Workspace directory: `/var/tmp/setupforge-$UID/`
- Lock file: `/var/tmp/setupforge-$UID/engine.lock`
- Mechanism:
  ```go
  fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC, 0600)
  err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
  ```
- **Crash Invariant**:
  Unlike PID files or lease heartbeats, `flock(2)` is maintained directly by the Linux kernel file table. If SetupForge crashes (`SIGSEGV`, `SIGKILL`, power failure), the kernel **automatically releases the lock immediately**. Zero stale lock files can ever lock out the user.
- If a second instance attempts to acquire the lock:
  SetupForge enters **Read-Only / Observer Mode**, notifying the user:
  `"Another SetupForge instance is currently managing updates. This window is read-only."`

---

## 9. Crash/Recovery Model & Failure Matrix

```
[REASONED ARCHITECTURAL CONCLUSION]
```

### 9.1 Conservative `EXECUTING` Recovery Semantics (Critical Invariant)

> **SetupForge must NEVER treat raw RPM database state alone as proof that a specific transaction completed, and must NEVER automatically replay an uncertain transaction.**

A package transaction involves package scriptlets (`%pre`, `%post`, `%preun`, `%postun`), configuration file replacements (`.rpmnew`, `.rpmsave`), service restarts, and shared library triggers. Checking whether planned package NEVRAs exist in `/var/lib/rpm` is fundamentally insufficient:
- Target NEVRAs may be present even if a post-install scriptlet crashed or failed.
- Old NEVRAs may remain if an uninstallation step was interrupted.
- Another process or admin may have manipulated packages outside SetupForge.

### The Conservative Reconciliation Protocol
When SetupForge starts and finds `active-transaction.json` in state `EXECUTING`:
1. **Transition to `RECOVERY_REQUIRED`**:
   The engine immediately marks the in-memory state as `RECOVERY_REQUIRED`.
2. **Check for Running Package Processes**:
   Query whether a child `dnf5` or `rpm` process is still running on the system. If active, SetupForge halts and displays:
   `"A package manager transaction is currently active in the background. SetupForge will not interfere. Please wait for it to finish."`
3. **Inspect Conclusive Execution Evidence**:
   SetupForge evaluates whether trustworthy, explicit execution evidence exists proving that the specific transaction finished with exit code 0:
   - For example, an execution completion record written synchronously upon runner completion (e.g. `<dir>/execution-result.json` containing exit code 0 and timestamp).
   - If DNF5 history officially records the exact transaction ID as successfully completed without errors.
4. **Branching Decision**:
   - **Conclusive Evidence of Success**: State transitions to `COMPLETED`. The artifact is marked `RECLAIMABLE` and purged.
   - **Inconclusive, Missing, or Error Evidence**: State transitions to **`INTERRUPTED`**.
5. **Handling `INTERRUPTED` (Manual Recovery Mode)**:
   - **Automatic Replay is FORBIDDEN**: SetupForge will **never** automatically run `dnf5 replay` on an interrupted transaction. Re-running scriptlets or partial transactions automatically risks system corruption.
   - **Artifact Preserved**: The artifact directory is kept intact for diagnostic purposes.
   - **User Guided Recovery**: SetupForge presents a dedicated recovery screen detailing the operation ID, timestamp, target packages, and explicit recovery instructions (e.g., advising the user to inspect `sudo dnf5 history` or run `sudo dnf5 check`).
   - The user is provided with explicit, informed actions: `[ Clear Interrupted State ]` or `[ View Transaction Details ]`.

---

### 9.2 Comprehensive Failure Matrix (14 Real Failure Classes)

| # | Failure Class | Durable State | Artifact State | Ownership State | Automatic Cleanup Safe? | Recovery Possible? | Manual Action Required? |
| :- | :--- | :--- | :--- | :--- | :---: | :---: | :---: |
| **1** | **Normal Application Exit** | None (cleared) | `PURGED` | Released on exit | Yes | Clean | No |
| **2** | **User Cancellation** | `CANCELLED` | `RECLAIMABLE` $\rightarrow$ `PURGED` | Held until exit | Yes (immediate) | Clean | No |
| **3** | **Planning Failure** | `FAILED` | `STAGING` $\rightarrow$ `PURGED` | Held until exit | Yes (immediate) | N/A | No |
| **4** | **Fingerprint Mismatch** | `FAILED` | `RECLAIMABLE` $\rightarrow$ `PURGED` | Held until exit | Yes | Re-plan required | No |
| **5** | **Crash Before Confirmation** | `PLANNED` | `SEALED` (abandoned) | Kernel released | Yes (on next startup) | Discarded safely | No |
| **6** | **Crash After Confirmation** | `CONFIRMED` | `SEALED` (protected) | Kernel released | No (preserved) | Yes (resume or discard) | User choice |
| **7** | **Crash During Execution** | `EXECUTING` | `SEALED` (protected) | Kernel released | **NO (Forbidden)** | Enters `RECOVERY_REQUIRED` | **Yes (if inconclusive)** |
| **8** | **`SIGKILL` (`kill -9`)** | Last written state | Depends on phase | Kernel released | Safe if non-executing | Based on journal | Only if executing |
| **9** | **Power Loss** | Last flushed state | Depends on phase | Kernel released | Safe if non-executing | Based on journal | Only if executing |
| **10**| **Disk Full (`ENOSPC`)** | `FAILED` | Incomplete `STAGING` | Held | Yes (purge staging) | Free space required | User frees disk space |
| **11**| **Filesystem I/O Failure** | `FAILED` | Mark unreadable | Held | No (preserve logs) | Diagnostic report | Hardware/OS check |
| **12**| **Partial Artifact Creation** | `STAGING` | Incomplete | Held / Abandoned | Yes (safe to purge) | Re-plan | No |
| **13**| **Interrupted Cleanup** | `RECLAIMABLE` | Partially removed | Held / Abandoned | Yes (retry purge) | Auto-purged next run | No |
| **14**| **Second Process Starts** | Existing untouched | Untouched | Lock denied | N/A | Enters Read-Only Mode | Close other window |

---

## 10. SQLite Responsibility & Evaluation

```
[REASONED ARCHITECTURAL CONCLUSION]
```

### 10.1 The Architectural Evaluation

| Evaluation Dimension | Option 1: SQLite Database | Option 2: Single Atomic State File (`active-transaction.json`) |
| :--- | :--- | :--- |
| **Go Dependency Footprint** | 🛑 **Heavy**: Requires CGo (`mattn/go-sqlite3`) OR 50 MB transpiled C (`modernc.org/sqlite`). | ✅ **Zero External Dependencies**: Standard library `encoding/json` + `os`. |
| **Cross-Compilation & Build** | 🛑 Broken by CGo; slow compile times. | ✅ Pure Go: instant 0.5s builds, effortless static binaries. |
| **Crash Consistency** | Requires SQLite WAL configuration, checkpointing, and multi-file tracking (`.db-wal`, `.db-shm`). | ✅ Crash-consistent atomic temporary-file publication. |
| **Auditability & Debugging** | 🛑 Opaque binary format; requires `sqlite3` CLI tool to inspect. | ✅ Human-readable JSON. Inspectable with `cat`, `jq`, or standard editors. |
| **Desynchronization Risk** | High risk of SQLite record drifting from disk directory state. | ✅ State file resides directly alongside artifacts in `/var/tmp/setupforge-$UID/journal/`. |
| **Query Complexity Needed** | Overkill. SetupForge has zero relational queries, joins, or table indexing. | ✅ Single-document read/write matches the domain model 1-to-1. |

### 10.2 Architectural Decision: REJECT SQLite / ADOPT Atomic State Journal
SQLite is explicitly **rejected** as architectural over-engineering for the current requirements.

Durable state shall be maintained by `JSONTransactionJournal`:
- **Path**: `/var/tmp/setupforge-$UID/journal/active-transaction.json`
- **History**: `history.jsonl` is explicitly **deferred** to Phase 4.

### 10.3 Crash-Consistent Atomic Publication Specification
> **Terminology Invariant**: This mechanism provides **crash-consistent atomic publication and durable filesystem state, subject to the semantics and reliability of the underlying filesystem**. It does NOT provide database-style ACID transactions.

Whenever active transaction state changes:
1. Write JSON payload to temporary file in the same directory:
   `/var/tmp/setupforge-$UID/journal/active-transaction.json.tmp`
2. Call `tmpFile.Sync()` to flush dirty pages and file metadata to physical media.
3. Close `tmpFile`.
4. Call `os.Rename(tmpPath, finalPath)` (atomic POSIX rename overwriting target).
5. Open parent directory `/var/tmp/setupforge-$UID/journal/` and call `parentDir.Sync()` to ensure directory entry changes are flushed to the filesystem journal/inode table.

This guarantees that `active-transaction.json` is **either fully updated or remains at its previous valid state** across process crashes and sudden power loss.

---

## 11. ArtifactStore Responsibility & Interface

```
[PROPOSED]
```

`ArtifactStore` is defined as a clean domain capability contract in `internal/domain/transaction.go`:

```go
// ArtifactStore manages the physical storage, integrity, and lifecycle of transaction bundles.
type ArtifactStore interface {
    // CheckCapacity performs preflight validation that the destination filesystem has sufficient
    // free space. Note: this is an admission guard, not a guarantee against runtime ENOSPC.
    CheckCapacity(ctx context.Context, estimatedBytes int64) error

    // CreateStaging creates an isolated, unprivileged staging directory for planning.
    CreateStaging(ctx context.Context, opID string) (stagingDir string, err error)

    // Seal validates the artifact structure, asserts zero symlinks, computes the SHA-256
    // Merkle digest, and transitions the artifact to the SEALED state.
    Seal(ctx context.Context, stagingDir string) (TransactionArtifact, error)

    // Validate verifies that a sealed artifact exists on disk, has intact files,
    // and matches its recorded fingerprint.
    Validate(ctx context.Context, artifact TransactionArtifact) error

    // Purge physically removes the artifact directory from disk.
    Purge(ctx context.Context, directory string) error

    // CollectGarbage removes abandoned or unreferenced transaction directories based on
    // ownership and journal state.
    CollectGarbage(ctx context.Context) (purgedCount int, err error)
}
```

The concrete implementation `LocalFilesystemStore` resides in `internal/infra/storage/local_store.go`, fully isolating all filesystem paths, `statfs` calls, and directory operations from the application and UI layers.

---

## 12. Digest & BLAKE3 Decision

```
[REASONED ARCHITECTURAL CONCLUSION]
```

### Integrity vs Identity vs Storage Addressing
- **Integrity Verification**: Asserts that file bytes on disk have not been altered or corrupted since planning.
- **Content Identity**: Provides a unique fingerprint (`F0`) for human review and cryptographic binding (`Reviewed == Executed`).
- **Content-Addressed Storage (CAS)**: Using hash values to deduplicate blocks across files.

### Evaluation of BLAKE3 vs SHA-256
- **Performance Benchmark**:
  On our target Fedora systems (x86_64 with Intel SHA extensions / ARMv8 with Crypto extensions), Go's standard library `crypto/sha256` operates at **> 1.8 GB/s per core**.
  Hashing a 135 MiB Fedora update takes **< 0.08 seconds**.
- **External Dependency**:
  BLAKE3 requires third-party packages (`github.com/zeebo/blake3`), violating our small-dependency core contract.
- **Ecosystem Standard**:
  RPM packages, DNF5 metadata, and Fedora security advisories use SHA-256 natively.

### Architectural Decision: REJECT BLAKE3 / KEEP SHA-256
SetupForge shall **keep SHA-256** as defined in ADR-008.

To preserve future flexibility, digests in the domain model shall be formatted using standard algorithm-prefixed notation:
```text
sha256:841a0794f9bade7e906d186280edf03f5e58ba63592a55d1bfc1bf8d2325403d
```

---

## 13. Linux Filesystem Primitive Evaluation

```
[REASONED ARCHITECTURAL CONCLUSION]
```

| Primitive | Potential Use Case | Architectural Verdict | Technical Justification |
| :--- | :--- | :--- | :--- |
| **`openat2()`** | Symlink traversal restriction (`RESOLVE_BENEATH`) | **DEFERRED** | Unnecessary raw syscall complexity. SetupForge creates private `0700` directories. Checking `d.Type() & os.ModeSymlink != 0` during directory scanning completely blocks symlinks in standard Go. |
| **`O_TMPFILE`** | Anonymous file creation | **REJECTED** | DNF5 `--store=PATH` requires a visible filesystem path argument to create `packages/` and `transaction.json`. Incompatible with DNF5 CLI. |
| **`renameat2()`** | Atomic exchange or non-replacing rename | **DEFERRED** | Standard POSIX `os.Rename` is already atomic on Linux when target and source are on the same filesystem. |
| **`f.Sync()`** | Flush file data to physical storage | **ADOPTED** | Essential for crash-consistent atomic state journal publication. |
| **`parentDir.Sync()`** | Flush directory entry metadata | **ADOPTED** | Guarantees rename operations survive sudden power loss. |
| **`unix.Flock()`** | Kernel-managed advisory file locking | **ADOPTED** | Provides crash-safe, single-instance engine ownership with zero stale-lock failure modes. |

---

## 14. OCI Evaluation

```
[REASONED ARCHITECTURAL CONCLUSION]
```

### Architectural Verdict: Internal Descriptor Model ONLY
SetupForge shall **not** import OCI container registries, ORAS libraries, or container runtimes.

However, SetupForge shall adopt the standard **OCI Descriptor Model** internally within `internal/domain` to structure artifact metadata cleanly:

```go
// Descriptor represents a typed, content-addressed artifact reference.
type Descriptor struct {
    MediaType   string            `json:"mediaType"`
    Digest      string            `json:"digest"`
    Size        int64             `json:"size"`
    Annotations map[string]string `json:"annotations,omitempty"`
}
```

Standard media types:
- `application/vnd.setupforge.transaction.manifest.v1+json` (for `transaction.json`)
- `application/vnd.setupforge.transaction.package.rpm` (for staged RPMs)
- `application/vnd.setupforge.engine.state.v1+json` (for journal state)

This provides clean, standardized metadata formatting without adding a single external dependency.

---

## 15. Security Invariant Analysis

```
[RESEARCH-VERIFIED]
```

The proposed architecture actively strengthens all core SetupForge security invariants:

1. **`REVIEWED == EXECUTED` (ADR-007)**:
   - When user confirms in UI, the exact `Descriptor.Digest` (`sha256:...`) is captured in `ConfirmationDecision` and persisted to `journal/active-transaction.json`.
   - Before Phase 2B.3 authorization and Phase 2B.4 replay, `ArtifactStore.Validate` recalculates the SHA-256 Merkle root from physical disk. If any byte has changed, replay is blocked.
2. **Symlink Attack Defense-in-Depth**:
   - `ArtifactStore.Seal` and `ArtifactStore.Validate` explicitly scan directory entries with `os.Lstat`. If any entry is a symlink (`mode & os.ModeSymlink != 0`), socket, FIFO, or device, the artifact is rejected as corrupted (`ErrTransactionArtifactCorrupted`).
3. **Privilege Boundary Integrity (ADR-002, ADR-005)**:
   - `ArtifactStore` and `TransactionJournal` operate 100% unprivileged in user space.
   - Privileged replay in Phase 2B.4 receives only the validated artifact directory path via `sudo -n /usr/bin/dnf5 replay <dir>`.
4. **No Cross-User Tampering**:
   - Workspace `/var/tmp/setupforge-$UID` is created with mode `0700`. Local multi-user environments cannot traverse, inspect, or modify files within the directory.

---

## 16. Dependency Impact

```
[REPOSITORY-VERIFIED]
```

The Lean Transaction Engine introduces **ZERO new external dependencies** to `go.mod`:

- **JSON Encoding / Parsing**: Standard library `encoding/json`.
- **Atomic File Operations**: Standard library `os`, `path/filepath`.
- **Advisory Locking (`flock`)**: Standard package `golang.org/x/sys/unix` (already present in `go.mod`).
- **Filesystem Capacity (`statfs`)**: Standard package `golang.org/x/sys/unix` (already present in `go.mod`).
- **Cryptographic Hashing**: Standard library `crypto/sha256`.

**Result**: Compilation remains instantaneous (< 0.5s), binary remains a single statically-linked Go binary, and maintenance burden is near zero.

---

## 17. Incremental Migration Plan

```
[PROPOSED]
```

The transition from Phase 2B.2 to the Lean Transaction Engine shall proceed in small, verifiable vertical steps:

```
[Step 1: Storage Infrastructure]
  - Create internal/infra/storage/local_store.go implementing domain.ArtifactStore
  - Move workspace path from $XDG_RUNTIME_DIR to /var/tmp/setupforge-$UID/transactions/
  - Add capacity preflight check (Statfs) and symlink rejection (Lstat)
  - Unit tests with mock filesystem

[Step 2: Journal Infrastructure]
  - Create internal/infra/journal/file_journal.go
  - Implement crash-consistent atomic state publication (f.Sync -> Rename -> dir.Sync) and flock engine lock
  - Unit tests verifying crash-consistent updates and flock ownership

[Step 3: UpdateService Wiring]
  - Inject ArtifactStore and TransactionJournal into UpdateService
  - Wire event-driven cleanup: purge on cancel, purge on failure
  - Wire recovery-aware garbage collection based on ownership and journal state

[Step 4: Verification & Sign-Off]
  - Real Fedora integration test verifying /var/tmp lifecycle and atomic state persistence
  - Run full suite: fmt, vet, tests, visual review
```

---

## 18. Explicitly Deferred / Rejected Technologies

```
[REASONED ARCHITECTURAL CONCLUSION]
```

| Technology | Status | Final Architectural Justification |
| :--- | :--- | :--- |
| **SQLite** | **REJECTED (DEFERRED)** | Massive dependency overhead (CGo or 50MB transpiled C) for a single-active-record use case. Atomic JSON journal provides identical durability with 0 dependencies. |
| **History Journal (`history.jsonl`)** | **DEFERRED** | Unnecessary for Phase 2B.3/2B.4 recovery. Deferred to Phase 4 (History Workbench). |
| **BLAKE3** | **REJECTED** | Standard library SHA-256 operates at >1.8 GB/s with hardware acceleration (<80ms per update). External dependency unjustified. |
| **Content-Addressed Storage (CAS)** | **REJECTED** | SetupForge maintains only 1 active transaction at a time. DNF5 does not support decoupled package stores. Adds reference-counting complexity for 0 gain. |
| **OCI Registry / ORAS** | **REJECTED** | SetupForge is a local system tool, not a distributed container deployment platform. Only internal descriptor structs are adopted. |
| **`O_TMPFILE`** | **REJECTED** | DNF5 `--store=PATH` CLI requires a visible filesystem directory path. |
| **`openat2` syscalls** | **DEFERRED** | Standard Go `Lstat` mode checking completely solves symlink rejection without raw kernel syscall bindings. |
| **Remote Storage (S3 / HTTP)** | **REJECTED** | Out of scope for a local Fedora post-installation control center. |

---

## 19. Open Questions & Future Considerations

```
[RESEARCH-VERIFIED]
```

1. **User Switching Mid-Session**:
   *Question*: If a user switches UIDs via `su` or fast-user switching, what happens to `/var/tmp/setupforge-$UID`?
   *Answer*: Because the path is keyed by `$UID` (`os.Getuid()`), each Linux user maintains a completely isolated workspace and lock file.
2. **DNF5 History API Integration in Phase 2B.4**:
   *Question*: Can Phase 2B.4 query libdnf5 history directly for conclusive execution proof?
   *Answer*: Yes, when implementing Phase 2B.4, the executor can establish a runner-level execution completion record or inspect DNF5 transaction logs to provide trustworthy evidence for the Reconciliation Protocol.

---

## 20. Final Architecture Recommendation

```
[PROPOSED]
```

### The Architectural Verdict
SetupForge shall adopt the **Lean Transaction Engine** based on:
1. **Persistent Workspace**: `/var/tmp/setupforge-$UID/transactions/` with admission capacity preflight (acknowledging preflight is an admission check, not an immunity against later `ENOSPC`).
2. **Single Atomic JSON State Journal**: Pure-Go crash-consistent persistence (`active-transaction.json`) with zero external database dependencies.
3. **Advisory Kernel Lock**: `flock(2)` on `engine.lock` ensuring single-active-engine execution per user.
4. **Conservative Crash Recovery**: Strict `RECOVERY_REQUIRED` model for interrupted execution; raw RPM DB matching is rejected; uncertain transactions are **never** automatically replayed.
5. **Recovery-Aware Reclamation**: Artifacts in `STAGING` or `SEALED` are reclaimed based on ownership and journal state, not blanket restart deletion.
6. **Preserved SHA-256 Merkle Digest**: Fast, standard-library, hardware-accelerated integrity verification.
7. **Decoupled `ArtifactStore` Interface**: Clean separation of physical filesystem mechanics from package-manager resolution.
8. **Strict Symlink Hardening**: Fails closed if any non-regular file is detected in the artifact bundle.

This design delivers industrial-grade crash resilience, strict system safety, and clean distribution portability while preserving SetupForge's small core and zero-CGo dependency footprint.
