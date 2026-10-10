package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrInvalidArtifact is returned when the transaction artifact directory or transaction.json is invalid.
	ErrInvalidArtifact = errors.New("invalid transaction artifact")

	// ErrIncompleteArtifact is returned when required package files are missing from the artifact.
	ErrIncompleteArtifact = errors.New("incomplete transaction artifact")

	// ErrUnreadableArtifact is returned when artifact files cannot be opened or read.
	ErrUnreadableArtifact = errors.New("unreadable transaction artifact")

	// ErrTransactionArtifactCorrupted is returned when an artifact digest does not match its recorded fingerprint.
	ErrTransactionArtifactCorrupted = errors.New("transaction artifact corrupted or modified")

	// ErrCorruptedJournal is returned when the authoritative active transaction journal cannot be parsed or contains invalid data.
	ErrCorruptedJournal = errors.New("active transaction journal is corrupted or invalid")

	// ErrPlanningFailed is returned when the package manager fails to resolve or store an update transaction.
	ErrPlanningFailed = errors.New("system update planning failed")

	// ErrEmptyTransaction is returned when DNF5 resolves an empty transaction with no packages to update.
	ErrEmptyTransaction = errors.New("no packages to update in transaction")

	// ErrUnknownAction is returned when transaction.json contains an unrecognized package action.
	ErrUnknownAction = errors.New("unknown package action in transaction")

	// ErrPlanUnavailable is returned when a transaction plan or its underlying artifact is no longer accessible on disk.
	ErrPlanUnavailable = errors.New("transaction plan is no longer available")

	// ErrStalePlan is returned when a plan's underlying artifact has been modified or corrupted since planning.
	ErrStalePlan = errors.New("transaction plan is stale or invalid")

	// ErrInsufficientStorage is returned when free disk space is insufficient for the transaction artifact.
	ErrInsufficientStorage = errors.New("insufficient storage space for update transaction")

	// ErrEngineLocked is returned when another SetupForge transaction engine holds the advisory workspace lock.
	ErrEngineLocked = errors.New("another setupforge instance is actively running")

	// ErrInvalidArtifactState is returned when an invalid lifecycle state transition is requested on an artifact.
	ErrInvalidArtifactState = errors.New("invalid artifact lifecycle state")

	// ErrInvalidTransactionState is returned when an invalid transaction lifecycle transition is requested.
	ErrInvalidTransactionState = errors.New("invalid transaction lifecycle state")

	// ErrRecoveryRequired is returned when an active transaction is detected in EXECUTING state following an ungraceful exit.
	ErrRecoveryRequired = errors.New("transaction execution interrupted: recovery required")

	// ErrInterruptedTransaction is returned when transaction execution outcome cannot be conclusively established.
	ErrInterruptedTransaction = errors.New("transaction execution interrupted; manual recovery required")

	// ErrAuthorizationFailed is returned when elevated privileges cannot be verified non-interactively.
	ErrAuthorizationFailed = errors.New("authorization failed: elevated privileges not available")

	// ErrArtifactMismatch is returned when an artifact's identity or content does not match the confirmation decision.
	ErrArtifactMismatch = errors.New("artifact identity mismatch against confirmation decision")
)

// PackageAction represents the discrete operational action applied to a package.
type PackageAction string

const (
	ActionUpgrade   PackageAction = "Upgrade"
	ActionInstall   PackageAction = "Install"
	ActionRemove    PackageAction = "Remove"
	ActionReplaced  PackageAction = "Replaced"
	ActionDowngrade PackageAction = "Downgrade"
)

