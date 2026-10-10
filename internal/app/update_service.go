package app

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
)

// UpdateServiceOption configures an UpdateService.
type UpdateServiceOption func(*UpdateService)

// WithUpdatePlanner provides an UpdatePlanner to the service.
func WithUpdatePlanner(planner domain.UpdatePlanner) UpdateServiceOption {
	return func(s *UpdateService) {
		s.planner = planner
	}
}

// WithArtifactStore provides an ArtifactStore to the service.
func WithArtifactStore(store domain.ArtifactStore) UpdateServiceOption {
	return func(s *UpdateService) {
		s.store = store
	}
}

// WithTransactionJournal provides a TransactionJournal to the service.
func WithTransactionJournal(journal domain.TransactionJournal) UpdateServiceOption {
	return func(s *UpdateService) {
		s.journal = journal
	}
}

// WithEngineLock provides an EngineLock to the service.
func WithEngineLock(lock domain.EngineLock) UpdateServiceOption {
	return func(s *UpdateService) {
		s.engineLock = lock
	}
}

// WithAuthorizationProvider provides an AuthorizationProvider to the service.
func WithAuthorizationProvider(auth domain.AuthorizationProvider) UpdateServiceOption {
	return func(s *UpdateService) {
		s.authProvider = auth
	}
}

// WithPrivilegedRunner provides a PrivilegedRunner to the service.
func WithPrivilegedRunner(runner domain.PrivilegedRunner) UpdateServiceOption {
	return func(s *UpdateService) {
		s.runner = runner
	}
}

// WithDNF5Path sets an explicit dnf5 binary path for replay.
func WithDNF5Path(path string) UpdateServiceOption {
	return func(s *UpdateService) {
		s.dnf5Path = path
	}
}

// WithReplayCommandBuilder configures a custom DNF5 replay command builder.
func WithReplayCommandBuilder(builder func(dnf5Path, opID, artifactDir string) domain.PrivilegedCommand) UpdateServiceOption {
	return func(s *UpdateService) {
		s.cmdBuilder = builder
	}
}

// WithHistorySource provides a domain.HistorySource to the service.
func WithHistorySource(source domain.HistorySource) UpdateServiceOption {
	return func(s *UpdateService) {
		s.historySource = source
	}
}

// UpdateService orchestrates checking for available system package updates, planning transactions,
// and executing confirmed update plans across the privileged execution boundary.
// It caches the latest status and protects against concurrent duplicate operations.
type UpdateService struct {
	checker       domain.UpdateChecker
	planner       domain.UpdatePlanner
	store         domain.ArtifactStore
	journal       domain.TransactionJournal
	engineLock    domain.EngineLock
	authProvider  domain.AuthorizationProvider
	runner        domain.PrivilegedRunner
	historySource domain.HistorySource
	cmdBuilder    func(dnf5Path, opID, artifactDir string) domain.PrivilegedCommand
	dnf5Path      string

	mu     sync.RWMutex
	cached *domain.UpdateStatus

	checkMu sync.Mutex
	planMu  sync.Mutex
	execMu  sync.Mutex
}

