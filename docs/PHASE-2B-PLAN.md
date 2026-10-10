# SetupForge — Phase 2B Architecture Plan: System Update Transaction

This document is the authoritative, evidence-backed architectural specification for Phase 2B: System Update Transaction in SetupForge.

---

## 1. Executive Summary

Phase 2A established least-privilege update status inspection (`dnf5 check-upgrade --json --setopt=system_state_dir=/var/empty`), decoupling presentation from system inspection and ensuring zero password prompts during normal application startup.

Phase 2B completes the update cycle: resolving an update transaction, presenting an exact plan for user review, acquiring system authorization safely, executing the transaction atomically, providing truthful progress feedback, verifying the result against Fedora's RPM database and DNF history, determining authoritative reboot requirements, and recording an audit trail.

### Critical Flaw Resolved from Initial Plan
The initial preliminary Phase 2B draft proposed:
```text
dnf5 upgrade --store=DIR → review plan → dnf5 upgrade -y (execute)
```
This contained a catastrophic safety violation: **the executed transaction was not guaranteed to match the reviewed transaction**. A network change, mirror desynchronization, or repository metadata update between review and execution could silently change packages, install unexpected dependencies, or alter versions without the user's consent.

### Refined Core Principle: Reviewed == Executed
Phase 2B establishes the strict transactional invariant:
```text
dnf5 upgrade --store=DIR → TransactionArtifact (transaction.json + packages/)
     ↓
User Reviews UpdatePlan built from transaction.json
     ↓
User Confirms Exact Multi-Component SHA-256 Fingerprint
     ↓
AuthorizationProvider (interactive sudo -v via terminal handoff)
     ↓
sudo -n dnf5 -y replay <TransactionArtifact> (strict replay of identical RPMs)
```
`dnf5 replay` strictly errors out if installed package versions or dependencies drift between review and execution, guaranteeing that SetupForge never applies an unreviewed transaction.

**Prohibited Replay Options**:
The following DNF5 replay options weaken the transaction and are strictly prohibited in SetupForge:
- `--ignore-installed` (PROHIBITED: ignores mismatches with installed system packages; can result in empty/partial transactions)
- `--ignore-extras` (PROHIBITED: ignores extra packages pulled into transaction)
- `--skip-unavailable` (PROHIBITED: silently drops packages that fail to download or are unavailable)
- `--skip-broken` (PROHIBITED: silently drops packages with dependency conflicts)

Under no circumstances will SetupForge discard the reviewed artifact and execute a newly resolved `dnf5 upgrade` or re-resolve dependencies after user confirmation. If replay fails, execution terminates safely with an explicit error.

---

## 2. Current Repository State & Boundaries

SetupForge currently enforces a strict 4-layer dependency architecture:
```text
Presentation (internal/ui/)
       ↓
Application (internal/app/)
       ↓
Domain (internal/domain/)
       ↓
Infrastructure (internal/infra/)
```

### Existing Components to Leverage:
1. `internal/domain/`:
   - `UpdateStatus`, `PackageUpdate`, `UpdateState`: read-only status models from Phase 2A.
   - `UpdateChecker`: domain contract implemented by `DNF5UpdateChecker`.
2. `internal/infra/privilege/`:
   - `PrivilegedRunner`: contract for executing elevated direct argument vectors (`exec.CommandContext`).
   - `SudoRunner`: production adapter executing `sudo -n`, with root EUID bypass.
3. `internal/infra/update/`:
   - `DNF5UpdateChecker`: unprivileged update inspection using `--setopt=system_state_dir=/var/empty`.
4. `internal/app/`:
   - `UpdateService`: coordinates update status caching and concurrent access.
5. `internal/ui/`:
   - Design system tokens (`internal/ui/theme`), responsive layout engine (`internal/ui/layout`), global frame (`app_model.go`), and overview workbench (`internal/ui/overview`).

---

## 3. Verified DNF5 Research Findings

All research was conducted and verified live on the target Fedora 44 (Rawhide) host running `dnf5 5.2.16` and `sudo 1.9.17p2`:

| Command / Flag | Verified Behavior & Evidence | Source / Exit Code |
|---|---|---|
| `dnf5 upgrade --store=PATH` | Stores transaction in `PATH/transaction.json` and downloads RPMs into `PATH/packages/`. Runs 100% unprivileged when `--setopt=system_state_dir=/var/empty` is passed. | Verified live; exit 0 |
| `dnf5 replay <PATH>` | Replays the exact stored transaction from `PATH/transaction.json` using pre-downloaded RPMs in `PATH/packages/`. Aborts with an error if installed packages or versions differ. | `man dnf5-replay`; exit 0 / error |
| `dnf5 history list --json` | Returns machine-readable list of transactions (`id`, `command_line`, `start_time`, `end_time`, `user_id`, `status`, `altered_count`). Database `/usr/lib/sysimage/libdnf5/transaction_history.sqlite` is mode `0644` (world-readable). | Verified live; exit 0 |
| `dnf5 history info <id> --json` | Returns structured details of a transaction (`nevra`, `action`, `reason`, `repository`, `rpmdb_version_begin`, `rpmdb_version_end`). | Verified live; exit 0 |
| `dnf5 needs-restarting --json` | Authoritative reboot check. Returns `[{"type":"reboot","reboot_required":bool,"packages":[...]}]`. Exits with **1** if reboot recommended, **0** if clean. | `man dnf5-needs-restarting`; exit 1 or 0 |
| `dnf5 needs-restarting --services --json` | Returns `[{"type":"unit","unit":"name.service"}]` for services requiring restart. | `man dnf5-needs-restarting`; exit 1 or 0 |
| `sudo -v` via controlling TTY | Validates/refreshes user sudo credentials. Running through `tea.ExecProcess` allows standard terminal password prompt without TUI interference or password capture. | Standard Linux PAM/sudo; exit 0 |

