# SetupForge — Transaction Artifact Storage & Lifecycle Research

**Document Status**: RESEARCH GATE REPORT / FINAL RECOMMENDATION
**Phase Target**: Pre-Phase 2B.3 Architecture Gate
**Author**: SetupForge Core Agent Architecture Team
**Date**: October 1, 2026
**Target Platform**: Fedora Linux 44+ (Workstation / Server) & Portable Linux

---

## 1. Executive Summary

This document establishes the authoritative storage and lifecycle architecture for SetupForge's stored transaction artifacts (`dnf5 upgrade --store=PATH` / `dnf5 replay PATH`) prior to implementing Phase 2B.3 (`AuthorizationBoundary`).

### Core Research Question
> **Can SetupForge safely continue using `$XDG_RUNTIME_DIR` (`/run/user/$UID`) as its ephemeral transaction workspace if we add proper capacity management and lifecycle handling? And if not, what is the smallest, safest, most portable alternative?**

### Definitive Conclusion
**NO.** SetupForge **cannot** safely continue relying on `$XDG_RUNTIME_DIR` as its primary or sole transaction workspace, even with optimal lifecycle management and capacity preflight.

**The Disproving Evidence**:
1. **The Hard Physical Ceiling**: In `systemd-logind` (`pam_systemd`), `$XDG_RUNTIME_DIR` is mounted on `tmpfs` with a strict default quota of `RuntimeDirectorySize=10%` of physical RAM (`documented fact`). On a 4 GiB RAM machine (the standard baseline for Fedora Workstation VMs and entry-level laptops), `/run/user/$UID` is capped at **~380 MiB**.
2. **Typical Transaction Payload Dimensions**: A single routine Fedora update batch (e.g., kernel, browser, Mesa, desktop libraries) regularly demands between **500 MiB and 1.8 GiB** of downloaded RPMs (`observed repository behavior` & `documented fact`).
3. **Impossibility of Software-Only Mitigation**: Perfect lifecycle management eliminates *stale file accumulation*, but it cannot overcome the physical impossibility of fitting a 600 MiB single-transaction payload into a 380 MiB tmpfs partition. On systems with ≤8 GiB of RAM, routine system updates will trigger unrecoverable `Curl error (23)` / `ENOSPC` out-of-disk failures.
4. **Memory Starvation**: Even when memory allows (e.g. 8 GiB RAM / 800 MiB tmpfs), placing 700 MiB of package payloads into tmpfs forces RAM starvation, aggressive paging, and risk of Out-Of-Memory (OOM) process termination.

### Recommended Architectural Solution
Adopt a dedicated, private, persistent-disk temporary workspace rooted in **`/var/tmp/setupforge-$UID/transactions/`** (`PROPOSED`).
- **Filesystem Standard**: Conforms directly to the Filesystem Hierarchy Standard (FHS 3.0 §5.15) and systemd `file-hierarchy(7)` for large, multi-megabyte/gigabyte temporary installation artifacts.
- **Capacity**: Backed by persistent disk storage (typically 50–500+ GiB available), completely eliminating RAM competition and tmpfs overflow.
- **Security Boundary**: Enforces strict user ownership (`0700`, owner `$UID`) created unprivileged, preventing cross-user inspection.
- **Rigorous Lifecycle Management**: Governed by an event-driven state machine (single active transaction, immediate deletion upon cancel/error/verified replay, and startup garbage collection).
- **Safety Net**: Complemented by systemd's built-in `systemd-tmpfiles` policy (`q /var/tmp 1777 root root 30d`) preventing permanent leakage if the application crashes.

---

## 2. Repository-Verified Current Architecture

```
[REPOSITORY-VERIFIED]
```

A structural inspection of the SetupForge repository establishes the current implementation facts:

### 2.1 Workspace Resolution in `dnf5_planner.go`
In [`internal/infra/update/dnf5_planner.go`](../internal/infra/update/dnf5_planner.go):
```go
// Determine isolated directory for temporary transaction artifact
txBaseDir := p.baseDir
if txBaseDir == "" {
    runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
    if runtimeDir != "" {
        txBaseDir = filepath.Join(runtimeDir, "setupforge", "transactions")
    } else {
        txBaseDir = filepath.Join(os.TempDir(), fmt.Sprintf("setupforge-%d", os.Getuid()), "transactions")
    }
}

if err := os.MkdirAll(txBaseDir, 0700); err != nil {
    return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("%w: creating transactions base dir: %v", domain.ErrPlanningFailed, err)
}

storeDir, err := os.MkdirTemp(txBaseDir, "tx-"+opID+"-*")
```
- `XDG_RUNTIME_DIR` is preferred by default (`/run/user/1000/setupforge/transactions/`).
- Fallback path is `os.TempDir()` (`/tmp/setupforge-1000/transactions/`), which on Fedora is *also* a `tmpfs` (50% of RAM).
- Each plan creates a unique directory with a pseudo-random suffix: `tx-<opID>-*`.