// NewUpdateService constructs an UpdateService with the given UpdateChecker and options.
func NewUpdateService(checker domain.UpdateChecker, opts ...UpdateServiceOption) *UpdateService {
	s := &UpdateService{
		checker: checker,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetUpdateStatus returns the current update status, using cached results if available.
func (s *UpdateService) GetUpdateStatus(ctx context.Context) (domain.UpdateStatus, error) {
	s.mu.RLock()
	if s.cached != nil {
		status := *s.cached
		s.mu.RUnlock()
		return status, nil
	}
	s.mu.RUnlock()

	s.checkMu.Lock()
	defer s.checkMu.Unlock()

	// Double-check under write lock
	s.mu.RLock()
	if s.cached != nil {
		status := *s.cached
		s.mu.RUnlock()
		return status, nil
	}
	s.mu.RUnlock()

	status, err := s.checker.Check(ctx, domain.CheckOptions{RefreshMetadata: false})
	if err != nil {
		return status, fmt.Errorf("checking update status: %w", err)
	}

	s.mu.Lock()
	s.cached = &status
	s.mu.Unlock()

	return status, nil
}

// RefreshUpdateStatus forces a metadata refresh and queries current update availability.
func (s *UpdateService) RefreshUpdateStatus(ctx context.Context) (domain.UpdateStatus, error) {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()

	status, err := s.checker.Check(ctx, domain.CheckOptions{RefreshMetadata: true})
	if err != nil {
		return status, fmt.Errorf("refreshing update status: %w", err)
	}

	s.mu.Lock()
	s.cached = &status
	s.mu.Unlock()

	return status, nil
}

// CachedStatus returns the currently cached UpdateStatus if available without performing any I/O.
func (s *UpdateService) CachedStatus() (domain.UpdateStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached != nil {
		return *s.cached, true
	}
	return domain.UpdateStatus{}, false
}

// PlanUpdate generates a stored transaction artifact and a reviewable UpdatePlan.
// It synchronizes planning calls to prevent concurrent duplicate planner runs.
func (s *UpdateService) PlanUpdate(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error) {
	if s.planner == nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("no update planner configured")
	}

	s.planMu.Lock()
	defer s.planMu.Unlock()

	// 1. Admission capacity check if store is configured
	if s.store != nil {
		if err := s.store.CheckCapacity(ctx, 0); err != nil {
			return domain.TransactionArtifact{}, domain.UpdatePlan{}, err
		}
	}

	// 2. Journal safety check: ensure no active or corrupted transaction exists before planning
	if s.journal != nil {
		active, readErr := s.journal.ReadActive(ctx)
		if readErr != nil {
			return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("%w: cannot plan updates with corrupted/unreadable journal: %v", domain.ErrRecoveryRequired, readErr)
		}
		if active != nil {
			switch active.State {
			case domain.TxStateExecuting, domain.TxStateRecoveryRequired, domain.TxStateManualRecovery, domain.TxStateInterrupted:
				return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("%w: cannot plan updates while existing transaction is in %s state", domain.ErrRecoveryRequired, active.State)
			case domain.TxStateConfirmed, domain.TxStateAuthorized:
				return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("%w: cannot plan updates while existing transaction is confirmed; cancel or clear it first", domain.ErrInvalidTransactionState)
			case domain.TxStatePlanned:
				// Discard previous unconfirmed plan before generating new one
				if s.store != nil && active.ArtifactDirectory != "" {
					_ = s.store.Purge(ctx, active.ArtifactDirectory)
				}
				_ = s.journal.ClearActive(ctx)
			}
		}
	}

	// 3. Resolve update plan and sealed artifact
	artifact, plan, err := s.planner.Plan(ctx, opID)
	if err != nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("generating update plan: %w", err)
	}

	// 4. Persist initial PLANNED state to active-transaction.json
	if s.journal != nil {
		activeTx := domain.ActiveTransaction{
			OperationID:         artifact.OperationID,
			State:               domain.TxStatePlanned,
			ArtifactState:       domain.ArtifactStateSealed,
			ArtifactDirectory:   artifact.Directory,
			ArtifactFingerprint: artifact.Fingerprint,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
			Descriptor:          artifact.Descriptor,
		}
		if writeErr := s.journal.WriteActive(ctx, activeTx); writeErr != nil {
			if s.store != nil {
				_ = s.store.Purge(ctx, artifact.Directory)
			}
			return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("persisting initial transaction state: %w", writeErr)
		}
	}

	return artifact, plan, nil
}

// ConfirmActiveTransaction records the explicit user confirmation decision in the durable journal.
// Invariant: Confirms the exact reviewed artifact. Zero execution, zero sudo, zero package mutation.
func (s *UpdateService) ConfirmActiveTransaction(ctx context.Context, decision domain.ConfirmationDecision) error {
	if !decision.IsConfirmed() {
		return s.CancelActiveTransaction(ctx, decision.OperationID)
	}

	// Invariant: REVIEWED == EXECUTED: assert artifact integrity on disk
	if s.store != nil {
		art := domain.TransactionArtifact{
			OperationID: decision.OperationID,
			Directory:   decision.ArtifactDirectory,
			Fingerprint: decision.ArtifactFingerprint,
		}
		if err := s.store.Validate(ctx, art); err != nil {
			return fmt.Errorf("%w: artifact validation failed before confirmation: %v", domain.ErrTransactionArtifactCorrupted, err)
		}
	}

	if s.journal != nil {
		active, err := s.journal.ReadActive(ctx)
		if err != nil {
			return fmt.Errorf("reading active transaction: %w", err)
		}
		if active == nil {
			return domain.ErrPlanUnavailable
		}
		if active.OperationID != decision.OperationID ||
			active.ArtifactFingerprint != decision.ArtifactFingerprint ||
			active.ArtifactDirectory != decision.ArtifactDirectory {
			return fmt.Errorf("%w: confirmation identity/fingerprint mismatch against journal", domain.ErrTransactionArtifactCorrupted)
		}

		active.State = domain.TxStateConfirmed
		active.ConfirmedAt = &decision.DecidedAt
		active.UpdatedAt = time.Now()
		if err := s.journal.WriteActive(ctx, *active); err != nil {
			return fmt.Errorf("persisting confirmed transaction state: %w", err)
		}
	}

	return nil
}

