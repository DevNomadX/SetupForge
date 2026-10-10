# Current Work

## Task
Phase 2B.4 — Controlled Vertical Slice 1: Privileged Execution Boundary, Exact Artifact Revalidation, Durable EXECUTING & Sudo Replay Dispatch.

## Status
Implemented, Integrated, and Verified on Real Fedora Host.

## Implementation Overview
1. **Domain Capability Contracts & Typed Errors** (`internal/domain/transaction.go`):
   - Added typed errors: `ErrAuthorizationFailed` (non-interactive privilege verification failed) and `ErrArtifactMismatch` (artifact identity or fingerprint mismatch against user confirmation decision).
   - Added domain structs: `PrivilegedCommand` (discrete executable vector, args, env, dir without shell interpolation) and `ProcessResult` (exit code, normal exit bool, signal info, cancellation flags, captured stdout/stderr, duration).
   - Added domain capability interfaces: `AuthorizationProvider` (`Authorize(ctx) error`) and `PrivilegedRunner` (`Run(ctx, cmd) (ProcessResult, error)`).
   - Updated `ValidateTransition`: enabled valid transition `CONFIRMED -> EXECUTING` in addition to `AUTHORIZED` (since authorization is an in-memory execution precondition, journal transitions directly from `CONFIRMED` to durable `EXECUTING`).
2. **Infrastructure Authorization Provider** (`internal/infra/privilege/sudo_auth.go`):
   - Implemented `SudoAuthorizationProvider` providing non-interactive `sudo -n true` preflight.
   - Enforces root bypass (`geteuid() == 0`).
   - Zero password handling in the UI; returns `ErrAuthorizationFailed` immediately if cached credentials are missing.
3. **Infrastructure Process Group Runner & Signal Escalation** (`internal/infra/privilege/sudo.go`):
   - Upgraded `SudoRunner` with `ExecuteWithProcessGroup`: Linux process group isolation (`Setpgid: true`), sanitized environment (`PATH=/usr/bin:/bin:/usr/sbin:/sbin`, `LC_ALL=C.UTF-8`), `/dev/null` stdin.
   - Bounded signal escalation upon context cancellation: `SIGINT` (30s grace) -> `SIGTERM` (10s grace) -> `SIGKILL` last resort.
   - Invariant enforced: Signal handling does NOT establish transaction success or failure; it merely bounds process lifetime.
4. **DNF5 Replay Command Construction** (`internal/infra/update/replay_command.go`):
   - Implemented `BuildDNF5ReplayCommand` constructing discrete argument vector:
     `/usr/bin/dnf5 --comment="setupforge:<opID>" -y replay <artifactDir>`
   - Rejects relaxation flags (`--ignore-installed`, `--skip-broken` strictly prohibited).
5. **Application Service Execution Orchestration** (`internal/app/update_service.go`):
   - Added functional options: `WithAuthorizationProvider`, `WithPrivilegedRunner`, `WithDNF5Path`, `WithReplayCommandBuilder`.
   - Implemented `ExecuteActiveTransaction(ctx, decision)`:
     1. Verifies advisory engine lock is held.
     2. Verifies user confirmation decision is explicitly confirmed (`ConfirmationStatusConfirmed`).
     3. Asserts active transaction journal precondition state is `CONFIRMED`.
     4. Enforces Check B exact artifact revalidation: `decision.OperationID == active.OperationID`, `decision.ArtifactDirectory == active.ArtifactDirectory`, `decision.ArtifactFingerprint == active.ArtifactFingerprint`, `active.ArtifactState == SEALED`, and physical disk integrity (`store.Validate`).
     5. Enforces authorization boundary check via `authProvider.Authorize`. If failed, transitions journal to `FAILED` and halts; NEVER writes `EXECUTING`.
     6. Persists durable `EXECUTING` state to journal before spawning the child process.
     7. Dispatches privileged runner with structured replay command.
     8. Captures raw `ProcessResult`. Exit code 0 DOES NOT equal `COMPLETED`. The transaction remains in `EXECUTING` state in the journal until Vertical Slice 2 DNF5 history reconciliation.
6. **Main Wiring** (`cmd/setupforge/main.go`):
   - Wires `SudoAuthorizationProvider` and `SudoRunner` into `UpdateService`.

## Verification Summary
- `gofmt -l .`: 0 formatting violations.
- `go vet ./...`: 0 static analysis issues.
- `go test -v -count=1 ./...`: 100% pass across all unit and integration tests (domain, detector, privilege, update, storage, journal, app, ui, components, layout, theme).
- Unit Tests:
  - `internal/infra/privilege/sudo_auth_test.go`: Root bypass, non-root success, password required failure (`ErrAuthorizationFailed`), missing sudo, context cancellation, non-blocking real execution.
  - `internal/infra/privilege/sudo_test.go`: User execution, root execution, non-interactive flag, empty executable, missing sudo, exit code preservation, raw executor, process group execution, cancellation escalation.
  - `internal/app/update_service_execution_test.go`: Execution success, engine lock guard, unconfirmed decision rejection, invalid state precondition, artifact ID/directory/fingerprint mismatch, artifact on-disk tampering, authorization failure (sets FAILED, avoids EXECUTING, suppresses runner), journal write failure halts execution, runner failure preservation.
- Real Fedora Integration (`TestRealFedora_ControlledExecutionSlice1`): Verified on Fedora Linux 44 Workstation (`dnf5 5.4.6.0`). Verified full pipeline from sealed artifact creation in `/var/tmp/setupforge-1000/transactions/` through confirmation, exact artifact revalidation, authorization boundary, durable EXECUTING journal publication, and mock runner execution. `REVIEWED_ARTIFACT == EXECUTED_ARTIFACT` verified end-to-end. Zero host packages mutated.
- Binary compilation: `bin/setupforge` recompiled cleanly and verified with `--version`.

## Current State
Phase 2B.4 Controlled Vertical Slice 1 COMPLETE.
Phase 2B.4 Controlled Vertical Slice 2 NOT STARTED.

## Next Action
Advance to Phase 2B.4 Controlled Vertical Slice 2: DNF5 Transaction History Inspection (`/var/lib/dnf/history.sqlite`), Authoritative Outcome Reconciliation (`ProvenSuccess` / `ProvenFailure` / `RECOVERY_REQUIRED`), and Truthful UI Progress Streaming.
