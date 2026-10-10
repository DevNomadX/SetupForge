# ADR-012: Privileged Transaction Execution, Authorization Boundary & Crash Recovery

- **Status**: Accepted / Evidence-Hardened & Validated
- **Context**:
  In Phase 2B.1, SetupForge established unprivileged DNF5 transaction planning using `--store=DIR`, capturing `transaction.json` and package payloads into an isolated staging directory. In Phase 2B.2, SetupForge implemented user review and explicit cryptographic confirmation binding the exact `OperationID`, artifact directory, and SHA-256 Merkle root digest without executing commands or elevating privileges. In Phase 2B.3, SetupForge implemented a lean, durable transaction engine: transaction workspaces were relocated to persistent disk at `/var/tmp/setupforge-$UID/transactions/`, single-engine ownership was enforced via kernel advisory `flock(2)` on `engine.lock`, and an atomic single-file journal (`active-transaction.json`) was established with strict state transition validation and symlink/hardlink rejection.

  Executing system-level package updates introduces critical safety and correctness challenges:
  1. **Privilege Boundary**: SetupForge must elevate privileges safely without prompting for or storing passwords in the TUI, without freezing the Bubble Tea event loop, and without invoking shell interpreters.
  2. **Security Invariant (`REVIEWED_ARTIFACT == EXECUTED_ARTIFACT`)**: The packages, versions, and scripts reviewed and confirmed by the user must be the exact ones executed on the system.
  3. **Non-Atomicity of Package Updates**: RPM transactions modify the filesystem sequentially. If a crash or power cut occurs during execution, the system is left in a partially updated state.
  4. **Scoped Evidence Principle**: SetupForge must never claim a transaction outcome that it cannot prove from authoritative execution evidence. In particular, querying the RPM database alone cannot prove transaction completion.
  5. **Conservative Recovery Invariant**: Uncertain executions must never be automatically replayed or retried.

