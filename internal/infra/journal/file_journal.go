package journal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
)

const (
	activeTransactionFilename = "active-transaction.json"
	tempTransactionFilename   = "active-transaction.json.tmp"
)

// FileJournalOption configures FileTransactionJournal.
type FileJournalOption func(*FileTransactionJournal)

// WithJournalDir overrides the directory where the active transaction journal is stored.
func WithJournalDir(dir string) FileJournalOption {
	return func(j *FileTransactionJournal) {
		j.journalDir = dir
	}
}

// FileTransactionJournal manages durable active transaction state using crash-consistent
// atomic publication and durable filesystem synchronization.
//
// Invariant: This provides crash-consistent atomic publication and durable filesystem state,
// subject to the semantics and reliability of the underlying filesystem.
type FileTransactionJournal struct {
	journalDir string
}

// NewFileTransactionJournal creates a TransactionJournal rooted under /var/tmp/setupforge-$UID/journal.
func NewFileTransactionJournal(opts ...FileJournalOption) *FileTransactionJournal {
	j := &FileTransactionJournal{
		journalDir: defaultJournalDir(),
	}

	for _, opt := range opts {
		opt(j)
	}

	return j
}

func defaultJournalDir() string {
	return filepath.Join("/var/tmp", fmt.Sprintf("setupforge-%d", os.Getuid()), "journal")
}

// ReadActive reads the authoritative active transaction state from active-transaction.json.
// Returns nil, nil if no active transaction exists.
// Returns domain.ErrCorruptedJournal (wrapped) if the file exists but cannot be parsed or contains invalid data.
func (j *FileTransactionJournal) ReadActive(ctx context.Context) (*domain.ActiveTransaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	activePath := filepath.Join(j.journalDir, activeTransactionFilename)
	data, err := os.ReadFile(activePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: reading active transaction journal: %v", domain.ErrCorruptedJournal, err)
	}

	var tx domain.ActiveTransaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, fmt.Errorf("%w: parsing active transaction journal: %v", domain.ErrCorruptedJournal, err)
	}

	if tx.OperationID == "" || tx.State == "" {
		return nil, fmt.Errorf("%w: missing operation ID or state", domain.ErrCorruptedJournal)
	}

	if !tx.State.IsValid() {
		return nil, fmt.Errorf("%w: unrecognized transaction state %q", domain.ErrCorruptedJournal, tx.State)
	}

	if tx.ArtifactDirectory == "" || tx.ArtifactFingerprint == "" {
		return nil, fmt.Errorf("%w: missing artifact directory or fingerprint", domain.ErrCorruptedJournal)
	}

	return &tx, nil
}

// WriteActive persists the active transaction state using crash-consistent atomic publication:
// write temporary file -> fsync file -> atomic rename -> fsync parent directory.
//
// Invariants:
// 1. Refuses to overwrite a corrupted or unreadable journal file.
// 2. Validates state transitions via domain.ValidateTransition.
func (j *FileTransactionJournal) WriteActive(ctx context.Context, tx domain.ActiveTransaction) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if tx.OperationID == "" {
		return errors.New("cannot write active transaction without operation ID")
	}
	if !tx.State.IsValid() {
		return fmt.Errorf("%w: cannot write invalid transaction state %q", domain.ErrInvalidTransactionState, tx.State)
	}

	// 1. Check existing journal state and transition validity
	finalPath := filepath.Join(j.journalDir, activeTransactionFilename)
	if _, err := os.Stat(finalPath); err == nil {
		existing, readErr := j.ReadActive(ctx)
		if readErr != nil {
			// Invariant: NEVER overwrite a corrupted journal file; fail closed and preserve evidence.
			return fmt.Errorf("%w: refusing to overwrite corrupted journal; manual recovery required: %v",
				domain.ErrRecoveryRequired, readErr)
		}
		if existing != nil {
			if existing.OperationID != "" && existing.OperationID != tx.OperationID && !existing.State.IsTerminal() {
				return fmt.Errorf("%w: active transaction %q in state %q cannot be replaced by %q",
					domain.ErrInvalidTransactionState, existing.OperationID, existing.State, tx.OperationID)
			}
			if err := domain.ValidateTransition(existing.State, tx.State); err != nil {
				return err
			}
		}
	} else if os.IsNotExist(err) {
		if err := domain.ValidateTransition("", tx.State); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(j.journalDir, 0700); err != nil {
		return fmt.Errorf("creating journal directory: %w", err)
	}

	tx.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling active transaction: %w", err)
	}

	tmpPath := filepath.Join(j.journalDir, tempTransactionFilename)

	// 1. Write temporary journal file with private 0600 permissions
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating temporary journal file: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing temporary journal file: %w", err)
	}

	// 2. Fsync dirty file pages to physical disk
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("fsync temporary journal file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing temporary journal file: %w", err)
	}

	// 3. Atomic rename (overwrites target file on the same filesystem)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic rename journal file: %w", err)
	}

	// 4. Fsync parent directory to flush directory entry metadata to disk journal
	if parentDir, err := os.Open(j.journalDir); err == nil {
		_ = parentDir.Sync()
		_ = parentDir.Close()
	}

	return nil
}

// ClearActive safely removes the active transaction record when a transaction completes, cancels, or fails.
func (j *FileTransactionJournal) ClearActive(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	activePath := filepath.Join(j.journalDir, activeTransactionFilename)
	if err := os.Remove(activePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("removing active transaction journal: %w", err)
	}

	// Flush directory changes
	if parentDir, err := os.Open(j.journalDir); err == nil {
		_ = parentDir.Sync()
		_ = parentDir.Close()
	}

	return nil
}