// PlannedPackage represents an individual package operation within an update plan.
type PlannedPackage struct {
	Name        string        // Base package name, e.g. "firefox"
	Action      PackageAction // Upgrade, Install, Remove, Replaced, Downgrade
	NEVRA       string        // Canonical Name-Epoch:Version-Release.Arch
	OldNEVRA    string        // Previous NEVRA if replacing an installed package
	Arch        string        // Target architecture, e.g. "x86_64", "noarch"
	EVR         string        // Target Epoch:Version-Release
	OldEVR      string        // Previous Epoch:Version-Release if replacing
	RepoID      string        // Source repository ID, e.g. "updates"
	Reason      string        // Resolution reason, e.g. "External User", "Dependency"
	PackagePath string        // Relative path to package file in artifact, e.g. "./packages/..."
	SizeBytes   int64         // Exact byte size of the downloaded package file on disk
}

// ArtifactState represents the discrete physical lifecycle state of a transaction bundle on disk.
type ArtifactState string

const (
	// ArtifactStateStaging indicates the artifact directory is actively being written (DNF5 --store).
	// Staging artifacts are mutable, incomplete, and must never be reviewed or executed.
	ArtifactStateStaging ArtifactState = "STAGING"

	// ArtifactStateSealed indicates planning is complete, structural invariants, zero-symlinks,
	// and zero-multi-link regular files are verified, and the deterministic SHA-256 Merkle root digest is locked.
	// SetupForge treats this artifact as logically immutable; physical immutability is guarded by
	// filesystem permissions as defense-in-depth and verified by cryptographic fingerprinting.
	ArtifactStateSealed ArtifactState = "SEALED"

	// ArtifactStateReclaimable indicates the associated transaction reached a terminal state
	// (Completed, Cancelled, Failed) and the bundle is eligible for physical deletion.
	ArtifactStateReclaimable ArtifactState = "RECLAIMABLE"

	// ArtifactStatePurged indicates the artifact directory has been completely deleted from physical media.
	ArtifactStatePurged ArtifactState = "PURGED"
)

// TransactionState represents the logical lifecycle state of an update transaction.
type TransactionState string

const (
	TxStatePlanned          TransactionState = "PLANNED"
	TxStateReviewed         TransactionState = "REVIEWED"
	TxStateConfirmed        TransactionState = "CONFIRMED"
	TxStateAuthorized       TransactionState = "AUTHORIZED"
	TxStateExecuting        TransactionState = "EXECUTING"
	TxStateCompleted        TransactionState = "COMPLETED"
	TxStateFailed           TransactionState = "FAILED"
	TxStateCancelled        TransactionState = "CANCELLED"
	TxStateRecoveryRequired TransactionState = "RECOVERY_REQUIRED"
	TxStateManualRecovery   TransactionState = "MANUAL_RECOVERY"
	TxStateInterrupted      TransactionState = "INTERRUPTED"
)

// IsValid reports whether s is a recognized valid TransactionState.
func (s TransactionState) IsValid() bool {
	switch s {
	case TxStatePlanned, TxStateReviewed, TxStateConfirmed, TxStateAuthorized,
		TxStateExecuting, TxStateCompleted, TxStateFailed, TxStateCancelled,
		TxStateRecoveryRequired, TxStateManualRecovery, TxStateInterrupted:
		return true
	default:
		return false
	}
}

// IsTerminal reports whether s is a terminal state where no further active execution occurs.
func (s TransactionState) IsTerminal() bool {
	switch s {
	case TxStateCompleted, TxStateFailed, TxStateCancelled, TxStateInterrupted, TxStateManualRecovery:
		return true
	default:
		return false
	}
}