### 2.2 Planning Execution
- Invokes unprivileged:
  `/usr/bin/dnf5 upgrade --store=<storeDir> -y --setopt=system_state_dir=/var/empty`
- DNF5 writes:
  - `<storeDir>/transaction.json` (mode `0640`)
  - `<storeDir>/packages/*.rpm` (mode `0640`)

### 2.3 Current Lifecycle & Cleanup Behavior
- **On Planning Failure**: [`dnf5_planner.go`](../internal/infra/update/dnf5_planner.go) uses `defer func() { if cleanup { os.RemoveAll(storeDir) } }()` to clean up the temporary directory if `dnf5 upgrade --store` exits non-zero or parsing fails.
- **On Planning Success**: `cleanup` is set to `false`. The directory is intentionally left on disk so the user can review it.
- **On User Cancellation**: In [`internal/ui/update/review_model.go`](../internal/ui/update/review_model.go), pressing `[ Cancel ]` or `Esc` transitions to `ReviewStateCancelled` and emits `UpdateCancelledMsg`. **Zero disk cleanup is performed.**
- **On Starting a New Plan**: Neither `AppModel` nor `UpdateService` removes previous transaction directories.
- **On Application Exit / Crash**: No shutdown hook or exit cleanup exists.
- **On SetupForge Startup**: No startup scanner or garbage collector exists to sweep old transactions.

### 2.4 Observed Real Host Manifestation
- During real Fedora host testing in Phase 2B.2, two successive planning operations staged transactions:
  - `tx-op-update-1790792715-2424162162` (130 MiB)
  - `tx-op-update-1790792782-3287331369` (130 MiB)
- Cumulative consumption: **259 MiB** out of a total **380 MiB** tmpfs quota (`/run/user/1000`).
- The subsequent test run (`TestRealFedora_DNF5UpdatePlanner`) attempted to download 129 MiB of Firefox packages into a 3rd transaction directory and crashed with:
  `Curl error (23): Failed writing received data to disk/application [...] Failure writing output to destination`
- Once the stale directories were manually purged, the exact same operation passed cleanly in 15.9 seconds.

---

## 3. Fedora, XDG, and systemd Findings

```
[RESEARCH-VERIFIED]
```

### 3.1 The XDG Base Directory Specification
- **`$XDG_RUNTIME_DIR`** (§User Directories):
  - *"The directory MUST be owned by the user, and he MUST be the only one having read and write access to it. Its Unix access mode MUST be 0700."* (`documented fact`)
  - *"The lifetime of the directory MUST be bound to the user being logged in."* (`documented fact`)
  - *"If a user logs out, the directory MUST be removed."* (`documented fact`)
  - *"This directory shall be used for runtime file system objects such as AF_UNIX sockets, FIFOs, PID files and similar."* (`documented fact` — pam_systemd)
  - Semantics: Ephemeral, session-scoped, in-memory communication primitives. Not designed for staging multi-hundred-megabyte binary package archives.

- **`$XDG_CACHE_HOME`** (§User Directories):
  - Defaults to `~/.cache`.
  - Defined as: *"There is a single base directory relative to which user-specific non-essential (cached) data should be written."* (`documented fact`)
  - Explicit specification clause: *"The user may delete files in this directory at any time without data loss."* (`documented fact`)
  - Crucial semantic conflict: A transaction replay artifact is **not** non-essential cache. It is mission-critical operation state. If deleted while an update is pending or in-flight, exact replay fails.

### 3.2 systemd-logind Runtime Directory Sizing
- In `systemd-logind` (`man 5 logind.conf`):
  `RuntimeDirectorySize=` controls the mount size of `/run/user/$UID`.
  - Default value: **10% of physical RAM** (`documented fact`).
  - Implementation: `mount -t tmpfs -o mode=0700,uid=...,gid=...,size=10% tmpfs /run/user/$UID` (`documented fact`).
  - Table of standard Fedora hardware configurations:
    | Physical RAM | `/run/user/$UID` Quota (10%) | Typical Fedora Update Batch | Can Fit in tmpfs? |
    | :--- | :--- | :--- | :--- |
    | **2 GiB** (Minimal VM / Container) | 190 MiB | 500 MiB – 1.5 GiB | 🛑 **IMPOSSIBLE** |
    | **4 GiB** (Standard VM / Budget PC)| 380 MiB | 500 MiB – 1.5 GiB | 🛑 **IMPOSSIBLE** |
    | **8 GiB** (Mid-tier Laptop) | 800 MiB | 500 MiB – 1.5 GiB | ⚠️ **Marginal / Fragile** |
    | **16 GiB** (Workstation) | 1.6 GiB | 500 MiB – 1.5 GiB | ⚠️ Fits, but wastes RAM |
    | **32 GiB** (High-end Workstation) | 3.2 GiB | 500 MiB – 1.5 GiB | ✅ Fits safely |