// ExecuteActiveTransaction executes a confirmed transaction across the privileged execution boundary.
//
// Invariants enforced:
//  1. EngineLock: asserts engine lock is held if configured.
//  2. Decision validity: decision must be explicitly confirmed.
//  3. State precondition: active transaction in journal must be CONFIRMED.
//  4. Exact Artifact Revalidation (Check B): OperationID, directory, fingerprint, structural
//     validity (zero symlinks/Nlink==1), and sealed state must match the confirmation decision byte-for-byte.
//  5. Authorization: verifies authorization before execution. If failed, marks transaction FAILED and halts.
//  6. Durable EXECUTING: writes EXECUTING to the journal BEFORE spawning the privileged process.
//  7. PrivilegedRunner: executes the exact replay command without relaxation flags.
//  8. Capture Evidence: returns the raw ProcessResult. Exit code 0 DOES NOT equal COMPLETED (history reconciliation deferred to Slice 2).
func (s *UpdateService) ExecuteActiveTransaction(ctx context.Context, decision domain.ConfirmationDecision) (domain.ProcessResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ProcessResult{ExitCode: -1, Cancelled: true}, err
	}

	if s.engineLock != nil && !s.engineLock.IsHeld() {
		return domain.ProcessResult{ExitCode: -1}, domain.ErrEngineLocked
	}

	if !decision.IsConfirmed() {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: decision is not confirmed", domain.ErrInvalidTransactionState)
	}

	s.execMu.Lock()
	defer s.execMu.Unlock()

	if s.journal == nil {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("no transaction journal configured")
	}

	// 1. Inspect durable journal state
	active, err := s.journal.ReadActive(ctx)
	if err != nil {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("reading active transaction from journal: %w", err)
	}
	if active == nil {
		return domain.ProcessResult{ExitCode: -1}, domain.ErrPlanUnavailable
	}

	if active.State != domain.TxStateConfirmed {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: active transaction must be CONFIRMED, found %s", domain.ErrInvalidTransactionState, active.State)
	}

	// 2. Exact Artifact Revalidation against Confirmation Decision (Check B)
	if decision.OperationID == "" || active.OperationID != decision.OperationID {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: operation ID mismatch (decision=%q, journal=%q)", domain.ErrArtifactMismatch, decision.OperationID, active.OperationID)
	}

	if decision.ArtifactDirectory == "" || active.ArtifactDirectory != decision.ArtifactDirectory {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: artifact directory mismatch (decision=%q, journal=%q)", domain.ErrArtifactMismatch, decision.ArtifactDirectory, active.ArtifactDirectory)
	}

	if decision.ArtifactFingerprint == "" || active.ArtifactFingerprint != decision.ArtifactFingerprint {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: artifact fingerprint mismatch (decision=%q, journal=%q)", domain.ErrArtifactMismatch, decision.ArtifactFingerprint, active.ArtifactFingerprint)
	}

	if active.ArtifactState != domain.ArtifactStateSealed {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: artifact is not sealed (state=%q)", domain.ErrInvalidArtifactState, active.ArtifactState)
	}

	if s.store != nil {
		art := domain.TransactionArtifact{
			OperationID: decision.OperationID,
			Directory:   decision.ArtifactDirectory,
			Fingerprint: decision.ArtifactFingerprint,
			State:       domain.ArtifactStateSealed,
		}
		if valErr := s.store.Validate(ctx, art); valErr != nil {
			return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: physical artifact revalidation failed: %v", domain.ErrTransactionArtifactCorrupted, valErr)
		}
	}

	// 3. Authorization Boundary Check
	if s.authProvider == nil {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: no authorization provider configured", domain.ErrAuthorizationFailed)
	}

	if authErr := s.authProvider.Authorize(ctx); authErr != nil {
		// Invariant: If authorization fails: CONFIRMED -> FAILED. Do NOT write EXECUTING.
		active.State = domain.TxStateFailed
		active.FailureReason = fmt.Sprintf("authorization failed: %v", authErr)
		active.UpdatedAt = time.Now()
		_ = s.journal.WriteActive(ctx, *active)
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("%w: %v", domain.ErrAuthorizationFailed, authErr)
	}

	// 4. Durable EXECUTING State Transition (must be persisted before process spawn)
	active.State = domain.TxStateExecuting
	active.UpdatedAt = time.Now()
	if writeErr := s.journal.WriteActive(ctx, *active); writeErr != nil {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("publishing durable EXECUTING state: %w", writeErr)
	}

	// 5. Privileged Execution
	if s.runner == nil {
		return domain.ProcessResult{ExitCode: -1}, fmt.Errorf("no privileged runner configured")
	}

	var cmd domain.PrivilegedCommand
	if s.cmdBuilder != nil {
		cmd = s.cmdBuilder(s.dnf5Path, decision.OperationID, decision.ArtifactDirectory)
	} else {
		cmd = defaultReplayCommandBuilder(s.dnf5Path, decision.OperationID, decision.ArtifactDirectory)
	}

	result, runErr := s.runner.Run(ctx, cmd)

	// INVARIANT: Process exit 0 != final transaction success.
	// DNF5 history reconciliation is deferred to Vertical Slice 2.
	// The transaction remains in EXECUTING state in the journal.
	return result, runErr
}