- **Decision**:

  ### 1. Privileged Execution Mechanism
  SetupForge executes DNF5 stored transactions using non-interactive sudo:
  ```bash
  /usr/bin/sudo -n /usr/bin/dnf5 --comment="setupforge:<operation_id>" -y replay <artifact_directory>
  ```
  - **Zero Password Handling**: The UI never prompts for, reads, or stores passwords.
  - **Non-Interactive Sudo (`-n`)**: Sudo checks for existing cached credentials (e.g. from `sudo -v`) or sudoers NOPASSWD configuration. If credentials are missing, sudo exits immediately with code 1 (`sudo: a password is required`), which maps cleanly to a typed domain error without blocking the TUI.
  - **Direct Argument Vectors**: Commands are executed directly via `exec.CommandContext` with discrete argument slices. Shell interpreters (`sh`, `bash`, `sh -c`) are strictly prohibited.
  - **Zero Relaxation Flags**: Replay flags that weaken exactness (`--ignore-installed`, `--ignore-extras`, `--skip-broken`, `--skip-unavailable`) are strictly forbidden. If the host system state has drifted since planning, `dnf5 replay` safely aborts without altering packages.

  ### 2. Authorization Boundary (`AuthorizationProvider`)
  - Introduce domain interface `AuthorizationProvider` and infrastructure adapter `SudoAuthorizationProvider`.
  - Authorization is verified prior to execution via a fast preflight check (`sudo -n true`).
  - **`AUTHORIZED` Semantics**: Authorization is an instantaneous execution precondition (an in-memory gate). Because kernel sudo credential caches do not survive reboots, authorization is not a persistent cross-reboot credential. If recorded in the journal, an interrupted `AUTHORIZED` transaction on reboot reverts to `CONFIRMED` or `CANCELLED`.

  ### 3. Pre-Execution Revalidation Gate (Check B)
  Immediately prior to spawning the privileged process, `UpdateService` performs an unprivileged revalidation of the artifact bundle:
  1. Asserts active engine lock (`flock`) is held.
  2. Asserts active journal state is strictly `CONFIRMED`.
  3. Performs recursive `Lstat` scan: re-verifies 0 symlinks, 0 non-regular files, and `Nlink == 1` for all files.
  4. Recalculates the streaming SHA-256 Merkle root digest across all regular files and asserts byte-for-byte equality with `journal.ArtifactFingerprint`. Any discrepancy immediately halts execution and flags tampering.
  5. Re-verifies storage capacity on the root filesystem via `unix.Statfs`.
  6. Captures execution baseline metadata (`BaselineLastTransID`, `BaselineStartTime`, `BaselineRPMDBVersion`).
  7. Persists `EXECUTING` state to `active-transaction.json` (`f.Sync` $\rightarrow$ `Rename` $\rightarrow$ `dir.Sync`) **before** process spawn.

  ### 4. Process Containment & Lifecycle
  - **Process Group Isolation**: Spawns child process with `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`.
  - **Sanitized Environment**:
    - `PATH` set to `/usr/bin:/bin:/usr/sbin:/sbin`.
    - Locale set to `LC_ALL=C.UTF-8` and `LANG=C.UTF-8`.
    - Strips all dynamic loader hooks (`LD_PRELOAD`, `LD_LIBRARY_PATH`, `LD_AUDIT`).
  - **Closed Stdin**: `cmd.Stdin = nil` (connected to `/dev/null`) prevents subprocesses from waiting for terminal input.
  - **Safe Working Directory**: Working directory set to `/`.
  - **Streaming Pipes**: Captures `StdoutPipe` and `StderrPipe` to stream progress events without buffering large outputs in RAM.

  ### 5. Signal Propagation & Bounded Escalation
  - When cancellation is requested or `SIGINT` is received, SetupForge sends `SIGINT` to the child process group (`syscall.Kill(-pgid, syscall.SIGINT)`).
  - SetupForge waits a bounded grace period (30 seconds), displaying: `"Cancellation requested. Waiting for process to exit gracefully..."`.
  - If the process fails to exit within 30 seconds, SetupForge escalates to `SIGTERM` (10s window).
  - Forced `SIGKILL` (`syscall.Kill(-pgid, syscall.SIGKILL)`) is strictly a last resort and immediately transitions the transaction to `MANUAL_RECOVERY`.
  - **Core Invariant**: **«Signal handling does not establish transaction success or failure.»** Actual package-system outcome must still be determined from authoritative DNF5 transaction evidence during normal completion or recovery. SetupForge never assumes package-level atomicity merely because `SIGINT` was delivered.

  ### 6. Authoritative Evidence Model & Correlation Semantics
  SetupForge correlates transactions with DNF5's persistent SQLite history (`/usr/lib/sysimage/libdnf5/transaction_history.sqlite`):
  - **Primary SetupForge Correlation Identity**: Cryptographically random `OperationID` bound to the exact transaction comment/correlation marker passed via `--comment="setupforge:<OperationID>"`. The exact OperationID/comment correlation remains the central SetupForge identity binding.
  - **Corroborating Consistency Checks**: DNF5 history transaction ID (`h.id > BaselineLastTransID`), start timestamp (`h.dt_begin >= BaselineStartTime - 60`), and baseline RPMDB checksum (`h.rpmdb_version_begin == BaselineRPMDBVersion`). These checks strengthen correlation and detect inconsistencies or stale historical evidence, but are corroborating consistency checks rather than independent primary identities or mathematical guarantees.
  - **`ProvenSuccess`**: Process exit code is 0, AND DNF5 history contains a correlated record with state `Ok` (`state_id = 2`), non-null `dt_end`, and valid `rpmdb_version_end`.
  - **`ProvenFailure`**: Process exit code is 1 or 2 with direct proof of pre-execution abort (0 packages modified), OR DNF5 history records state `Error` (`state_id = 3`).
  - **`UnknownOutcome`**: Any execution that does not satisfy `ProvenSuccess` or `ProvenFailure` transitions to `RECOVERY_REQUIRED`.

  ### 7. Corrected Recovery State Machine (`RECOVERY_REQUIRED` & `MANUAL_RECOVERY`)
  - **`RECOVERY_REQUIRED` is an Active Reconciliation State**: Upon encountering an un-finalized execution (e.g. after restart), SetupForge evaluates available authoritative evidence:
    - If authoritative evidence proves success (e.g. crash occurred after DNF5 succeeded but before journal flush): promotes cleanly to `COMPLETED`.
    - If authoritative evidence proves clean failure: promotes cleanly to `FAILED`.
    - If authoritative evidence is missing or inconclusive (e.g. power loss during RPM unpacking, `state_id = 1` 'Started'): transitions to `MANUAL_RECOVERY`.
  - **`MANUAL_RECOVERY` is Terminal for Automation**:
    - **Zero automatic replay.** SetupForge will never automatically retry an interrupted transaction.
    - SetupForge presents an actionable 4-question recovery dialog in the UI and displays the manual command the operator can execute in their root terminal: `sudo dnf5 history redo <id>`. (Explicitly disclosed as external manual recovery, NOT SetupForge replay).
    - **Engine Admission Blocked**: While `MANUAL_RECOVERY` is terminal for the individual transaction, the transaction engine blocks new planning (`PlanUpdate` fails closed with `ErrRecoveryRequired`) to protect forensic evidence in `active-transaction.json` from overwrite and prevent operating against an unverified host RPM state.
  - **Legacy `INTERRUPTED` Compatibility Policy**:
    - Existing Phase 2B.3 journal records with state `INTERRUPTED` remain recognized for reading (`IsValid() == true`) and are classified as terminal (`IsTerminal() == true`).
    - SetupForge does not silently mutate or rewrite existing on-disk `INTERRUPTED` journals; their artifacts are protected from garbage collection, and startup recovery returns `domain.ErrInterruptedTransaction`.
    - All incoming transitions into `INTERRUPTED` (including `RECOVERY_REQUIRED -> INTERRUPTED`) and all outgoing transitions from `INTERRUPTED` are rejected by `ValidateTransition`. Automatic replay from `INTERRUPTED` is strictly prohibited.
    - All new inconclusive recovery paths transition strictly to `MANUAL_RECOVERY`. Planning remains blocked while `INTERRUPTED` persists.

  ### 8. Truthful Progress Streaming
  - Because `dnf5 replay` does not support `--json`, SetupForge streams stdout line-by-line and applies regex pattern matching (`^\[\s*(\d+)/(\d+)\]\s+(\w+)\s+([^\s]+)`).
  - Progress percentages are rendered **only** when explicit `[i/N]` step markers are present.
  - During scriptlets, GPG checks, and preflight, the UI renders an active indeterminate spinner with the exact text message from DNF5.