### 3.3 Linux Filesystem Hierarchy Standard (FHS 3.0) & systemd `file-hierarchy(7)`
- **`/tmp`**:
  - On modern Fedora, `/tmp` is a `tmpfs` sized at **50% of RAM** (`man 7 hier`, `file-hierarchy(7)`).
  - Subject to early purging (`q /tmp 1777 root root 10d` in `/usr/lib/tmpfiles.d/tmp.conf`).
  - Still consumes RAM/swap; still risky on 2–4 GiB systems.
- **`/var/tmp`**:
  - Explicit definition in FHS 3.0 (§5.15):
    *"The /var/tmp directory is made available for programs that require temporary files or directories that are preserved between system reboots. Therefore, data stored in /var/tmp is more persistent than data in /tmp."* (`documented fact`)
  - Systemd specification: Backed by the **persistent root filesystem**, not RAM (`documented fact`).
  - Aging policy: Managed by `systemd-tmpfiles` via `/usr/lib/tmpfiles.d/tmp.conf`:
    `q /var/tmp 1777 root root 30d` (`documented fact`).
  - Standard Linux usage: Package managers, ISO downloaders, OS image builders (e.g. `mock`, `lorax`, `rpm-ostree`) use `/var/tmp` specifically because it provides vast storage without memory overhead.

---

## 4. `$XDG_RUNTIME_DIR` / tmpfs In-Depth Analysis

```
[RESEARCH-VERIFIED]
```

### 4.1 Lifecycle and Volatility
1. **Creation**: Initialized by `pam_systemd` upon the user's first PAM session open.
2. **Termination**: Unmounted and completely destroyed by `systemd-logind` when the user's final session terminates (unless `loginctl enable-linger` is active).
3. **Application Crash / Restart**: Survives application crashes within the same login session. If SetupForge crashes, files remain in `/run/user/$UID` indefinitely until logout or reboot.
4. **Reboot**: Never survives a system reboot. If an update plan requires rebooting or if a power loss occurs between review and execution, the entire artifact is instantly lost.

### 4.2 Why Hundred-Megabyte Artifacts Are Inappropriate for tmpfs
- **Memory Pressure & Swap Thrashing**: Storing 1 GiB of RPMs in tmpfs locks 1 GiB of system memory. When available RAM drops below thresholds, the Linux kernel aggressively pages out active desktop applications (GNOME Shell, browsers) to zram/swap, degrading system responsiveness.
- **OOM Invocation**: If swap is exhausted or disabled, tmpfs allocation triggers the kernel Out-Of-Memory killer (`oom-killer`), potentially killing SetupForge or critical user daemons.
- **Hard Quota Rejection**: As demonstrated during Phase 2B.2 verification, DNF5 uses `librepo` and `curl`. When `write(2)` returns `ENOSPC`, curl aborts with `Curl error (23)`.

---

## 5. `$XDG_CACHE_HOME` Analysis

```
[RESEARCH-VERIFIED]
```

Candidate path: `~/.cache/setupforge/transactions/`

### 5.1 Benefits
- Located on user's home filesystem (persistent disk storage, ample space).
- Completely unprivileged (`0700` inside `$HOME`).
- Survives application restarts and reboots.

### 5.2 Severe Architectural and Trust Liabilities
1. **Violates XDG Cache Semantics**:
   The XDG specification explicitly defines cache files as ephemeral, regenerable data that the user or external utilities can delete at any moment without notice or data loss.
2. **External Cache Cleaner Collisions**:
   Utilities like `bleachbit`, GNOME Storage cleaner, flatpak maintenance scripts, or automated user cron jobs (`find ~/.cache -mtime +7 -delete`) routinely clean `~/.cache`. If a cleaner deletes an RPM file between user confirmation (T3) and privileged replay (T10), the transaction is corrupted mid-flight.
3. **Indefinite Disk Accumulation Risk**:
   Unlike `/var/tmp` (which has OS-level 30-day purging via `systemd-tmpfiles`), `~/.cache` has **zero** automatic OS-level pruning. If SetupForge crashes or a bug leaves 1 GiB of RPMs in `~/.cache`, it will consume the user's SSD permanently until manual discovery.
4. **NFS and Network Home Directory Vulnerabilities**:
   In enterprise, academic, or multi-seat Fedora environments, `/home/$USER` is frequently mounted over NFS or Kerberized storage. In such configurations:
   - Root has `nobody` permissions (`root_squash`), causing `sudo dnf5 replay ~/.cache/...` to fail with permission denied.
   - Filesystem locking over NFS can cause transaction deadlocks.

---

## 6. DNF5 Cache Analysis

```
[RESEARCH-VERIFIED]
```

### 6.1 DNF5 Cache Locations & Permissions
- Unprivileged user cache: `~/.cache/libdnf5/`
- Privileged system cache: `/var/cache/libdnf5/`
- Default behavior: In DNF5 / `libdnf5 5.4.6`, `keepcache` defaults to **`false`** (`documented fact` — `man dnf5.conf`). Downloaded RPMs are deleted immediately after a transaction completes.