---

## 4. Architecture & Layering Diagram

```text
┌────────────────────────────────────────────────────────────────────────┐
│                          PRESENTATION LAYER                            │
│  Update Modal / View  ──►  Progress Sheet  ──►  Completion Report     │
│  (Confirm Plan)            (RPM Phase Locked)    (Reboot Badge)        │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ tea.Cmd / Domain Events
┌───────────────────────────────────▼────────────────────────────────────┐
│                           APPLICATION LAYER                            │
│                            UpdateService                               │
│  - Coordinates: Plan -> Authorize -> Execute -> Verify -> Journal     │
│  - Enforces: State transitions, non-cancellable execution lock         │
└──────────────────┬─────────────────┬───────────────────┬───────────────┘
                   │                 │                   │
┌──────────────────▼────────┐ ┌──────▼────────────┐ ┌────▼───────────────┐
│     DOMAIN CONTRACTS      │ │   DOMAIN MODELS   │ │   DOMAIN ERRORS    │
│  - UpdatePlanner          │ │ - TransactionArt. │ │ - ErrAuthDenied    │
│  - AuthorizationProvider  │ │ - UpdatePlan      │ │ - ErrReplayMismatch│
│  - UpdateExecutor         │ │ - TransactionProg.│ │ - ErrLocked        │
│  - TransactionVerifier    │ │ - PostUpdateReport│ │ - ErrNeedsReboot   │
│  - UpdateJournal          │ │ - RebootRequirement                    │
└──────────────────▲────────┘ └───────────────────┘ └────────────────────┘
                   │ Implements (Inward Dependencies Only)
┌──────────────────┴─────────────────────────────────────────────────────┐
│                         INFRASTRUCTURE LAYER                           │
│  DNF5Planner          ──►  dnf5 upgrade --store=PATH (unprivileged)    │
│  SudoAuthProvider     ──►  sudo -v (via tea.ExecProcess terminal tty)  │
│  DNF5ReplayExecutor   ──►  sudo -n dnf5 replay PATH -y                 │
│  DNF5Verifier         ──►  dnf5 history list/info & needs-restarting   │
│  FileJournal          ──►  ~/.local/share/setupforge/journal/          │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Transaction Artifact Model

### Structure & Abstraction
The `TransactionArtifact` represents the authoritative, verified DNF5-resolved transaction bundle stored on disk:

```go
package domain

import "time"

type TransactionArtifact struct {
    // OperationID is the unique SetupForge correlation identifier.
    OperationID string
    // Directory is the isolated filesystem path containing transaction.json and packages/.
    Directory string
    // Fingerprint is the multi-component deterministic SHA-256 digest of the entire artifact.
    Fingerprint string
    // CreatedAt is the timestamp when the planner stored the transaction.
    CreatedAt time.Time
    // PackageCount is the total number of RPM packages in the artifact.
    PackageCount int
    // TotalBytes is the total size of downloaded RPMs on disk.
    TotalBytes int64
}
```

### Storage Location & Permissions
- **Location**: Isolated directory created in `$XDG_RUNTIME_DIR/setupforge/transactions/<operation_id>/` (defaulting to `/tmp/setupforge-runtime-<uid>/transactions/<operation_id>/`).
- **Permissions**: Directory `0700` (`rwx------`), files `0600` (`rw-------`), owned exclusively by the executing user.
- **Root accessibility during replay**: Because `sudo` runs as root, root can read user runtime files in standard Fedora environments.

### Deterministic Artifact Digest Algorithm
The artifact fingerprint MUST cover every replay-relevant artifact component, not just `transaction.json`. To prevent directory traversal order or file boundary ambiguity from causing false mismatches or collisions, SetupForge uses a canonical Merkle-style streaming digest:

1. **File Discovery**: Recursively discover all regular files inside the artifact root directory (`transaction.json`, all RPMs in `packages/`, and any comps files) via `IsReplayRelevantArtifactFile`. Exclude non-regular filesystem objects (symlinks, sockets, pipes, FIFOs, character/block devices). No filename-based exclusions (`.tmp`, `.lock`) are permitted: DNF5 creates exclusively regular files, and any unexpected regular file within the artifact directory must invalidate the digest under the `Reviewed == Executed` invariant.
2. **Normalized Relative Paths**: Convert every file path to a relative path from the artifact root using forward slashes (`/`) regardless of OS (e.g., `packages/firefox-156.0.1-2.fc44.x86_64.rpm`).
3. **Deterministic Lexicographical Ordering**: Sort the list of relative file paths in strict byte-order sequence (`sort.Strings`). This guarantees that filesystem enumeration (`readdir`) order has zero effect on the computed digest.
4. **Framed Streaming Digest (SHA-256)**:
   Initialize a single `crypto/sha256` hasher and process each sorted file sequentially:
   - Write length-prefixed path framing: `PATH:<len(relPath)>:<relPath>\n`
   - Write length-prefixed size framing: `SIZE:<sizeBytes>\n`
   - Stream the file content bytes directly into the hasher in 64 KiB chunks (preserving low memory usage for large updates).
   - Write delimiter: `\nEND\n`
5. **Final Digest**: Format the final SHA-256 sum as a lowercase 64-character hexadecimal string.

```text
Deterministic Digest Framing:
┌─────────────────────────────────────────────────────────────┐
│ PATH:16:transaction.json\n                                  │
│ SIZE:429\n                                                  │
│ <transaction.json bytes>                                    │
│ \nEND\n                                                     │
│ PATH:48:packages/libmaxminddb-1.14.1-1.fc44.x86_64.rpm\n   │
│ SIZE:46124\n                                                │
│ <RPM file bytes>                                            │
│ \nEND\n                                                     │
└─────────────────────────────────────────────────────────────┘
                             ▼
                 SHA-256 Hex Digest (64 chars)
