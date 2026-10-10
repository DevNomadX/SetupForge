# SetupForge — Phase 2B.4 DNF5 Execution Evidence & Recovery Research

**Document ID**: `docs/PHASE-2B-4-EXECUTION-RESEARCH.md`
**Status**: Technical Research & Evidence-Hardened Validation
**Target Milestone**: Phase 2B.4 (Privileged Execution & Recovery Gate)
**Host Target**: Fedora Linux (Fedora 44 Workstation, `dnf5 5.4.6.0`, `libdnf5 5.4.6.0`, RPM 4.20+)
**Author**: Antigravity / SetupForge Architecture Gate

---

## 1. Executive Summary & Purpose

This document completes the final evidence-hardening audit for the privileged execution boundary and execution evidence model in SetupForge Phase 2B.4.

The core principle governing Phase 2B.4 is:

> **«SetupForge must never claim a transaction outcome that it cannot prove from authoritative execution evidence.»**

This hardened report replaces provisional assumptions with empirically bounded findings:
1. Clarifies the **correlation identity hierarchy**: the primary SetupForge correlation identity is `OperationID ↔ exact SetupForge transaction comment/correlation marker`. DNF5 history transaction ID monotonicity, start timestamp bounding, and baseline RPMDB checksums are corroborating consistency checks that strengthen correlation and detect stale historical evidence or replay collisions, rather than independent primary identities or mathematical guarantees.
2. Explicitly bounds the interaction between SetupForge's concurrency lock (`engine.lock`) and external Linux package managers (which are governed by DNF5's native system-repository locking semantics).
3. Formalizes the primary security invariant as **`REVIEWED_ARTIFACT == EXECUTED_ARTIFACT`**, explicitly disclosing the boundary between binary package provenance and un-audited RPM scriptlet execution.
4. Refines the recovery state machine so that **`RECOVERY_REQUIRED`** is an active reconciliation state and **`MANUAL_RECOVERY`** is the terminal state for automated execution, barring automatic replay.
5. Re-evaluates the external `dnf5 history redo` command as an administrative recovery option that does *not* preserve the reviewed artifact invariant.
6. Analyzes the SQLite WAL durability boundary and traces ten explicit execution interruption points (Stages A through J).

---

## 2. Scoped Authoritative Evidence Hierarchy

To avoid overclaiming what any single subsystem can prove, evidence sources are categorized according to their explicit evidentiary scope:

| Evidence Source | Classification | Scope of Authority | Inherent Limitations & Failure Modes |
|---|---|---|---|
| **SetupForge Journal** (`active-transaction.json`) | **AUTHORITATIVE** *(Internal)* | Authoritative for SetupForge's internal lifecycle, user confirmation intent, and recorded artifact fingerprint. | Cannot prove whether an external tool modified host packages while SetupForge was offline. |
| **DNF5 Transaction History** (`transaction_history.sqlite`) | **AUTHORITATIVE** *(Recorded DNF5 Lifecycle)* | Authoritative for DNF5's recorded transaction lifecycle, DNF5 start/end timestamps, recorded package alterations, and DNF5 outcome status (`Started`, `Ok`, `Error`). | Does not guarantee that files on disk remain unaltered if sudden power loss occurred before SQLite WAL flush or filesystem sync. |
| **RPM Database** (`/var/lib/rpm/rpmdb.sqlite`) | **SUPPORTING** *(Host DB State)* | Evidence of current package registration in the RPM database. | **INSUFFICIENT** to prove SetupForge transaction completion. Cannot prove whether packages were altered by SetupForge or external tools; cannot prove whether `%posttrans` scriptlets succeeded. |
| **Host Filesystem** (`/usr`, `/etc`, `/var`) | **SUPPORTING** *(Physical File State)* | Evidence of file presence and file modes on disk. | Not independently attributable to SetupForge; modified by many subsystems. |
| **Process Exit Status & Signals** | **SUPPORTING** *(Process Observation)* | Direct observation of whether the child process exited normally (`0`), failed (`1`, `2`), or was signaled (`SIGKILL`, `SIGTERM`). | Volatile across system crashes. Lost if SetupForge dies before recording the result. |
| **Stdout / Stderr Streams** | **SUPPORTING** *(Live Diagnostic Feedback)* | Real-time step progress (`[i/N]`), scriptlet error messages, human diagnostics. | Non-authoritative strings; vulnerable to terminal formatting changes. |

---

## 3. DNF5 Transaction History Architecture & Durability Boundaries