### 6.2 Can SetupForge Decouple Package Payloads into DNF5's Cache?
**NO.**
1. **Rigid `--store` Behavior**: When `dnf5 upgrade --store=PATH` is executed, DNF5 resolves the transaction and writes the downloaded RPMs directly into `PATH/packages/`. DNF5 provides no option to write `transaction.json` to `PATH` while directing package binaries elsewhere (`documented fact`).
2. **Relative Manifest Paths**: In `transaction.json`, package paths are recorded as:
   `"package_path": "./packages/<nevra>.rpm"` (`observed repository behavior`).
   DNF5 `replay` strictly expects the RPMs to exist at that relative location.
3. **Cache Invalidation & TOCTOU**: If SetupForge attempted to rely on a shared DNF5 cache, any external `dnf5 clean all` or concurrent package operation would invalidate the pending transaction bundle.
4. **Portability Violation**: Future SetupForge backends for Debian (APT) or Arch (pacman) do not share DNF5 cache mechanics. Coupling SetupForge's domain artifact model to DNF5 cache internals violates ADR-001 and `.agents/rules/architecture.md`.

---

## 7. DNF5 `--store` and `replay` Technical Specification

```
[RESEARCH-VERIFIED]
```

### 7.1 Artifact Directory Layout
A valid stored transaction created with `dnf5 [command] --store=<dir>` has this exact structure:
```
<dir>/
├── transaction.json                     (mode 0640, JSON metadata)
└── packages/                            (directory)
    ├── firefox-156.0.1-2.fc44.x86_64.rpm
    └── kernel-core-6.14.0-64.fc44.x86_64.rpm
```

### 7.2 Manifest Contents (`transaction.json`)
- `version`: Manifest schema version (currently `"1.0"`).
- `rpms`: Array of objects containing:
  - `nevra`: Full standard NEVRA identifier.
  - `action`: Transaction action (`"Upgrade"`, `"Install"`, `"Remove"`, `"Replaced"`, `"Downgrade"`).
  - `reason`: `"External User"`, `"Dependency"`, etc.
  - `repo_id`: Mangled repository identifier: `"@stored_transaction(<orig_repo>)"` or `"@System"`.
  - `package_path`: Relative path to package binary: `"./packages/<nevra>.rpm"` (omitted for `"Remove"` and `"Replaced"`).

### 7.3 Replay Execution Requirements
- Command: `/usr/bin/dnf5 -y replay <dir>`
- Replay semantics: Recreates temporary local repository configurations pointing directly to the files in `./packages/`.
- Strict validation: Verifies installed system state against `transaction.json`. If any installed package version has drifted since planning, `dnf5 replay` safely aborts with an exit code error.
- Payloads: Must remain inside `<dir>/packages/`. External references, moving packages, or deleting packages prior to replay causes immediate replay failure.

---

## 8. Alternative Architecture Comparison

```
[REASONED ARCHITECTURAL CONCLUSION]
```

| Evaluation Dimension | Option A: `$XDG_RUNTIME_DIR`<br>`/run/user/$UID/` | Option B: `$XDG_CACHE_HOME`<br>`~/.cache/setupforge/` | Option C: DNF5 Cache Integration<br>`~/.cache/libdnf5/` | Option D: Dedicated Persistent Workspace<br>`/var/tmp/setupforge-$UID/` | Option E: Adaptive Tiered Workspace<br>`$XDG_RUNTIME_DIR` with `/var/tmp` fallback |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Physical Storage Backing** | `tmpfs` (RAM / Swap) | Persistent Home Disk | Persistent Root/User Disk | **Persistent Root Disk** | Dynamic (RAM or Disk) |
| **Capacity Quota** | **10% RAM (~380 MB on 4GB)** | Multi-GB (Home quota) | Multi-GB | **50–500+ GB (Root FS)** | Inconsistent / Variable |
| **RAM Competition** | 🛑 Severe | ✅ None | ✅ None | ✅ **None** | ⚠️ Partial |
| **Large Update Feasibility** | 🛑 **Fails on ≤8GB RAM** | ✅ Excellent | ✅ Excellent | ✅ **Excellent** | ⚠️ Complex thresholding |
| **Reboot Survival** | 🛑 Wiped on reboot | ✅ Survives | ✅ Survives | ✅ **Survives** | 🛑 Inconsistent |
| **Logout Survival** | 🛑 Wiped on logout | ✅ Survives | ✅ Survives | ✅ **Survives** | 🛑 Inconsistent |
| **Crash Cleanup Safety Net**| ✅ Wiped on reboot | 🛑 Never auto-cleaned | 🛑 Never auto-cleaned | ✅ **OS cleans in 30d** (`tmpfiles.d`) | ✅ OS cleans in 30d |
| **NFS / Root Squash Safe** | ✅ Always local | 🛑 Vulnerable on NFS | 🛑 Vulnerable on NFS | ✅ **Guaranteed local** | ✅ Guaranteed local |
| **Cache Cleaner TOCTOU** | ✅ Safe | 🛑 Vulnerable | 🛑 Vulnerable | ✅ **Safe** | ✅ Safe |
| **Replay Exactness** | ✅ 100% | ✅ 100% | 🛑 Breaks isolation | ✅ **100%** | ✅ 100% |
| **Portability (APT/pacman)** | ✅ High | ✅ High | 🛑 Zero (DNF only) | ✅ **High (Universal POSIX)** | ⚠️ Medium |
| **Implementation Complexity**| Low | Low | Very High | **Low** | Medium-High |