- **Consequences**:
  - Eliminates stale historical false positives via rigorous correlation checks.
  - Production-grade execution safety on Fedora with zero external CGo dependencies.
  - Strict adherence to `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT` across the privileged boundary.
  - Failures and crashes are reconciled accurately without risking double-installs or silent package corruption.
  - Zero external CGo dependencies; implemented purely with Go standard library, `sudo`, `dnf5`, and Linux kernel primitives.

- **Rejected Alternatives**:
  - *Bare Comment Correlation*: Rejected because without transaction ID monotonicity and start timestamp bounds, stale historical records can produce false-positive success classifications.
  - *Polkit `pkexec`*: Rejected because `pkttyagent` interferes with Bubble Tea raw terminal mode and can cause terminal corruption or deadlocks in headless/SSH environments.
  - *Password Prompts in TUI*: Rejected because handling, passing, or storing plaintext root passwords in application memory is an unacceptable security hazard and violates project safety rules.
  - *Replay Relaxation Flags (`--ignore-installed`, `--skip-broken`)*: Rejected because they permit DNF5 to alter or skip packages without user review, violating `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT`.
  - *Relying on RPM Database Queries for Completion*: Rejected because RPM database queries cannot verify scriptlet execution, transaction atomicity, or operation correlation.
  - *Automated `dnf5 history redo` on Startup*: Rejected because automatically running privileged commands on crash recovery could repeat a fatal scriptlet crash or infinite loop.
  - *External Daemon Architecture (`setupforged`)*: Rejected as premature complexity; the direct subprocess execution model with `sudo -n`, `flock(2)`, and process-group signal containment provides complete control without daemon sprawl.