### 3.1 Database Location & Configuration
- **Location**: `/usr/lib/sysimage/libdnf5/transaction_history.sqlite`
- **Ownership/Mode**: `-rw-r--r--. 1 root root` (world-readable, root-writable).
- **SQLite Pragmas**:
  - `journal_mode = wal` (Write-Ahead Logging).
  - `synchronous = 2` (`FULL` synchronous mode). SQLite flushes WAL frames to physical media with `fdatasync()` at transaction commits.
  - `sqlite_sequence`: Tracks the highest allocated auto-incrementing integer ID.

### 3.2 Table Schema & State Definitions
In the SQLite schema:
- `trans.id`: `INTEGER PRIMARY KEY AUTOINCREMENT`. Monotonically increasing within a single database file. In SQLite, `AUTOINCREMENT` prevents reuse of deleted IDs. (Assumption: The database file is not manually deleted or recreated by an administrator).
- `trans.state_id`: References `trans_state(id)`:
  - `1`: `Started`
  - `2`: `Ok`
  - `3`: `Error`
- `trans.dt_begin`: Wall-clock Unix timestamp (seconds) when transaction initialized.
- `trans.dt_end`: Wall-clock Unix timestamp when transaction finalized (`NULL` if interrupted).
- `trans.rpmdb_version_begin`: SHA-256 digest of RPM database prior to execution.
- `trans.rpmdb_version_end`: SHA-256 digest of RPM database after execution (`NULL` if interrupted).
- `trans.comment`: User string passed via `--comment`.

### 3.3 Timing of SQLite Writes in `libdnf5`
Direct disassembly of `/usr/lib64/libdnf5.so.2` reveals the exact execution sequence:
1. `INSERT INTO trans ... state_id = 'Started'` is executed **before** `rpmtsRun()`.
2. `librpm` executes: `%pretrans` $\rightarrow$ package payloads and `%pre`/`%post` scriptlets $\rightarrow$ `%posttrans`.
3. `UPDATE trans ... state_id = 'Ok' | 'Error'` is executed **after** `rpmtsRun()` completes.
4. **Interruption Boundary**: If power is lost or the process receives `SIGKILL` during Step 2, the record remains in `state_id = 1` (`Started`) with `dt_end IS NULL`.
5. **Pre-WAL Flush Power Loss**: If power is cut immediately after spawn before the initial WAL frame for `INSERT` is flushed to disk, the database on reboot will contain **no record** of the transaction. Therefore, an absent history record after a crash is treated as **uncertain**, never assumed to be clean failure.

---

## 4. Hardened Transaction Correlation Model

### 4.1 Identity Hierarchy & Consistency Checks
To eliminate stale false positives while avoiding overclaiming absolute guarantees:
- **Primary SetupForge Correlation Identity**: The `OperationID` generated by SetupForge (a cryptographically random UUIDv4 or 128-bit hex string) bound to the exact transaction comment/correlation marker passed via `--comment="setupforge:<OperationID>"`. The exact OperationID/comment binding remains the central SetupForge identity binding.
- **Corroborating Consistency Checks** (strengthen correlation and help detect inconsistencies or stale evidence, but are not independent primary identities):
  1. `trans.id > BaselineLastTransID`: Monotonic transaction ID check ensuring the candidate history record was created after this execution attempt began (detecting stale records from prior attempts).
  2. `trans.dt_begin >= BaselineStartTime - 60`: Bounded temporal sanity check ensuring the record is consistent with the current session (tolerating minor clock skew; explicitly not a security-critical identity claim).
  3. `trans.rpmdb_version_begin == BaselineRPMDBVersion`: Verifies the candidate transaction began from the expected baseline host package state.

### 4.2 Correlation Evaluation
A DNF5 history record $h$ correlates to execution attempt $E$ when the central identity matches and corroborating consistency checks succeed:

$$\text{Correlated}(h, E) \iff \begin{cases}
h.\text{comment} = \text{"setupforge:"} + E.\text{OperationID} & \text{(Primary Identity Binding)} \\
h.\text{id} > E.\text{BaselineLastTransID} & \text{(Monotonic Freshness Check)} \\
h.\text{dt\_begin} \ge E.\text{BaselineStartTime} - 60 & \text{(Temporal Consistency Sanity Check)} \\
h.\text{rpmdb\_version\_begin} = E.\text{BaselineRPMDBVersion} & \text{(Baseline Host State Consistency)}
\end{cases}$$

These corroborating checks prevent stale history records from prior execution attempts from producing false-positive correlation. They do not claim mathematical infallibility on their own; their role is to corroborate the primary `OperationID` identity binding.

---

## 5. Analysis of Concurrent External DNF5 Activity