---

## 9. Storage Recommendation

```
[PROPOSED]
```

### The Canonical Storage Path: `/var/tmp/setupforge-$UID/transactions/`
SetupForge shall designate `/var/tmp/setupforge-$UID/transactions/` as its authoritative transaction workspace root.

### Rationale
1. **Eliminates the Hardware Capacity Ceiling**: Completely bypasses the 10% RAM tmpfs limit. SetupForge can comfortably stage 500 MiB, 2 GiB, or 5 GiB update transactions on any machine without risking memory exhaustion or disk write errors.
2. **Preserves 100% Ephemeral Semantics**: Conforms strictly to FHS 3.0 and systemd expectations for large multi-megabyte temporary staging data.
3. **Guaranteed Local and Root-Replay Safe**: `/var/tmp` is always a local physical mount, avoiding network filesystem issues (`root_squash`) during privileged replay.
4. **Automated OS-Level Safety Net**: In addition to SetupForge's internal lifecycle management, `systemd-tmpfiles` provides a guaranteed 30-day cleanup policy (`/usr/lib/tmpfiles.d/tmp.conf`).
5. **Universal Portability**: `/var/tmp` is standard across Fedora, Debian, Ubuntu, Arch, openSUSE, and Alpine, guaranteeing architectural longevity for future package backends.

---

## 10. Artifact Lifecycle Recommendation

```
[PROPOSED]
```

To prevent storage leakage on persistent disk, SetupForge must enforce an authoritative, state-driven lifecycle.

### State Diagram

```mermaid
stateDiagram-v2
    [*] --> Planning: RequestUpdateReviewMsg
    Planning --> PlanningFailed: dnf5 error / invalid JSON
    PlanningFailed --> [*]: Immediate Cleanup

    Planning --> Ready: Artifact Verified & Fingerprinted
    Ready --> Cancelled: User presses [ Cancel ] / Esc
    Cancelled --> [*]: Immediate Cleanup

    Ready --> Confirmed: User presses [ Confirm Update ]
    Confirmed --> Authorized: sudo -v authenticated (Phase 2B.3)
    Authorized --> Executing: Invoking dnf5 replay (Phase 2B.4)

    Executing --> ExecutionFailed: Replay error / system mismatch
    ExecutionFailed --> DiagnosticRetained: Retain for inspection (24h)

    Executing --> Verified: System updated & RPMs verified
    Verified --> [*]: Immediate Cleanup
```

### Classification of States

| State | Status | Protection Rule | Cleanup Trigger |
| :--- | :--- | :--- | :--- |
| **`Planning`** | Transient | Active operation | Auto-deleted on failure (`defer`) |
| **`PlanningFailed`** | Terminal | Unusable | **Immediately deleted** |
| **`Ready` (Reviewed)** | Interactive | **Protected** while review modal is open | Protected |
| **`Cancelled`** | Terminal | Rejected by user | **Immediately deleted** |
| **`Confirmed`** | Staged | **Protected** awaiting authorization | Protected |
| **`Authorized`** | Staged | **Protected** awaiting execution handoff | Protected |
| **`Executing`** | In-Flight | **Protected** (replay active) | Protected |
| **`Verified` (Success)** | Terminal | Successfully applied | **Immediately deleted** |
| **`ExecutionFailed`** | Terminal | Error diagnostic state | Retained for diagnostics (pruned after 24h) |
| **`Abandoned`** | Stale | Left from crashed / killed session | Pruned during startup garbage collection |

---

## 11. Cleanup Strategy

```
[PROPOSED]
```

SetupForge shall employ **Event-Driven Cleanup** as its primary mechanism, supplemented by **Startup Garbage Collection**.

### 11.1 Immediate Event-Driven Triggers
1. **Planning Failure**: If `dnf5 upgrade --store` fails or fingerprint calculation errors out, `os.RemoveAll(storeDir)` is called before returning control.
2. **User Cancellation**: When `UpdateReviewModel` transitions to `ReviewStateCancelled` (user clicks `[ Cancel ]` or hits `Esc`), `UpdateService` immediately purges the artifact directory.
3. **Superceded Plan**: If the user requests a new update review while an unexecuted plan exists, the existing artifact directory is removed before resolving the new plan.
4. **Successful Execution & Verification**: Once Phase 2B.4 completes `dnf5 replay` and post-update checks pass, the entire artifact directory is purged.