func defaultReplayCommandBuilder(dnf5Path, opID, artifactDir string) domain.PrivilegedCommand {
	if dnf5Path == "" {
		dnf5Path = "/usr/bin/dnf5"
	}
	return domain.PrivilegedCommand{
		Executable: dnf5Path,
		Args: []string{
			"--comment=setupforge:" + opID,
			"-y",
			"replay",
			artifactDir,
		},
		Dir: "/",
		Env: []string{
			"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
			"LC_ALL=C.UTF-8",
			"LANG=C.UTF-8",
		},
	}
}

// CancelActiveTransaction handles user cancellation, updating durable state and purging artifact directories.
func (s *UpdateService) CancelActiveTransaction(ctx context.Context, opID string) error {
	if s.journal != nil {
		active, err := s.journal.ReadActive(ctx)
		if err != nil {
			return err
		}
		if active != nil && (opID == "" || active.OperationID == opID) {
			active.State = domain.TxStateCancelled
			active.ArtifactState = domain.ArtifactStateReclaimable
			active.UpdatedAt = time.Now()
			_ = s.journal.WriteActive(ctx, *active)

			if s.store != nil && active.ArtifactDirectory != "" {
				_ = s.store.Purge(ctx, active.ArtifactDirectory)
			}
			_ = s.journal.ClearActive(ctx)
		}
	}
	return nil
}