// ValidateTransition enforces valid lifecycle transitions between TransactionStates.
// It strictly prevents illegal jumps such as PLANNED -> EXECUTING, REVIEWED -> EXECUTING,
// CONFIRMED -> COMPLETED, FAILED -> EXECUTING, and INTERRUPTED/MANUAL_RECOVERY -> EXECUTING (preventing automatic replay).
func ValidateTransition(from, to TransactionState) error {
	if from == to {
		return nil
	}

	valid := false
	switch from {
	case "":
		valid = (to == TxStatePlanned)
	case TxStatePlanned:
		// PLANNED can transition to REVIEWED, CONFIRMED, CANCELLED, or FAILED.
		// PLANNED -> EXECUTING is strictly forbidden.
		valid = (to == TxStateReviewed || to == TxStateConfirmed || to == TxStateCancelled || to == TxStateFailed)
	case TxStateReviewed:
		// REVIEWED can transition to CONFIRMED, CANCELLED, or FAILED.
		// REVIEWED -> EXECUTING is strictly forbidden.
		valid = (to == TxStateConfirmed || to == TxStateCancelled || to == TxStateFailed)
	case TxStateConfirmed:
		// CONFIRMED can transition to AUTHORIZED, EXECUTING, CANCELLED, or FAILED.
		// CONFIRMED -> COMPLETED is strictly forbidden (cannot bypass execution).
		valid = (to == TxStateAuthorized || to == TxStateExecuting || to == TxStateCancelled || to == TxStateFailed)
	case TxStateAuthorized:
		// AUTHORIZED can transition to EXECUTING, CANCELLED, or FAILED.
		// AUTHORIZED -> COMPLETED is strictly forbidden.
		valid = (to == TxStateExecuting || to == TxStateCancelled || to == TxStateFailed)
	case TxStateExecuting:
		// EXECUTING can transition to COMPLETED, FAILED, or RECOVERY_REQUIRED.
		valid = (to == TxStateCompleted || to == TxStateFailed || to == TxStateRecoveryRequired)
	case TxStateRecoveryRequired:
		// RECOVERY_REQUIRED is an active reconciliation state.
		// It can transition to COMPLETED (proven success), FAILED (proven clean failure),
		// or MANUAL_RECOVERY (inconclusive evidence/timeout).
		// New transitions into legacy INTERRUPTED are strictly rejected.
		// RECOVERY_REQUIRED -> EXECUTING is strictly forbidden (never automatic replay).
		valid = (to == TxStateManualRecovery || to == TxStateCompleted || to == TxStateFailed)
	case TxStateManualRecovery:
		// MANUAL_RECOVERY is terminal for automation; no automatic outgoing transitions are permitted.
		valid = false
	case TxStateInterrupted:
		// INTERRUPTED is a legacy terminal state; automatic replay is strictly forbidden.
		valid = false
	case TxStateCompleted, TxStateFailed, TxStateCancelled:
		// Terminal states cannot transition to active states.
		valid = false
	default:
		valid = false
	}

	if !valid {
		return fmt.Errorf("%w: illegal transition from %q to %q", ErrInvalidTransactionState, from, to)
	}
	return nil
}

const (
	MediaTypeTransactionManifest = "application/vnd.setupforge.transaction.manifest.v1+json"
	MediaTypePackageRPM          = "application/vnd.setupforge.transaction.package.rpm"
	MediaTypeEngineState         = "application/vnd.setupforge.engine.state.v1+json"
)