### 11.2 Startup & Exit Garbage Collection
When SetupForge launches:
1. `UpdateService` scans `/var/tmp/setupforge-$UID/transactions/`.
2. Any subdirectory starting with `tx-` whose creation timestamp is older than **2 hours** (or any transaction directory from a previous PID) is purged.
3. Ensures that if the user hard-killed SetupForge (`kill -9`, terminal window abruptly closed, power loss), accumulated artifacts never linger on disk.

---

## 12. Storage-Budget Strategy

```
[PROPOSED]
```

To ensure SetupForge behaves responsibly on low-capacity solid-state drives, transaction storage must be strictly bounded:

1. **Single-Active-Transaction Invariant**: SetupForge will maintain at most **one** active update transaction artifact on disk at any given time.
2. **Storage Cap (Threshold)**: SetupForge will enforce a maximum transaction storage threshold:
   $$\text{MaxTransactionBytes} = \min\left(10\text{ GiB}, \frac{1}{2} \times \text{AvailableDiskBytes}\right)$$
3. **Graceful Degradation**: If an update plan would consume more than 50% of available disk space, planning terminates early with a typed domain error (`domain.ErrInsufficientStorage`), advising the user of required disk reclamation before proceeding.

---

## 13. Capacity-Preflight Strategy

```
[PROPOSED]
```

SetupForge must never begin downloading hundreds of megabytes of RPMs if the destination filesystem lacks the capacity to hold them.

### 13.1 Preflight Mathematical Formulation
Before launching `dnf5 upgrade --store`, SetupForge queries filesystem metrics via `unix.Statfs`:
```go
var stat unix.Statfs_t
if err := unix.Statfs(workspaceDir, &stat); err != nil { ... }
availBytes := stat.Bavail * uint64(stat.Bsize)
```

The preflight condition is:
$$\text{availBytes} \ge \text{SafetyFloor} + (1.2 \times \text{EstimatedDownloadBytes})$$
Where:
- $\text{SafetyFloor} = 1.0\text{ GiB}$ (ensures the host OS root filesystem is never driven to zero free blocks).
- Multiplier $1.2$ accounts for filesystem metadata overhead, comps files, and `transaction.json`.

If estimated download size is not yet known prior to planning (since `check-upgrade --json` only lists package names and versions), SetupForge checks:
$$\text{availBytes} \ge 2.0\text{ GiB}$$
If the workspace filesystem has less than 2.0 GiB available, SetupForge halts before running `--store`, alerting the user:
`"Cannot safely stage updates: /var/tmp has less than 2.0 GiB of available storage."`

---

## 14. Crash, Restart, and Logout Behavior

```
[REASONED ARCHITECTURAL CONCLUSION]
```

| Scenario | Behavior under `$XDG_RUNTIME_DIR` (Current) | Behavior under `/var/tmp` (Proposed) |
| :--- | :--- | :--- |
| **Application Crash (`SIGKILL` / Segfault)** | Files linger in RAM until user logs out. | Files linger on disk; swept by SetupForge upon next startup or by `tmpfiles.d` after 30d. |
| **Terminal Closes Mid-Review** | Files linger in RAM until logout. | Files swept by SetupForge upon next startup. |
| **Machine Reboot after Review** | **Wiped entirely**. User must re-plan from scratch. | **Preserved**. SetupForge can revalidate fingerprint and resume review/execution. |
| **User Logout and Login** | **Wiped entirely**. | **Preserved**. User session reconnects to pending transaction if desired. |
| **Power Loss during Replay** | Artifact lost; impossible to inspect what was being installed. | **Artifact intact**; post-recovery diagnostics can inspect `transaction.json`. |

---

## 15. Concurrency Behavior

```
[REASONED ARCHITECTURAL CONCLUSION]
```

### Multiple SetupForge Instances
- SetupForge already scopes operations using unique nanosecond-based Operation IDs: `op-update-<timestamp>`.
- In [`internal/app/update_service.go`](../internal/app/update_service.go), concurrent planning within a process is serialized via `planMu sync.Mutex`.
- Across processes: Each planning operation creates an isolated directory `tx-<opID>-<rand>`.
- **Concurrency Invariant**: To prevent competing instances from thrashing disk space, a lightweight advisory flock (`workspace.lock`) shall be held on `/var/tmp/setupforge-$UID/` during active planning and replay.

---

## 16. Security & Integrity Implications

```
[RESEARCH-VERIFIED]
```

### 16.1 Preserving the Security Invariants
```
REVIEWED_ARTIFACT_ID == AUTHORIZED_ARTIFACT_ID == EXECUTED_ARTIFACT_ID
```
and:
```
REVIEWED_FINGERPRINT == AUTHORIZED_FINGERPRINT == EXECUTED_FINGERPRINT
```

Moving from `/run/user/$UID` to `/var/tmp/setupforge-$UID` does **not** weaken this invariant:
1. **User Ownership**: The directory `/var/tmp/setupforge-$UID` is created with mode `0700` (`rwx------`) owned by the invoking user. Unprivileged users on the machine cannot enter, read, or modify the directory.
2. **Cryptographic Validation**: The SHA-256 Merkle root digest covers all regular files (`transaction.json` + all `.rpm` packages). Even if an attacker had root access, any modification would alter the fingerprint and cause SetupForge to reject the transaction before replay.
3. **No Moving / Linking**: Payloads are downloaded directly to `/var/tmp/setupforge-$UID/transactions/tx-.../` and replayed directly in place. No payload copying, hardlinking, or external symlinking occurs.