### 5.1 Lock Scopes
- **`engine.lock`** (`/var/tmp/setupforge-$UID/engine.lock`): An advisory kernel lock (`flock`) that protects against concurrent **SetupForge** instances. It has **no visibility or control** over external `dnf5`, `rpm`, or `packagekit` processes.
- **DNF5 Native System-Repository Lock**: DNF5's native system-repository lock arbitrates package-manager transactions and prevents conflicting access according to DNF5's own locking semantics. (Implementation note: On Fedora 44, this may concretely reside at `/var/lib/dnf/system-repo.lock`, but this concrete filesystem path is version/environment-dependent and is not part of the stable SetupForge architecture contract).

### 5.2 Interference Scenarios & Boundaries
1. **External Modification Between Planning and Replay**:
   - If an administrator installs, updates, or removes packages via CLI between SetupForge review and execution:
   - When SetupForge executes `dnf5 replay`, DNF5 validates current host installed packages against the preconditions recorded in `transaction.json`.
   - Because SetupForge supplies no relaxation flags, DNF5 detects the mismatch and **aborts with exit code 1 before modifying the host**.
2. **Concurrent DNF Execution During Replay**:
   - While SetupForge's `dnf5 replay` is active, DNF5 holds its native system-repository lock according to DNF5's own locking semantics.
   - Any external DNF5 process attempting to run concurrently will fail or block waiting for DNF5's native lock.

---

## 6. Formal Security Invariant: `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT`

### 6.1 Formal Definition
The invariant is defined as:

$$\text{REVIEWED\_ARTIFACT} \equiv \text{EXECUTED\_ARTIFACT}$$

Where an artifact $A$ is the 5-tuple:
$$A = \langle \text{OperationID}, \text{Directory}, \text{Fingerprint}, \text{transaction.json}, \text{Packages} \rangle$$
- $\text{Fingerprint}$ is the streaming SHA-256 Merkle root digest over all regular files in $\text{Directory}$, sorted lexicographically.
- **Check A (Confirmation)**: Unprivileged service verifies structure (`Lstat`, `Nlink == 1`, mode `0400`/`0500`) and asserts fingerprint equality.
- **Check B (Pre-Execution Gate)**: Immediately prior to spawn, unprivileged service repeats `Lstat` scan, recomputes the Merkle root digest byte-for-byte against the journal, and flushes `EXECUTING` state to disk.
- **Check C (Execution Boundary)**: `/usr/bin/sudo -n /usr/bin/dnf5 --comment="setupforge:<OpID>" -y replay <Directory>` executes strictly from `<Directory>`, loading packages from `<Directory>/packages/` without relaxation flags.

### 6.2 Explicit Scriptlet Boundary Disclosure
- **Package Provenance**: SetupForge proves that the exact reviewed RPM binary files from the planned repositories are executed.
- **Scriptlet Execution**: RPM packages contain maintainer scripts (`%pre`, `%post`, `%posttrans`). These scripts execute arbitrary shell/binary code as root under `librpm`. SetupForge guarantees package provenance and EVR matching, but **does not decompile or audit RPM scriptlet logic**. This is an intrinsic boundary of the RPM packaging system.

---

## 7. Replay Payload & Network Behavior (Empirical Findings)

Testing on Fedora 44 with `dnf5 5.4.6.0` confirms:
1. `dnf5 replay <path>` consumes stored metadata and local packages from `<path>/packages/`.
2. Repository IDs in stored transactions are namespaced to `@stored_transaction(repo_id)` pointing to the local directory.
3. Network mirrors are **not contacted** during replay under standard invocation without relaxation flags.
4. If a package in `packages/` is corrupted or modified, pre-execution Check B detects the SHA-256 Merkle root mismatch and halts. In addition, `librpm` verifies internal package header digests and OpenPGP signatures.

---

## 8. Process Containment, Sudo & Signal Semantics

### 8.1 Sudo Signal Limitations & Process Groups
- `man sudo` documents that `sudo` relays `SIGINT` and `SIGQUIT`, but **cannot relay `SIGKILL` or `SIGSTOP`**.
- Calling `cmd.Process.Kill()` on `sudo` kills `sudo` immediately, potentially leaving `dnf5` and scriptlet processes running as orphans reparented to PID 1.
- **Mandatory Control**: SetupForge spawns child commands with `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and delivers signals to the **process group** (`syscall.Kill(-pgid, signal)`). The Linux kernel delivers the signal directly to every descendant process in that group.

### 8.2 Cancellation Protocol & Bounded Signal Escalation
When user cancellation is requested (`c` / `Esc`) during `EXECUTING`:

```text
SetupForge requests SIGINT
        ↓
wait bounded grace period (30s)
        ↓
SIGTERM if still running
        ↓