```

### Artifact Validity Rules
An artifact is validated against strict domain integrity rules before confirmation and before execution:
1. **Missing `transaction.json`**: If `transaction.json` is missing or unreadable, the artifact is **INVALID** (`ErrInvalidArtifact`).
2. **Missing Package Files**: Every package referenced in `transaction.json` with a `package_path` must exist on disk inside `packages/`, be readable, and have a non-zero size. If any file is missing, the artifact is **INCOMPLETE** (`ErrIncompleteArtifact`).
3. **Unreadable Files / Bad Permissions**: If any file cannot be read or directory permissions are compromised, the artifact is **UNREADABLE** (`ErrUnreadableArtifact`).
4. **Fingerprint Mismatch**: Before execution, SetupForge re-computes the deterministic digest. If the digest differs from `TransactionArtifact.Fingerprint` by even one bit, the artifact has changed after planning and execution is strictly aborted (`ErrTransactionArtifactCorrupted`).
5. **Zero Silent Re-resolution**: If an artifact fails validation, SetupForge does NOT re-resolve or regenerate dependencies. It aborts safely to `Failed` state.

---

## 6. UpdatePlan Domain Model (Field Classification)

The `UpdatePlan` is the user-facing presentation and decision model derived from `TransactionArtifact`.

### Field Classification Matrix

| Field | Type | Classification | Source & Verification Mechanism |
|---|---|---|---|
| `OperationID` | `string` | **DIRECTLY VERIFIED** | Generated by SetupForge domain UUID/nanoid. |
| `ArtifactFingerprint` | `string` | **DIRECTLY VERIFIED** | SHA-256 hash of `transaction.json`. |
| `Packages` | `[]PlannedPackage` | **DIRECTLY VERIFIED** | Parsed directly from `transaction.json` `rpms` array. |
| `TotalDownloadSize` | `int64` | **DIRECTLY VERIFIED** | Computed from `os.Stat` on downloaded files in `packages/`. |
| `UpgradeCount` | `int` | **DIRECTLY VERIFIED** | Count of items with `action == "Upgrade"`. |
| `InstallCount` | `int` | **DIRECTLY VERIFIED** | Count of items with `action == "Install"`. |
| `RemoveCount` | `int` | **DIRECTLY VERIFIED** | Count of items with `action == "Remove"`. |
| `ReplacedCount` | `int` | **DIRECTLY VERIFIED** | Count of items with `action == "Replaced"`. |
| `HasKernelUpdate` | `bool` | **DERIVED** | Derived by inspecting if any package name starts with `kernel`. |
| `HasSystemdUpdate` | `bool` | **DERIVED** | Derived by inspecting if any package name starts with `systemd`. |
| `OldEVR` / `NewEVR` | `string` | **DERIVED** | Derived by matching `Upgrade` and `Replaced` NEVRA pairs for same package name. |
| `InstalledSizeDelta` | `int64` | **NOT CURRENTLY AVAILABLE** | Excluded from Phase 2B (not in `transaction.json`). |
| `IsSecurity` | `bool` | **NOT CURRENTLY AVAILABLE** | Excluded from Phase 2B (no advisory metadata in `transaction.json`). |
| `PackageDBHealthy` | `bool` | **NOT CURRENTLY AVAILABLE** | Excluded from Phase 2B (requires separate RPM verify pass). |

### Domain Struct Definitions
```go
package domain

type PackageAction string

const (
    ActionUpgrade  PackageAction = "Upgrade"
    ActionInstall  PackageAction = "Install"
    ActionRemove   PackageAction = "Remove"
    ActionReplaced PackageAction = "Replaced"
)

type PlannedPackage struct {
    Name        string        // e.g. "firefox"
    Action      PackageAction // Upgrade, Install, Remove, Replaced
    NewNEVRA    string        // e.g. "firefox-156.0.1-2.fc44.x86_64"
    OldNEVRA    string        // e.g. "firefox-156.0.1-1.fc44.x86_64" (if replacing)
    RepoID      string        // e.g. "updates"
    Reason      string        // e.g. "External User", "Dependency"
    PackagePath string        // Path to RPM in artifact (empty for removals)
    SizeBytes   int64         // Size of RPM file on disk (0 for removals)
}

type UpdatePlan struct {
    OperationID         string
    Artifact            TransactionArtifact
    Packages            []PlannedPackage
    UpgradeCount        int
    InstallCount        int
    RemoveCount         int
    TotalDownloadBytes  int64
    HasKernelUpdate     bool
    HasSystemdUpdate    bool
    CreatedAt           time.Time
}
```

---

## 7. Transaction State Machine

The transaction follows a deterministic, unidirectional finite state machine with strict cancellation boundaries:

```text
       ┌──────────┐
       │   IDLE   │
       └────┬─────┘
            │ RequestUpdate()
            ▼
       ┌──────────┐               Err / Cancel
       │ PLANNING │ ──────────────────────────────────────┐
       └────┬─────┘                                       │
            │ Plan Generated                              │
            ▼                                             │
       ┌──────────┐                                       │
       │PLAN_READY│                                       │
       └────┬─────┘                                       │
            │ User Opens Modal                            │
            ▼                                             │
 ┌──────────────────────┐         Cancel (Esc / 'n')      │
 │AWAITING_CONFIRMATION │ ────────────────────────────────┤
 └──────────┬───────────┘                                 │
            │ Confirm ('y' / Enter)                       │
            ▼                                             │
       ┌───────────┐              Auth Denied / Cancel    │
       │AUTHORIZING│ ─────────────────────────────────────┤
       └────┬──────┘                                      │
            │ Auth Success (sudo -v OK)                   │
            ▼                                             │
       ┌───────────┐      [CANCELLATION STRICTLY LOCKED]  │
       │ EXECUTING │                                      │
       │(RPM Phase)│                                      │
       └────┬──────┘                                      │
            │ Process Terminated                          │
            ▼                                             │
       ┌───────────┐                                      │
       │ VERIFYING │                                      │
       └────┬──────┘                                      │
            ├──────────────────────────┐                  │
            │ Verified Match           │ Mismatch / Error │
            ▼                          ▼                  ▼
      ┌───────────┐              ┌──────────┐       ┌───────────┐
      │ COMPLETED │              │  FAILED  │       │ CANCELLED │
      └───────────┘              └──────────┘       └──────────┘