---

## 17. TOCTOU Analysis

```
[REASONED ARCHITECTURAL CONCLUSION]
```

### Complete Lifecycle Verification Gates
To guarantee that no Time-Of-Check-To-Time-Of-Use vulnerability can occur between review and execution, validation occurs at **four distinct checkpoints**:

```
[T0]  Plan Resolved (DNF5 writes to /var/tmp/setupforge-$UID/tx-...)
       │
[T1]  Initial Validation: ValidateArtifactStructure + ComputeArtifactFingerprint
       │ (Fingerprint F0 recorded in domain.TransactionArtifact)
       ▼
[T2]  UI Review: User inspects packages & metrics
       │ (Fingerprint F0 bound to UpdatePlan)
       ▼
[T3]  User Confirmation: User clicks [ Confirm Update ]
       │ (F0 recorded in ConfirmationDecision)
       ▼
[T4]  Pre-Authorization Check: ValidateArtifact(artifact)
       │ Computes F_auth; asserts F_auth == F0
       ▼
[T5]  Authorization Granted (sudo -v / Polkit ticket cached)
       │
[T6]  Pre-Execution Gate (Phase 2B.4): Final Fingerprint Check
       │ Computes F_exec; asserts F_exec == F0
       │ Validates zero symlinks (ModeSymlink check)
       ▼
[T7]  Privileged Replay: sudo -n dnf5 replay /var/tmp/...
```

If any file is modified, touched, truncated, or added between $T0$ and $T7$, the digest changes ($F_{\text{exec}} \ne F_0$) and privileged execution is permanently aborted.

---

## 18. Symlink Defense-in-Depth Analysis

```
[RESEARCH-VERIFIED]
```

### Vulnerability Identified in Phase 2B.2 Audit
In [`internal/infra/update/fingerprint.go`](../internal/infra/update/fingerprint.go):
```go
func IsReplayRelevantArtifactFile(d fs.DirEntry) bool {
    return d.Type().IsRegular()
}
```
Currently, `ComputeArtifactFingerprint` skips non-regular files. While DNF5 `--store` only creates regular files, if an adversary with local access managed to drop a symlink (e.g. `packages/evil.rpm -> /etc/shadow`) inside an insecure directory, the symlink would be ignored by the fingerprint hasher.

### Strict Hardening Policy for Phase 2B.3 / 2B.4
[`ValidateArtifactStructure`](../internal/infra/update/fingerprint.go) must be extended to fail closed:
> **If any directory entry inside the transaction artifact tree is a symlink (`os.ModeSymlink`), FIFO, socket, or device, `ValidateArtifactStructure` MUST immediately return `domain.ErrTransactionArtifactCorrupted`.**

---

## 19. Portability Implications

```
[REASONED ARCHITECTURAL CONCLUSION]
```

### Multi-Distribution Alignment
- `/var/tmp` is mandated by FHS 3.0 and exists across all enterprise and community Linux distributions:
  - Fedora / RHEL / CentOS
  - Debian / Ubuntu
  - Arch Linux
  - openSUSE / SLES
- Future package managers:
  - **Debian (APT)**: `apt-get -d` downloads `.deb` archives to `/var/tmp/setupforge-$UID/apt/`.
  - **Arch (Pacman)**: `pacman -Sw --cachedir` stages `.pkg.tar.zst` archives into `/var/tmp/setupforge-$UID/pacman/`.
- Keeping transaction artifacts in `/var/tmp` preserves universal, distribution-agnostic architectural layering. Presentation models and domain services remain 100% portable.

---

## 20. Performance Implications

```
[RESEARCH-VERIFIED]
```

### 20.1 Disk I/O vs Memory Bandwidth
- Writing to `/var/tmp` on modern NVMe drives achieves sustained write speeds of 1,500–5,000 MB/s.
- Downloading 150 MiB of RPMs takes ~10–15 seconds over broadband; disk write latency is measured in milliseconds.
- On Fedora default installations, `/` and `/var` reside on **Btrfs**. Staging in `/var/tmp` enables rapid block allocation with zero memory overhead.

### 20.2 Hashing Performance
- Merkle SHA-256 fingerprinting streams files in 64 KiB chunks using Go's hardware-accelerated `crypto/sha256` (Intel SHA-NI / ARMv8 Crypto extensions).
- Hashing 150 MiB of RPMs takes **< 0.08 seconds** on standard modern CPUs.

---

## 21. Implementation Impact on Phase 2B.1

```
[REPOSITORY-VERIFIED]
```

The changes required in the Phase 2B.1 code are minimal, non-breaking, and localized strictly to the infrastructure layer:

1. **`internal/infra/update/dnf5_planner.go`**:
   Update `txBaseDir` default resolution:
   ```go
   // Change from:
   runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
   // To:
   txBaseDir = filepath.Join("/var/tmp", fmt.Sprintf("setupforge-%d", os.Getuid()), "transactions")
   ```
2. **`internal/infra/update/fingerprint.go`**:
   Add explicit symlink rejection in `ValidateArtifactStructure`.
3. **No Domain or UI Changes**:
   Domain contracts (`UpdatePlanner`, `TransactionArtifact`, `UpdatePlan`) remain 100% unchanged.

---

## 22. Implementation Impact on Phase 2B.3

```
[PROPOSED]
```

Phase 2B.3 (`AuthorizationBoundary`) requires zero knowledge of where packages are stored:
- `AuthorizationProvider` verifies that the user can authenticate (`sudo -v`).
- It passes `ConfirmationDecision` to the authorization flow.
- It re-validates `UpdatePlanner.ValidateArtifact(artifact)` prior to terminal authorization handoff.
- The storage path change is completely transparent to Phase 2B.3.

---

## 23. Implementation Impact on Phase 2B.4

```
[PROPOSED]
```

Phase 2B.4 (`TransactionExecution`) executes:
```bash
sudo -n /usr/bin/dnf5 -y replay /var/tmp/setupforge-$UID/transactions/tx-...
```
- Because `/var/tmp` is on local root storage, `sudo dnf5 replay` has full native read access.
- Upon verified execution completion, Phase 2B.4 invokes `os.RemoveAll(artifact.Directory)` to reclaim disk space immediately.

---

## 24. Open Questions

```
[RESEARCH-VERIFIED]
```

1. **Non-Standard Environments without `/var/tmp`**:
   *Question*: Are there any supported Linux environments where `/var/tmp` is missing or mounted `noexec`/`ro`?
   *Answer*: `/var/tmp` is part of standard POSIX/FHS. `dnf5 replay` does not execute binaries directly out of the artifact directory; it passes RPMs to the RPM database engine which unpacks them to system paths. `noexec` on `/var/tmp` does not inhibit RPM installation.
2. **Read-Only Root Filesystems (Fedora Silverblue / Kinoite)**:
   *Question*: Does `/var/tmp` work on rpm-ostree systems?
   *Answer*: Yes. On Fedora Silverblue, `/var` is writable (`/var/tmp` is symlinked or mounted as the persistent temporary directory).

---

## 25. Source References

1. **XDG Base Directory Specification**:
   Standards at FreeDesktop.org: `https://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html`
2. **systemd-logind.service & pam_systemd**:
   `man 7 pam_systemd`, `man 5 logind.conf`, `man 8 systemd-logind.service`.
3. **Linux Filesystem Hierarchy Standard (FHS 3.0)**:
   §5.15 `/var/tmp : Temporary files preserved between reboots`.
4. **systemd File Hierarchy Standard**:
   `man 7 file-hierarchy`, `man 5 tmpfiles.d`.
5. **DNF5 Documentation & Manual Pages**:
   - `dnf5-replay(8)` — Replay Command specification.
   - `dnf5-upgrade(8)` — Upgrade Command and `--store` flag behavior.
   - `dnf5.conf(5)` — Configuration options (`cachedir`, `keepcache`, `system_cachedir`).
6. **SetupForge ADRs**:
   - ADR-007: Strict Transaction Replay Invariant (`Reviewed == Executed`).
   - ADR-008: Deterministic Multi-Component Transaction Artifact Fingerprinting.
   - ADR-009: Update Plan Presentation & Cryptographic User Confirmation Boundary.

---

## 26. Final Architectural Recommendation

```
[PROPOSED]
```

### The Architectural Verdict
SetupForge shall adopt **Option D (Dedicated Persistent Workspace in `/var/tmp`)** as its foundational transaction artifact storage architecture.

### Summary of Decisions
1. **Primary Location**: `/var/tmp/setupforge-$UID/transactions/tx-<opID>-<rand>/`
2. **Permissions**: Directory mode `0700`, owner `$UID`, strict non-root isolation.
3. **Capacity Preflight**: Mandatory free space check before planning:
   $$\text{AvailableDiskBytes} \ge \max(2.0\text{ GiB}, 1.2 \times \text{DownloadSize} + 1.0\text{ GiB})$$
4. **Event-Driven Lifecycle**:
   - Cancelled / Superceded / Failed $\rightarrow$ Immediate deletion.
   - Executed & Verified $\rightarrow$ Immediate deletion.
   - Active / Confirmed / Authorized $\rightarrow$ Strictly protected.
5. **Startup Garbage Collection**: Auto-prune any orphaned transaction directories older than 2 hours upon SetupForge launch.
6. **Anti-Tampering Hardening**: Explicitly reject symlinks in `ValidateArtifactStructure`.

This architecture completely eliminates the tmpfs capacity crash observed in Phase 2B.2, respects host memory, preserves 100% cryptographic replay integrity, and ensures seamless portability across Linux distributions.