wait bounded grace period (10s)
        ↓
SIGKILL if necessary
```

1. SetupForge delivers `SIGINT` to the child process group (`-pgid`).
2. SetupForge waits a bounded grace period (30 seconds), displaying: `"Cancellation requested. Waiting for process to exit gracefully..."`.
3. If the process has not terminated after 30 seconds, SetupForge escalates by delivering `SIGTERM` to `-pgid` and waits a second bounded grace period (10 seconds).
4. If still running, SetupForge delivers `SIGKILL` to `-pgid` as a last resort, immediately transitioning the transaction to `MANUAL_RECOVERY`.

**Core Invariant**:
> **«Signal handling does not establish transaction success or failure.»**
Actual package-system outcome must still be determined from authoritative DNF5 transaction evidence during normal completion or recovery. SetupForge never assumes package-level atomicity merely because `SIGINT` was delivered.

---

## 9. Truthful Progress Reporting Contract

- `dnf5 replay` does not support `--json`.
- SetupForge parses stdout using regex: `^\[\s*(\d+)/(\d+)\]\s+(\w+)\s+([^\s]+)`.
- **Determinate Progress**: Rendered **only** when `[i/N]` step counters are parsed from DNF5.
- **Indeterminate Progress**: During scriptlets, pre-transaction checks, GPG verification, and cleanup, the UI renders an **active indeterminate spinner** with the raw descriptive line from DNF5.
- **Honesty Guarantee**: Zero synthetic or time-interpolated percentages.

---

## 10. Comprehensive Crash & Power-Loss Matrix (Stages A through J)

| Stage | Interruption Boundary | Journal State | DNF5 History State | Process State | Host DB State | Predicate Evaluation | Resulting State | Automatic Replay Allowed? |
|---|---|---|---|---|---|---|---|---|
| **A** | Before process spawn | `EXECUTING` | No record | Not spawned | Unchanged | ProvenFailure (Observed non-spawn) | `FAILED` | No |
| **B** | After spawn, before DNF5 history creation | `EXECUTING` | No record | Exited / Dead | Unchanged | Inconclusive (Post-crash) | `RECOVERY_REQUIRED` $\rightarrow$ `MANUAL_RECOVERY` | **NO** |
| **C** | After Started written, before RPM execution | `EXECUTING` | `state_id = 1` (`Started`), $dt\_end=\text{NULL}$ | Dead | Unchanged | Inconclusive | `RECOVERY_REQUIRED` $\rightarrow$ `MANUAL_RECOVERY` | **NO** |
| **D** | During RPM payload unpack | `EXECUTING` | `state_id = 1` (`Started`), $dt\_end=\text{NULL}$ | Dead | Partially updated | UnknownOutcome | `RECOVERY_REQUIRED` $\rightarrow$ `MANUAL_RECOVERY` | **NO** |
| **E** | During RPM scriptlet run | `EXECUTING` | `state_id = 1` (`Started`), $dt\_end=\text{NULL}$ | Dead | Partially updated | UnknownOutcome | `RECOVERY_REQUIRED` $\rightarrow$ `MANUAL_RECOVERY` | **NO** |
| **F** | After rpmtsRun, before history finalization | `EXECUTING` | `state_id = 1` (`Started`), $dt\_end=\text{NULL}$ | Dead | Fully updated | UnknownOutcome | `RECOVERY_REQUIRED` $\rightarrow$ `MANUAL_RECOVERY` | **NO** |
| **G** | After history Ok, before process exit | `EXECUTING` | `state_id = 2` (`Ok`), $dt\_end \ne \text{NULL}$ | Dead | Fully updated | ProvenSuccess | `RECOVERY_REQUIRED` $\rightarrow$ `COMPLETED` | No (Already done) |
| **H** | After process exit, before journal write | `EXECUTING` | `state_id = 2` (`Ok`), $dt\_end \ne \text{NULL}$ | Exited 0 | Fully updated | ProvenSuccess | `RECOVERY_REQUIRED` $\rightarrow$ `COMPLETED` | No (Already done) |
| **I** | During journal write finalization | Corrupted / `EXECUTING` | `state_id = 2` (`Ok`), $dt\_end \ne \text{NULL}$ | Exited 0 | Fully updated | ProvenSuccess | `RECOVERY_REQUIRED` $\rightarrow$ `COMPLETED` | No (Already done) |
| **J** | Sudden power loss at any boundary | `EXECUTING` | `state_id = 1` OR missing | Dead | Uncertain | UnknownOutcome | `RECOVERY_REQUIRED` $\rightarrow$ `MANUAL_RECOVERY` | **NO** |

---

## 11. Final Evidence Predicates & Recovery Model

### 11.1 Mathematical Predicates
- **`ProvenSuccess(E, res, h)`**:
  $$\text{res}.\text{ExitCode} = 0 \land \neg \text{res}.\text{Signaled} \land \text{Correlated}(h, E) \land (h.\text{state\_id} = 2) \land (h.\text{dt\_end} \ge h.\text{dt\_begin}) \land (h.\text{rpmdb\_version\_end} \ne \text{NULL})$$
- **`ProvenFailure(E, res, h)`**:
  $$( \text{res}.\text{ExitCode} \in \{1, 2\} \land \text{DirectPreExecutionFailure} ) \lor ( \text{res}.\text{ExitCode} = 1 \land \text{Correlated}(h, E) \land (h.\text{state\_id} = 3) \land (h.\text{dt\_end} \ne \text{NULL}) )$$
- **`UnknownOutcome(E, res, h)`**:
  $$\neg \text{ProvenSuccess}(E, \text{res}, h) \land \neg \text{ProvenFailure}(E, \text{res}, h)$$

### 11.2 State Machine Flow
```text
EXECUTING
    ├── ProvenSuccess ─────────→ COMPLETED (Purge/Reclaim artifact)
    │
    ├── ProvenFailure ─────────→ FAILED (Preserve diagnostics, allow new plan)
    │
    └── UnknownOutcome ────────→ RECOVERY_REQUIRED (Active Reconciliation)
                                      ├── Authoritative proof of success
                                      │   (Correlated h with state_id=2, dt_end!=NULL)
                                      │        ↓
                                      │    COMPLETED
                                      │
                                      ├── Authoritative proof of clean failure
                                      │   (Correlated h with state_id=3)
                                      │        ↓
                                      │      FAILED
                                      │
                                      └── Inconclusive / Missing proof
                                          (h with state_id=1 OR missing history)
                                               ↓
                                       MANUAL_RECOVERY (Terminal for automation)
                                               │
                                               ▼
                                      ZERO AUTOMATIC REPLAY