```

### Valid State Transitions Table

| From State | Trigger Event | To State | Side Effects / Actions |
|---|---|---|---|
| `Idle` | `StartPlan` | `Planning` | Spawn unprivileged `dnf5 upgrade --store` |
| `Planning` | `PlanSuccess` | `PlanReady` | Parse `transaction.json`, calculate hash |
| `Planning` | `PlanError` | `Failed` | Cleanup temp store dir, record error |
| `Planning` | `UserCancel` | `Cancelled` | Terminate process (SIGINT), cleanup dir |
| `PlanReady` | `OpenReview` | `AwaitingConfirmation` | Display Review modal |
| `PlanReady` | `Timeout` / `Reset` | `Idle` | Cleanup artifact dir |
| `AwaitingConfirmation` | `UserConfirm` | `Authorizing` | Invoke `AuthorizationProvider.Authorize()` |
| `AwaitingConfirmation` | `UserReject` | `Cancelled` | Cleanup artifact dir |
| `Authorizing` | `AuthSuccess` | `Executing` | Lock cancellation, invoke `dnf5 replay` |
| `Authorizing` | `AuthDenied` | `AwaitingConfirmation` | Display auth error banner, allow retry |
| `Authorizing` | `AuthCancel` | `Cancelled` | Cleanup artifact dir |
| `Executing` | `ProcessExit` | `Verifying` | Read DNF history, invoke `needs-restarting` |
| `Verifying` | `VerifySuccess` | `Completed` | Write journal, cleanup artifact, emit completion |
| `Verifying` | `VerifyMismatch` | `Failed` | Write journal with failure diagnosis |
| `Completed` | `Dismiss` | `Idle` | Clear modal, refresh Phase 2A update status |
| `Failed` | `Dismiss` | `Idle` | Clear modal |
| `Cancelled` | `Dismiss` | `Idle` | Clear modal |

---

## 8. Planner Architecture

### Command Invocation
The planner generates the transaction bundle without requiring root privileges:
```bash
/usr/bin/dnf5 upgrade \
    --store=/path/to/runtime/setupforge/transactions/<op_id> \
    -y \
    --setopt=system_state_dir=/var/empty
```

### Key Decisions
1. **Unprivileged Execution**:
   - *Decision*: Run the planner as the unprivileged user using `--setopt=system_state_dir=/var/empty`.
   - *Why*: Principle of least privilege. Resolving dependencies and downloading packages does not require modifying system state.
   - *Evidence*: Verified on Fedora 44 host; returns exit code 0 and stores packages cleanly.
   - *Trade-offs*: Downloaded packages occupy user runtime space until replayed.
2. **Structured Artifact Parsing**:
   - *Decision*: Read `transaction.json` from the store directory.
   - *Why*: Completely removes brittle scraping of ANSI/tabular terminal output.
   - *Evidence*: `transaction.json` adheres to a stable schema (`version: "1.0"`, array of `rpms` with `nevra`, `action`, `reason`, `repo_id`, and `package_path`).

---

## 9. Authorization Architecture

### Separation of Concerns
SetupForge strictly separates **authorization** (validating that the user has administrative permission) from **privileged execution** (running the command non-interactively).

```go
package domain

type AuthorizationProvider interface {
    // Authorize acquires or validates elevation credentials.
    // Must return nil if credentials are valid and active.
    Authorize(ctx context.Context) error
}
```

### Implementation: Terminal Handoff via `tea.ExecProcess`
In `internal/infra/privilege/sudo_auth.go`:
```go
// SudoAuthorizationProvider implements domain.AuthorizationProvider.
type SudoAuthorizationProvider struct {
    euidGetter func() int
}

func (p *SudoAuthorizationProvider) Authorize(ctx context.Context) error {
    if p.euidGetter() == 0 {
        return nil // Already elevated
    }
    // Check if sudo credentials are already cached
    cmd := exec.CommandContext(ctx, "sudo", "-n", "true")
    if err := cmd.Run(); err == nil {
        return nil // Cached ticket active
    }
    // Interactive prompt required: handed off to controlling terminal
    return ErrInteractiveAuthRequired
}
```

When `ErrInteractiveAuthRequired` is returned, the Bubble Tea model emits:
```go
tea.ExecProcess(exec.Command("sudo", "-v"), func(err error) tea.Msg {
    return AuthResultMsg{Err: err}
})
```
- **Why**: `tea.ExecProcess` temporarily halts the Bubble Tea render loop, releases raw mode on `/dev/tty`, hands terminal control directly to `sudo -v` for standard PAM password authentication, and cleanly resumes the TUI upon completion.
- **Safety Invariant**: SetupForge NEVER captures passwords in Go memory, NEVER creates a Bubble Tea password input widget, and NEVER parses sudo prompts.

---

## 10. Executor Architecture

### Verified Command Invocation
Once authorized, execution runs strictly non-interactively via `PrivilegedRunner`:
```bash
sudo -n /usr/bin/dnf5 -y replay /path/to/runtime/setupforge/transactions/<op_id>
```
Direct argument vector:
```go
args := []string{"sudo", "-n", "/usr/bin/dnf5", "-y", "replay", artifact.Directory}
```

### Exact Replay & Prohibited Flags
DNF5's man page (`man dnf5-replay`) explicitly defines:
> "The replay will perform the exact same operations on the packages as in the original transaction and will return with an error in case of any differences in installed packages or their versions."

To preserve the **Reviewed == Executed** invariant, the following options are **strictly prohibited** in SetupForge:
- `--ignore-installed`: Prohibited. Ignores mismatches with installed system packages; can result in an empty/partial transaction.
- `--ignore-extras`: Prohibited. Ignores extra packages pulled into transaction.
- `--skip-unavailable`: Prohibited. Silently drops packages that fail to download or are unavailable.
- `--skip-broken`: Prohibited. Silently drops packages with dependency conflicts.

If any system drift or mismatch occurs, `dnf5 replay` terminates with an error code, and SetupForge transitions to `Failed` rather than applying an unexpected or degraded diff.

### Unified Event Stream
Instead of multiple independent channels, `UpdateExecutor` exposes a single sequential channel:
```go
type TransactionEvent struct {
    Type      EventType // EventProgress, EventLog, EventComplete, EventFailed
    Progress  *TransactionProgress
    Result    *TransactionResult
    Error     error
}