// RecoverOrReconcile inspects durable journal state upon startup, applying the conservative
// reconciliation protocol for interrupted transactions and safely reclaiming stale staging bundles.
//
// Invariants:
// 1. Never discards uncertainty: malformed/corrupted journal fails closed (ErrRecoveryRequired).
// 2. Intact sealed artifacts with missing journal fail closed (ErrRecoveryRequired).
// 3. EXECUTING transitions to RECOVERY_REQUIRED -> INTERRUPTED. Automatic replay is strictly forbidden.
// 4. Active transactions (PLANNED, CONFIRMED) are protected against garbage collection.
func (s *UpdateService) RecoverOrReconcile(ctx context.Context) (*domain.ActiveTransaction, error) {
	if s.journal == nil {
		return nil, nil
	}

	active, err := s.journal.ReadActive(ctx)
	if err != nil {
		// Invariant: Do not silently discard uncertainty or clear a corrupted journal
		return nil, fmt.Errorf("%w: active transaction journal is corrupt or invalid: %v", domain.ErrRecoveryRequired, err)
	}

	if active == nil {
		if s.store != nil {
			sealed, findErr := s.store.FindSealedArtifacts(ctx)
			if findErr == nil && len(sealed) > 0 {
				// Invariant: Missing journal when an apparently active sealed artifact exists
				return nil, fmt.Errorf("%w: found intact sealed artifact %s on disk but active journal is missing; manual recovery required",
					domain.ErrRecoveryRequired, sealed[0])
			}
			_, _ = s.store.CollectGarbage(ctx, nil)
		}
		return nil, nil
	}

	// Verify journal/artifact consistency if directory is specified
	if s.store != nil && active.ArtifactDirectory != "" {
		if _, statErr := os.Stat(active.ArtifactDirectory); os.IsNotExist(statErr) {
			switch active.State {
			case domain.TxStateExecuting:
				active.State = domain.TxStateRecoveryRequired
				active.FailureReason = "execution interrupted; artifact directory missing"
				active.UpdatedAt = time.Now()
				if writeErr := s.journal.WriteActive(ctx, *active); writeErr != nil {
					return nil, fmt.Errorf("%w: failed to write RECOVERY_REQUIRED to journal: %v", domain.ErrRecoveryRequired, writeErr)
				}
				return active, domain.ErrRecoveryRequired
			case domain.TxStateRecoveryRequired:
				return active, domain.ErrRecoveryRequired
			case domain.TxStateManualRecovery, domain.TxStateInterrupted:
				return active, domain.ErrInterruptedTransaction
			case domain.TxStateConfirmed:
				return nil, fmt.Errorf("%w: journal references missing confirmed artifact directory: %s", domain.ErrRecoveryRequired, active.ArtifactDirectory)
			case domain.TxStatePlanned:
				return nil, fmt.Errorf("%w: journal references missing planned artifact directory: %s", domain.ErrPlanUnavailable, active.ArtifactDirectory)
			}
		}

		// Directory exists: validate artifact integrity
		art := domain.TransactionArtifact{
			OperationID: active.OperationID,
			Directory:   active.ArtifactDirectory,
			Fingerprint: active.ArtifactFingerprint,
		}
		if valErr := s.store.Validate(ctx, art); valErr != nil {
			switch active.State {
			case domain.TxStateExecuting:
				active.State = domain.TxStateRecoveryRequired
				active.FailureReason = "execution interrupted; artifact validation failed: " + valErr.Error()
				active.UpdatedAt = time.Now()
				if writeErr := s.journal.WriteActive(ctx, *active); writeErr != nil {
					return nil, fmt.Errorf("%w: failed to write RECOVERY_REQUIRED to journal: %v", domain.ErrRecoveryRequired, writeErr)
				}
				if active.ArtifactDirectory != "" {
					_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
				}
				return active, domain.ErrRecoveryRequired
			case domain.TxStateRecoveryRequired:
				if active.ArtifactDirectory != "" {
					_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
				}
				return active, domain.ErrRecoveryRequired
			case domain.TxStateManualRecovery, domain.TxStateInterrupted:
				if active.ArtifactDirectory != "" {
					_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
				}
				return active, domain.ErrInterruptedTransaction
			case domain.TxStateConfirmed:
				return nil, fmt.Errorf("%w: confirmed artifact %s failed integrity validation: %v", domain.ErrRecoveryRequired, active.ArtifactDirectory, valErr)
			case domain.TxStatePlanned:
				return nil, fmt.Errorf("%w: planned artifact %s failed integrity validation: %v", domain.ErrStalePlan, active.ArtifactDirectory, valErr)
			}
		}
	}

	switch active.State {
	case domain.TxStateExecuting:
		// CRITICAL RECOVERY INVARIANT:
		// When an engine starts and finds EXECUTING state:
		// 1. Transition durably to RECOVERY_REQUIRED.
		// 2. Do NOT immediately transition to INTERRUPTED or MANUAL_RECOVERY;
		//    RECOVERY_REQUIRED is an active reconciliation state for evidence evaluation.
		// 3. Preserve original OperationID, ArtifactDirectory, and ArtifactFingerprint.
		// 4. Protect artifact against garbage collection.
		// 5. NEVER automatically replay an uncertain transaction.
		active.State = domain.TxStateRecoveryRequired
		active.UpdatedAt = time.Now()
		if err := s.journal.WriteActive(ctx, *active); err != nil {
			return nil, fmt.Errorf("%w: failed to transition executing transaction to RECOVERY_REQUIRED in journal: %v", domain.ErrRecoveryRequired, err)
		}

		// Preserve artifact for reconciliation; protect against garbage collection
		if s.store != nil && active.ArtifactDirectory != "" {
			_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
		}
		return active, domain.ErrRecoveryRequired

	case domain.TxStateRecoveryRequired:
		// Repeated startup while already in RECOVERY_REQUIRED:
		// Safely recognize the active reconciliation state, protect artifact against GC,
		// and preserve journal evidence for reconciliation without automatic replay.
		if s.store != nil && active.ArtifactDirectory != "" {
			_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
		}
		return active, domain.ErrRecoveryRequired

	case domain.TxStateConfirmed:
		// Valid confirmed artifact survives startup; protect from GC
		if s.store != nil && active.ArtifactDirectory != "" {
			_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
		}
		return active, nil

	case domain.TxStatePlanned:
		// Valid planned artifact survives startup; protect from GC
		if s.store != nil && active.ArtifactDirectory != "" {
			_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
		}
		return active, nil

	case domain.TxStateManualRecovery:
		// Protect manual recovery artifact for operator diagnosis
		if s.store != nil && active.ArtifactDirectory != "" {
			_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
		}
		return active, domain.ErrInterruptedTransaction

	case domain.TxStateInterrupted:
		// Legacy read-compatible state: protect artifact for operator diagnosis
		if s.store != nil && active.ArtifactDirectory != "" {
			_, _ = s.store.CollectGarbage(ctx, []string{active.ArtifactDirectory})
		}
		return active, domain.ErrInterruptedTransaction

	case domain.TxStateCancelled, domain.TxStateFailed:
		if s.store != nil && active.ArtifactDirectory != "" {
			_ = s.store.Purge(ctx, active.ArtifactDirectory)
		}
		_ = s.journal.ClearActive(ctx)
		if s.store != nil {
			_, _ = s.store.CollectGarbage(ctx, nil)
		}
		return nil, nil

	default:
		return active, nil
	}
}