```

### 11.3 Clarification of the Manual Recovery Command
- The command `sudo dnf5 history redo <id>` is an **external, manual administrative tool** provided for operator convenience.
- It is **NOT** a SetupForge automatic replay.
- It does **NOT** guarantee equivalence to the reviewed artifact because it re-resolves dependencies against current active repositories and uses relaxation flags (`--ignore-installed`, `--ignore-extras`).

---

## 12. Phase 2B.4 Evidence-Hardening Conclusion

### 1. Final Execution Evidence Model
Evidence authority is strictly scoped: the SetupForge journal is authoritative for internal lifecycle; DNF5 history is authoritative for recorded DNF5 lifecycle; RPM DB and filesystem are supporting host state; process exit status and logs are supporting diagnostic signals.

### 2. Final Correlation Model
The primary SetupForge correlation identity is `OperationID ↔ exact SetupForge transaction comment/correlation marker`. DNF5 transaction ID monotonicity, start timestamp bounds, and pre-execution RPMDB version matching serve as corroborating consistency checks that detect inconsistencies and stale historical evidence.

### 3. Final Replay Semantics
`dnf5 replay` executes strictly from local stored payloads without relaxation flags, aborting if host installed packages have drifted.

### 4. Final Recovery State Machine
`RECOVERY_REQUIRED` is an active reconciliation state. `MANUAL_RECOVERY` is terminal for automation. **Zero automatic replay.** Pre-existing Phase 2B.3 journals with `INTERRUPTED` remain supported as read-only terminal states without in-place mutation, and incoming transitions into `INTERRUPTED` are rejected. While a transaction is in `MANUAL_RECOVERY` or `INTERRUPTED`, new planning is blocked to protect unresolved forensic evidence and prevent planning against an unverified host state.

### 5. Final `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT` Invariant
Cryptographically and structurally proven for all package payloads and transaction metadata via pre-execution Check B; explicit disclosure of un-audited RPM scriptlet execution.

### 6. Known Assumptions & Limitations
- Supported environment: Fedora Linux 44 with `dnf5 5.4.6.0+`.
- SetupForge's `engine.lock` does not lock external package managers; external conflicts are handled by DNF5's precondition check.
- RPM scriptlets are opaque code executed by `librpm`.

### 7. Explicit Unsupported Scenarios
- Replaying transactions across differing architecture or major OS release versions.
- Automatic recovery or blind replay following power loss during package unpacking.

### 8. Final Verdict

```text
Phase 2B.4 Final Evidence-Hardening Gate: APPROVED
Status: Ready to Freeze Phase 2B.4 Architecture
```