type UpdateExecutor interface {
    Execute(ctx context.Context, artifact TransactionArtifact) (<-chan TransactionEvent, error)
}
```

---

## 11. Progress Architecture: Evaluation & Fallback Isolation

### Structured API Evaluation

1. **Direct libdnf5 C++ Integration (Option C)**:
   - *Investigation*: libdnf5 exposes C++ callback classes (`libdnf5::transaction::TransactionProgress`).
   - *Why Rejected*: There are no official upstream Go bindings for libdnf5. Integrating via Cgo (`CGO_ENABLED=1`) would require custom C++ wrapper shims, introduce C++ ABI fragility across Fedora updates, require build-time installation of `libdnf5-devel` (which is not installed by default on Fedora Workstation), prevent pure Go compilation, and destroy hermetic unit testing on non-Linux or CI environments.
2. **D-Bus `dnf5daemon-server` Integration (`org.rpm.dnf.v0`)**:
   - *Investigation*: Fedora 44 provides `dnf5daemon-server` exposing D-Bus interfaces.
   - *Why Rejected*: The D-Bus daemon is designed for interactive PackageKit sessions. It does NOT expose stored transaction `replay` directories (`transaction.json` + pre-downloaded offline packages). Furthermore, it relies on PolicyKit desktop prompts rather than headless-safe TTY authorization.
3. **Structured CLI Stream (Option A)**:
   - *Investigation*: Evaluated whether `dnf5 replay` provides a `--json` or `--progress-json` stream flag.
   - *Finding*: `dnf5 replay` does not support structured machine-readable progress streaming over stdout.

### Decoupled CLI Parsing Architecture (Option B Fallback)
Because structured APIs are either impractical or lack stored replay support, CLI stdout parsing is used as an infrastructure-layer fallback under strict architectural constraints:

1. **Strict Interface Boundary**:
   Progress parsing is isolated behind an infrastructure interface:
   ```go
   package update

   type TransactionProgressParser interface {
       // ParseLine converts a raw stdout/stderr line into domain progress or nil.
       ParseLine(line string) (*domain.TransactionProgress, bool)
   }
   ```
2. **UI Isolation (Zero String Parsing in Presentation)**:
   - UI code NEVER parses strings or inspects DNF output.
   - UI receives only typed `domain.TransactionProgress` structs over the Bubble Tea event channel.
3. **Anchored Step Metrics**:
   - `TotalSteps` and the list of expected packages are NOT guessed from stdout. They are anchored to the authoritative counts in `UpdatePlan` (derived from `transaction.json`).
4. **Deterministic Line Parsing**:
   Matches lines of the form:
   `^\[([0-9]+)/([0-9]+)\]\s+([A-Za-z]+):\s+(.+)$`
   Extracting `CurrentStep`, `TotalSteps`, `CurrentAction`, and `CurrentPackage`.
5. **Hermetic Test Fixtures**:
   The parser is verified with unit tests using captured stdout fixtures from real Fedora 44 DNF5 runs, ensuring zero reliance on live root execution during tests.

---

## 12. Cancellation & Safety Boundaries

SetupForge enforces an ironclad distinction between cancellable and non-cancellable phases:

| Phase | State | Cancellable? | UI Key / Action | Mechanism & Safety Guarantee |
|---|---|---|---|---|
| Plan Generation | `Planning` | **YES** | `Esc` / `q` | Sends `SIGINT` to unprivileged `dnf5 upgrade --store`. Temp directory purged. Zero host mutation. |
| Plan Review | `AwaitingConfirmation` | **YES** | `Esc` / `n` | Closes modal, purges artifact directory. Zero host mutation. |
| Authorization | `Authorizing` | **YES** | `Ctrl+C` in prompt | User aborts sudo prompt. State reverts to `AwaitingConfirmation`. Zero host mutation. |
| RPM Transaction | `Executing` | **NO (LOCKED)** | **DISABLED** | UI displays warning: `Applying system changes — do not interrupt.` `Esc`, `q`, and `Ctrl+C` are trapped and ignored. Process executes to completion or natural exit. No partial rollback promises. |
| Verification | `Verifying` | **NO (LOCKED)** | **DISABLED** | Non-mutating read-only queries. Completes in < 2 seconds. |

---

## 13. Verification Architecture

### Problem Solved
Never assume `dnf5 history info last` corresponds to SetupForge's transaction. Another process or user could have executed DNF in the interim.

### Correlation Algorithm
1. `UpdateExecutor` records `PreExecTimestamp := time.Now().Unix()` before launching `dnf5 replay`.
2. Upon process exit, `TransactionVerifier` runs `dnf5 history list --json`.
3. It iterates through the history array and finds the transaction satisfying:
   - `start_time >= (PreExecTimestamp - 2)`
   - `command_line` contains `"replay"` and the artifact path
   - `altered_count == plan.TotalPackageCount`
4. Once correlated, it retrieves full details via `dnf5 history info <matched_id> --json`.
5. It extracts `status == "Ok"`, `rpmdb_version_end`, and the list of altered packages.
6. If no correlation matches or status is not `"Ok"`, it marks the operation as `VerificationFailed`.

```go
package domain