// Descriptor represents an internal OCI-inspired typed metadata descriptor for artifacts.
type Descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ActiveTransaction represents the authoritative durable state of the currently active transaction,
// persisted to active-transaction.json via crash-consistent atomic publication.
type ActiveTransaction struct {
	OperationID         string           `json:"operationId"`
	State               TransactionState `json:"state"`
	ArtifactState       ArtifactState    `json:"artifactState"`
	ArtifactDirectory   string           `json:"artifactDirectory"`
	ArtifactFingerprint string           `json:"artifactFingerprint"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
	ConfirmedAt         *time.Time       `json:"confirmedAt,omitempty"`
	CompletedAt         *time.Time       `json:"completedAt,omitempty"`
	FailureReason       string           `json:"failureReason,omitempty"`
	Descriptor          Descriptor       `json:"descriptor"`
}

// TransactionArtifact represents an authoritative, verified DNF5-resolved transaction bundle on disk.
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
	// State represents the physical lifecycle state (STAGING, SEALED, RECLAIMABLE, PURGED).
	State ArtifactState
	// Descriptor is the typed metadata descriptor for the artifact bundle.
	Descriptor Descriptor
}

// IsZero reports whether the artifact struct is uninitialized.
func (a TransactionArtifact) IsZero() bool {
	return a.OperationID == "" && a.Directory == "" && a.Fingerprint == ""
}

// Validate checks essential structural invariants of the artifact record.
func (a TransactionArtifact) Validate() error {
	if a.OperationID == "" {
		return errors.New("artifact missing operation ID")
	}
	if a.Directory == "" {
		return errors.New("artifact missing directory")
	}
	if len(a.Fingerprint) != 64 {
		return errors.New("artifact has invalid SHA-256 fingerprint length")
	}
	return nil
}

// UpdatePlan represents the reviewable, user-facing presentation model of an update transaction.
type UpdatePlan struct {
	OperationID        string
	Artifact           TransactionArtifact
	Packages           []PlannedPackage
	UpgradeCount       int
	InstallCount       int
	RemoveCount        int
	ReplacedCount      int
	DowngradeCount     int
	TotalDownloadBytes int64
	HasKernelUpdate    bool
	HasSystemdUpdate   bool
	CreatedAt          time.Time
}

// TotalPackageCount returns the total number of packages scheduled for active changes.
func (p UpdatePlan) TotalPackageCount() int {
	return p.UpgradeCount + p.InstallCount + p.RemoveCount + p.DowngradeCount
}

// DeriveUpdatePlanMetadata derives summary metrics (counts, sizes, kernel/systemd presence) from a slice of packages.
func DeriveUpdatePlanMetadata(packages []PlannedPackage) (upgrades, installs, removes, replaced, downgrades int, totalBytes int64, hasKernel, hasSystemd bool) {
	for _, pkg := range packages {
		switch pkg.Action {
		case ActionUpgrade:
			upgrades++
		case ActionInstall:
			installs++
		case ActionRemove:
			removes++
		case ActionReplaced:
			replaced++
		case ActionDowngrade:
			downgrades++
		}

		if pkg.SizeBytes > 0 {
			totalBytes += pkg.SizeBytes
		}

		name := strings.ToLower(pkg.Name)
		if name == "kernel" || name == "kernel-core" || name == "kernel-modules" || strings.HasPrefix(name, "kernel-") {
			hasKernel = true
		}
		if name == "systemd" || name == "systemd-libs" || name == "systemd-udev" || strings.HasPrefix(name, "systemd-") {
			hasSystemd = true
		}
	}
	return
}

// ConfirmationStatus represents the explicit user confirmation decision.
type ConfirmationStatus string

const (
	ConfirmationStatusConfirmed ConfirmationStatus = "confirmed"
	ConfirmationStatusCancelled ConfirmationStatus = "cancelled"
)

// ConfirmationDecision represents the immutable record of a user's confirmation decision
// bound strictly to the reviewed transaction artifact.
//
// Invariant: Confirmation DOES NOT execute the transaction. It records the user's
// explicit decision regarding the exact reviewed artifact.
type ConfirmationDecision struct {
	Status              ConfirmationStatus
	OperationID         string
	ArtifactDirectory   string
	ArtifactFingerprint string
	DecidedAt           time.Time
}

// IsConfirmed reports whether the user explicitly approved the reviewed transaction.
func (d ConfirmationDecision) IsConfirmed() bool {
	return d.Status == ConfirmationStatusConfirmed
}

// IsCancelled reports whether the user explicitly cancelled or backed out of the review.
func (d ConfirmationDecision) IsCancelled() bool {
	return d.Status == ConfirmationStatusCancelled
}

// UpdatePlanner is the domain capability interface for creating and validating stored transaction artifacts and plans.
type UpdatePlanner interface {
	// Plan creates a stored transaction artifact for available updates and builds a reviewable UpdatePlan.
	Plan(ctx context.Context, opID string) (TransactionArtifact, UpdatePlan, error)
	// ValidateArtifact verifies that the stored artifact still exists on disk, has valid structure,
	// and matches its recorded fingerprint without modification.
	ValidateArtifact(artifact TransactionArtifact) error
}

// ArtifactStore manages the physical storage, integrity, and lifecycle of transaction bundles.
type ArtifactStore interface {
	// CheckCapacity performs preflight admission validation that the destination filesystem has sufficient
	// free space. Note: this is an admission guard, not a guarantee against runtime ENOSPC.
	CheckCapacity(ctx context.Context, estimatedBytes int64) error

	// CreateStaging creates an isolated, unprivileged staging directory for planning.
	CreateStaging(ctx context.Context, opID string) (stagingDir string, err error)

	// Seal validates the artifact structure, asserts zero symlinks/non-regular objects,
	// computes the SHA-256 Merkle digest, and transitions the artifact to SEALED.
	Seal(ctx context.Context, stagingDir string, opID string) (TransactionArtifact, error)

	// Validate verifies that a sealed artifact exists on disk, has intact files without symlinks,
	// and matches its recorded fingerprint.
	Validate(ctx context.Context, artifact TransactionArtifact) error

	// Purge physically removes the artifact directory from disk.
	Purge(ctx context.Context, directory string) error

	// CollectGarbage removes abandoned or unreferenced transaction directories based on
	// ownership and journal state.
	CollectGarbage(ctx context.Context, activeDirs []string) (purgedCount int, err error)

	// FindSealedArtifacts scans the transactions storage and returns paths of all intact sealed artifacts.
	FindSealedArtifacts(ctx context.Context) ([]string, error)
}

// TransactionJournal owns the durable state of the active transaction using crash-consistent atomic publication.
type TransactionJournal interface {
	// ReadActive loads the authoritative active transaction record from disk, or returns nil if none exists.
	ReadActive(ctx context.Context) (*ActiveTransaction, error)

	// WriteActive atomically writes and syncs the active transaction state to disk.
	WriteActive(ctx context.Context, tx ActiveTransaction) error

	// ClearActive safely removes the active transaction record once the transaction is terminal.
	ClearActive(ctx context.Context) error
}

// EngineLock enforces single-active-engine ownership per Linux user session via kernel advisory flock(2).
type EngineLock interface {
	// TryLock attempts to acquire the exclusive advisory engine lock without blocking.
	TryLock() (bool, error)

	// Unlock releases the advisory lock.
	Unlock() error

	// IsHeld reports whether the lock is currently held by this process.
	IsHeld() bool
}

// AuthorizationProvider verifies whether elevated privileges can be acquired
// without blocking on interactive password prompts (e.g. non-interactive sudo preflight).
type AuthorizationProvider interface {
	Authorize(ctx context.Context) error
}

// PrivilegedCommand represents a structured system command requiring elevated execution.
// Discrete argument slices; shell interpreters and arbitrary interpolation are strictly prohibited.
type PrivilegedCommand struct {
	Executable string
	Args       []string
	Env        []string
	Dir        string
}

// ProcessResult captures structured execution evidence from a child process execution.
type ProcessResult struct {
	ExitCode       int           `json:"exitCode"`
	ExitedNormally bool          `json:"exitedNormally"`
	Signaled       bool          `json:"signaled"`
	Signal         string        `json:"signal,omitempty"`
	Cancelled      bool          `json:"cancelled"`
	TimedOut       bool          `json:"timedOut"`
	Stdout         []byte        `json:"stdout,omitempty"`
	Stderr         []byte        `json:"stderr,omitempty"`
	Duration       time.Duration `json:"duration"`
}

// PrivilegedRunner executes structured privileged commands via an operating system authorization provider,
// managing process group containment, signal propagation, and capturing raw process execution evidence.
type PrivilegedRunner interface {
	Run(ctx context.Context, cmd PrivilegedCommand) (ProcessResult, error)
}