// HasPlanner reports whether the service is equipped with an UpdatePlanner.
func (s *UpdateService) HasPlanner() bool {
	return s.planner != nil
}

// HasJournal reports whether a TransactionJournal is configured.
func (s *UpdateService) HasJournal() bool {
	return s.journal != nil
}

// HasStore reports whether an ArtifactStore is configured.
func (s *UpdateService) HasStore() bool {
	return s.store != nil
}

// HasEngineLock reports whether an EngineLock is configured.
func (s *UpdateService) HasEngineLock() bool {
	return s.engineLock != nil
}

// HasAuthorizationProvider reports whether an AuthorizationProvider is configured.
func (s *UpdateService) HasAuthorizationProvider() bool {
	return s.authProvider != nil
}

// HasPrivilegedRunner reports whether a PrivilegedRunner is configured.
func (s *UpdateService) HasPrivilegedRunner() bool {
	return s.runner != nil
}

// HasHistorySource reports whether a HistorySource is configured.
func (s *UpdateService) HasHistorySource() bool {
	return s.historySource != nil
}

// CollectEvidence queries the configured HistorySource and correlates history records against
// the execution baseline in a strictly read-only manner.
//
// Invariants enforced:
//  1. Strictly read-only: does not modify active-transaction.json, does not purge artifacts,
//     does not trigger replay, and does not execute package commands.
//  2. Returns EvidenceUnavailable if HistorySource is not configured or query fails.
//  3. Correlates via domain.CorrelateHistory adhering to exact OperationID matching and corroborating checks.
func (s *UpdateService) CollectEvidence(ctx context.Context, baseline domain.ExecutionBaseline) (domain.ExecutionEvidence, error) {
	if s.historySource == nil {
		return domain.ExecutionEvidence{
			Classification: domain.EvidenceUnavailable,
			CandidateCount: 0,
			Diagnostic:     "no history source configured; cannot collect evidence",
		}, nil
	}

	records, err := s.historySource.QueryHistory(ctx, domain.HistoryCriteria{})
	if err != nil {
		return domain.ExecutionEvidence{
			Classification: domain.EvidenceUnavailable,
			CandidateCount: 0,
			Diagnostic:     fmt.Sprintf("querying transaction history failed: %v", err),
		}, nil
	}

	evidence := domain.CorrelateHistory(baseline, records)
	return evidence, nil
}

// GetActiveTransaction returns the active transaction from the durable journal if one exists.
func (s *UpdateService) GetActiveTransaction(ctx context.Context) (*domain.ActiveTransaction, error) {
	if s.journal == nil {
		return nil, nil
	}
	return s.journal.ReadActive(ctx)
}

// ValidateArtifact verifies that a transaction artifact exists on disk, has intact files,
// and matches its recorded cryptographic digest.
func (s *UpdateService) ValidateArtifact(ctx context.Context, artifact domain.TransactionArtifact) error {
	if s.planner == nil {
		return fmt.Errorf("no update planner configured")
	}
	return s.planner.ValidateArtifact(artifact)
}