type PostUpdateReport struct {
    OperationID       string
    DNFTransactionID  int
    Status            string // "Ok"
    PackagesAltered   int
    RPMDBVersionEnd   string
    RebootRequired    bool
    ServicesToRestart []string
    CompletedAt       time.Time
}
```

---

## 14. Authoritative Reboot Detection

### Single Authority Principle
Informational badges in the UI may note "Kernel updated", but **the sole authority determining `RebootRequired` is `dnf5 needs-restarting`**.

### Execution & Exit Codes
Executed via `PrivilegedRunner`:
```bash
sudo -n /usr/bin/dnf5 needs-restarting --json
```

### Exit Code Handling Matrix
- **Exit Code 0**: System is clean. `RebootRequired = false`.
- **Exit Code 1**: Reboot recommended! Output JSON contains `reboot_required: true` and the triggering packages. **This is a successful detection, NOT an error**.
- **Exit Code > 1** (or non-JSON stderr): Tool failure. Logged as diagnostic warning, falls back to `RebootRequired = plan.HasKernelUpdate`.

### Service Restarts
Executed via:
```bash
sudo -n /usr/bin/dnf5 needs-restarting --services --json
```
Parses units (e.g. `systemd-resolved.service`) requiring restart and lists them in the `PostUpdateReport`.

---

## 15. Journal Architecture

SetupForge maintains a persistent, human-readable audit log of system modifications.

### Path & Security
- Stored at `~/.local/share/setupforge/journal.jsonl` (mode `0600`).
- **Zero Secrets**: No passwords, tokens, or environment secrets are recorded.

### Journal Entry Schema
```json
{
  "operation_id": "sf-upd-20260930-7f2a",
  "operation": "system_upgrade",
  "start_time": "2026-09-30T16:54:00Z",
  "end_time": "2026-09-30T16:54:35Z",
  "status": "completed",
  "dnf_transaction_id": 29,
  "artifact_fingerprint": "a3f5b72...",
  "packages_altered": 7,
  "reboot_required": true,
  "services_to_restart": ["user@1000.service"],
  "rpmdb_version_end": "81174f8792051..."
}
```

---

## 16. UI Architecture & Responsive Wireframes

The UI strictly adheres to SetupForge's signature token system (`internal/ui/theme`), M-unit spatial grid, and panel borders with embedded bracket headers (`┌─ Title ─ count ─ pos ─┐`).

### 16.1 State 1: Plan Review & Confirmation Modal (`AwaitingConfirmation`)

#### S Breakpoint (80×24)
```text
┌─ System Update ────────────── [?] Help ─┐
│ 7 packages to update (129 MB download)   │
│                                          │
│ Packages:                                │
│ ↑ blivet-data    1:3.14.1 -> 1:3.14.2    │
│ ↑ firefox        156.0.1-1 -> 156.0.1-2  │
│ ↑ firefox-lang   156.0.1-1 -> 156.0.1-2  │
│ ↑ libmaxminddb   1.14.0-1 -> 1.14.1-1    │
│ [↓ 3 more packages...]                   │
│                                          │
│ ⚠ Authorization required to apply        │
│                                          │
│ [Enter / y] Confirm & Update   [Esc / n] Cancel │
└──────────────────────────────────────────┘
```

#### M / L Breakpoint (100×30 / 120×35)
```text
┌─ System Update Plan ────────────────────────────────── [?] Help ─ op: sf-upd-7f2a ─┐
│ Summary: 7 packages to upgrade · 0 to install · 0 to remove · 129 MB inbound       │
│                                                                                    │
│ Package Name              Current Version           New Version          Repo      │
│ ────────────────────────────────────────────────────────────────────────────────── │
│ ↑ blivet-data             1:3.14.1-1.fc44           1:3.14.2-1.fc44      updates   │
│ ↑ firefox                 0:156.0.1-1.fc44          0:156.0.1-2.fc44     updates   │
│ ↑ firefox-langpacks       0:156.0.1-1.fc44          0:156.0.1-2.fc44     updates   │
│ ↑ hunspell                0:1.7.3-1.fc44            0:1.7.4-2.fc44       updates   │
│ ↑ hunspell-filesystem     0:1.7.3-1.fc44            0:1.7.4-2.fc44       updates   │
│ ↑ libmaxminddb            0:1.14.0-1.fc44           0:1.14.1-1.fc44      updates   │
│ ↑ python3-blivet          0:1.14.1-1.fc44           0:1.14.2-1.fc44      updates   │
│                                                                                    │
│ Transaction Integrity: Verified offline bundle · SHA-256: 8a9f...3e1b             │
│ Requires privilege: sudo authentication will be requested on confirmation          │
│                                                                                    │
│  [ y / Enter ] Confirm & Apply Updates         [ n / Esc ] Cancel & Discard        │
└────────────────────────────────────────────────────────────────────────────────────┘
```

### 16.2 State 2: Progress & Non-Cancellable State (`Executing`)

#### S Breakpoint (80×24)
```text
┌─ Applying System Updates ───────────────┐
│ Phase: RPM Transaction                   │
│ [===================>      ] 71% (5/7)  │
│ Upgrading: libmaxminddb-1.14.1-1.fc44    │
│                                          │
│ ⚠ Applying system changes                │
│   Cancellation is disabled               │
└──────────────────────────────────────────┘
```

#### M / L Breakpoint (100×30 / 120×35)
```text
┌─ Applying System Updates ────────────────────────────────────────── [LOCKED] ──────┐
│ Current Phase: RPM Transaction Execution                                           │
│ Progress: [====================================>               ] 71% (Step 5 of 7) │
│                                                                                    │
│ Active Package: libmaxminddb-0:1.14.1-1.fc44.x86_64                                │
│ Action:         Upgrading package files and executing scriptlets                   │
│                                                                                    │
│ ⚠ SYSTEM STATE MUTATION IN PROGRESS                                                │
│ Cancellation has been disabled to protect system and package database integrity.   │
│ Please do not close terminal or interrupt power.                                   │
└────────────────────────────────────────────────────────────────────────────────────┘
```

### 16.3 State 3: Completion Report (`Completed`)

```text
┌─ System Update Complete ────────────────────────────────────────── [OK] ───────────┐
│ Status: Successfully updated 7 packages in 34 seconds                              │
│ DNF Transaction ID: #29 · RPM DB Verified                                          │
│                                                                                    │
│ ● REBOOT RECOMMENDED                                                               │
│ Updated core system libraries require a system restart to take full effect.        │
│                                                                                    │
│ Services requiring restart:                                                        │
│ • user@1000.service                                                                │
│                                                                                    │
│ Journal recorded: ~/.local/share/setupforge/journal.jsonl                          │
│                                                                                    │
│  [ Enter / Esc ] Done                                                              │
└────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 17. Testing Strategy

SetupForge enforces strict hermetic testing. **Ordinary tests must never mutate the developer's system or require root.**

### Test Suites
1. **Transaction Artifact Tests** (`internal/infra/update/artifact_test.go`):
   - Verifies creation of `TransactionArtifact` in temporary directory.
   - Verifies SHA-256 fingerprint generation.
   - Verifies that tampering with `transaction.json` triggers `ErrTransactionArtifactCorrupted`.
   - Verifies cleanup logic on cancel and completion.
2. **Plan Parser Tests** (`internal/infra/update/plan_parser_test.go`):
   - Hermetically tests parsing of actual captured Fedora 44 `transaction.json` fixtures.
   - Verifies accurate extraction of `PlannedPackage`, actions, NEVRAs, and size sums.
   - Verifies error handling on truncated or invalid JSON.
3. **Replay Executor Tests** (`internal/infra/update/replay_executor_test.go`):
   - Uses fake command executor returning simulated DNF replay stdout streams.
   - Verifies line scanner step increments (`[X/Y] Upgrading: ...`).
   - Verifies proper mapping of `TransactionEvent` stream.
4. **Verification & Reboot Tests** (`internal/infra/update/verifier_test.go`):
   - Hermetic fixtures of `dnf5 history list --json` and `dnf5 history info --json`.
   - Verifies exact correlation logic (matching start time, command line, altered count).
   - Verifies handling of `dnf5 needs-restarting` exit code 1 as valid reboot requirement.
5. **UI State Machine Tests** (`internal/ui/update_model_test.go`):
   - Pure in-memory Bubble Tea model tests verifying all transitions (`Idle` → `Planning` → `AwaitingConfirmation` → `Executing` → `Verifying` → `Completed`).
   - Verifies that `Esc` and `Ctrl+C` are ignored during `Executing` state.

---

## 18. Security Invariants

1. **Least-Privilege Planning**: Planning and dependency resolution run unprivileged (`euid != 0`) using `--setopt=system_state_dir=/var/empty`.
2. **Zero Password Capture**: SetupForge never reads, stores, prompts for, or proxies user passwords. Interactive authentication is strictly delegated to `sudo -v` via `tea.ExecProcess`.
3. **No Shell Interpolation**: All command invocations use explicit direct argument vectors (`exec.CommandContext`). Shell interpreters (`sh`, `bash`, `sh -c`) are strictly prohibited.
4. **Exact Replay Guarantee (Reviewed == Executed)**: The transaction confirmed by the user is the exact transaction executed. Strict replay (`dnf5 -y replay <path>`) errors out on any system drift or package divergence. All relaxation flags (`--ignore-installed`, `--ignore-extras`, `--skip-unavailable`, `--skip-broken`) are strictly prohibited.
5. **Deterministic Multi-Component Integrity**: Artifact digests cover `transaction.json` plus all downloaded RPMs and comps metadata using canonical framed streaming SHA-256. Any post-planning tampering or file corruption aborts execution.
6. **Zero Silent Re-Resolution**: Under no circumstances will SetupForge discard a confirmed artifact and re-resolve dependencies or run a fresh `dnf5 upgrade`. If replay fails, execution terminates safely.
7. **Safe Cancellation Boundary**: Cancellation is permitted during planning, review, and authorization, but strictly disabled during active RPM mutation.
8. **No Silent Elevation**: Privileged actions require explicit, intentional user confirmation.

---

## 19. Implementation Slices (2B.1 → 2B.6)

### Slice 2B.1: Transaction Artifact Model & DNF5 Update Planner
- **Scope**: Define `TransactionArtifact` in `internal/domain/`. Implement `DNF5UpdatePlanner` in `internal/infra/update/` running unprivileged `dnf5 upgrade --store=PATH --setopt=system_state_dir=/var/empty`. Deterministic multi-component SHA-256 fingerprinting.
- **Tests**: Hermetic artifact creation, hash verification, directory traversal ordering independence, and cleanup tests.
- **Safety**: 100% unprivileged; zero system modifications.

### Slice 2B.2: Update Plan Domain Model, Presentation & Confirmation
- **Scope**: Define `UpdatePlan` and `PlannedPackage`. Implement JSON parser for `transaction.json`. Build Bubble Tea Review Modal overlay in `internal/ui/` with responsive layouts (80×24 through 160×45+).
- **Tests**: Parser tests against real JSON fixtures; UI state transition and keybinding tests.
- **Safety**: Read-only presentation.

### Slice 2B.3: Authorization Boundary
- **Scope**: Define `AuthorizationProvider` contract. Implement `SudoAuthorizationProvider` in `internal/infra/privilege/`. Wire `tea.ExecProcess("sudo", "-v")` for terminal password handoff.
- **Tests**: Unit tests with fake EUID and exec checkers.
- **Safety**: Zero password capture; non-interactive fallback.

### Slice 2B.4: Transaction Execution & Real Progress Streaming
- **Scope**: Implement `DNF5ReplayExecutor` in `internal/infra/update/` running `sudo -n /usr/bin/dnf5 -y replay PATH` (prohibiting all relaxation flags). Implement isolated `TransactionProgressParser` infrastructure adapter for stdout line scanning (`[X/Y]` progress steps) anchored to `UpdatePlan` totals. Implement Bubble Tea progress view with cancellation lock during RPM phase.
- **Tests**: Mock stream scanner tests with captured Fedora 44 fixtures; state machine lock tests.
- **Safety**: Non-cancellable execution lock; exact replay invariant.
- **Safety**: Non-cancellable execution lock; exact replay invariant.

### Slice 2B.5: Post-Transaction Verification & Authoritative Reboot Detection
- **Scope**: Implement `DNF5TransactionVerifier` correlating DNF history via `dnf5 history list/info --json`. Implement `DNF5RebootDetector` parsing `dnf5 needs-restarting --json` (with exit code 1 handling).
- **Tests**: Correlation algorithm tests; needs-restarting fixture tests.
- **Safety**: Prevents false success states.

### Slice 2B.6: Journal Recording & End-to-End Fedora Integration
- **Scope**: Implement `FileJournal` in `internal/infra/journal/`. Wire full workflow end-to-end in `UpdateService`. Final live test on Fedora Rawhide host.
- **Tests**: Journal format validation tests; real runtime integration test.
- **Safety**: Audit trail persisted with zero secrets.

---

## 20. Files to Create/Modify

### New Files to Create:
- `internal/domain/transaction.go`: `TransactionArtifact`, `UpdatePlan`, `PlannedPackage`, `TransactionProgress`, `PostUpdateReport`.
- `internal/domain/planner.go`: `UpdatePlanner` interface.
- `internal/domain/executor.go`: `UpdateExecutor`, `TransactionEvent`, `TransactionVerifier`.
- `internal/infra/update/dnf5_planner.go`: Planner implementing `dnf5 upgrade --store`.
- `internal/infra/update/plan_parser.go`: Parser for `transaction.json`.
- `internal/infra/update/dnf5_executor.go`: Executor implementing `dnf5 replay`.
- `internal/infra/update/dnf5_verifier.go`: History correlation & `needs-restarting`.
- `internal/infra/privilege/sudo_auth.go`: `SudoAuthorizationProvider`.
- `internal/infra/journal/file_journal.go`: Audit log writer.
- `internal/ui/update/`: Update review modal, progress sheet, and completion components.

### Existing Files to Modify:
- `internal/app/update_service.go`: Extend to orchestrate Phase 2B transaction lifecycle.
- `internal/ui/app_model.go`: Integrate update review modal and execution overlay into root event loop.
- `project/STATUS.md`, `project/ROADMAP.md`: Update phase tracking once implemented.

---

## 21. Risks and Unresolved Questions

1. **Storage Consumption in `$XDG_RUNTIME_DIR`**:
   - *Risk*: On systems with very large updates (e.g. multi-gigabyte package sets), `$XDG_RUNTIME_DIR` (tmpfs) might run low on space.
   - *Mitigation*: Fall back to `~/.cache/setupforge/transactions/` if runtime dir disk space is insufficient.
2. **DNF5 History Table Race Condition**:
   - *Question*: Could an external background daemon (e.g. `dnf5daemon-server`) execute an update at the exact second SetupForge finishes?
   - *Resolution*: Solved by the correlation algorithm which checks `command_line` matching `replay <artifact_path>` in addition to timestamp and package count.

---

## 22. Explicit Non-Goals

1. **No Package Selection / Deselection**: Phase 2B is a whole-system update. Individual package toggle is deferred to Phase 3 (Package Management).
2. **No Arbitrary Command Execution**: SetupForge will not execute user-provided scripts or commands.
3. **No Automatic Rollback**: DNF does not provide atomic rollback for partial RPM failures. SetupForge will not promise impossible rollback guarantees.
4. **No Multi-Distro Implementations**: Architecture remains strictly Fedora-first. No Debian/Arch stubs.

---

## 23. Acceptance Criteria

1. **Exact Replay Guarantee**: Executed transaction matches reviewed transaction with 100% fidelity.
2. **Zero Password Capture**: SetupForge never inspects or captures sudo passwords.
3. **Truthful Progress**: Progress reporting reflects actual step counts without synthetic timers or fake 100% completions.
4. **Non-Cancellable RPM Phase**: Active RPM mutations cannot be interrupted by UI keys.
5. **Authoritative Reboot**: Reboot recommendations derive exclusively from `dnf5 needs-restarting` (handling exit code 1 correctly).
6. **Zero Host Mutation During Tests**: Complete unit suite runs unprivileged with zero host modifications.
7. **Responsive UI**: All review, progress, and completion views render cleanly across 80×24, 100×30, 120×35, and 160×45+.
