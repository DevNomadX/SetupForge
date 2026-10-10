package journal_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/journal"
)

func TestFileTransactionJournal_Lifecycle(t *testing.T) {
	ctx := context.Background()
	journalDir := t.TempDir()

	j := journal.NewFileTransactionJournal(
		journal.WithJournalDir(journalDir),
	)

	// 1. Initial read should return nil when no journal exists
	tx, err := j.ReadActive(ctx)
	if err != nil {
		t.Fatalf("unexpected error reading empty journal: %v", err)
	}
	if tx != nil {
		t.Fatalf("expected nil active transaction, got: %+v", tx)
	}

	// 2. Write active transaction (PLANNED)
	now := time.Now().Truncate(time.Millisecond)
	initialTx := domain.ActiveTransaction{
		OperationID:         "op-journal-test",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   "/var/tmp/setupforge-1000/transactions/tx-1",
		ArtifactFingerprint: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		CreatedAt:           now,
		UpdatedAt:           now,
		Descriptor: domain.Descriptor{
			MediaType: domain.MediaTypeTransactionManifest,
			Digest:    "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			Size:      12345,
		},
	}

	if err := j.WriteActive(ctx, initialTx); err != nil {
		t.Fatalf("WriteActive failed: %v", err)
	}

	// Verify temporary file does not remain
	tmpFile := filepath.Join(journalDir, "active-transaction.json.tmp")
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Errorf("temporary journal file %s was not cleaned up after atomic publication", tmpFile)
	}

	// 3. Read active transaction and verify
	readTx, err := j.ReadActive(ctx)
	if err != nil {
		t.Fatalf("ReadActive failed: %v", err)
	}
	if readTx == nil {
		t.Fatal("expected active transaction, got nil")
	}
	if readTx.OperationID != "op-journal-test" {
		t.Errorf("expected opID op-journal-test, got %s", readTx.OperationID)
	}
	if readTx.State != domain.TxStatePlanned {
		t.Errorf("expected state PLANNED, got %s", readTx.State)
	}
	if readTx.ArtifactState != domain.ArtifactStateSealed {
		t.Errorf("expected artifactState SEALED, got %s", readTx.ArtifactState)
	}
	if readTx.Descriptor.Digest != initialTx.Descriptor.Digest {
		t.Errorf("expected descriptor digest %s, got %s", initialTx.Descriptor.Digest, readTx.Descriptor.Digest)
	}

	// 4. State transition to CONFIRMED
	readTx.State = domain.TxStateConfirmed
	confirmTime := time.Now().Truncate(time.Millisecond)
	readTx.ConfirmedAt = &confirmTime

	if err := j.WriteActive(ctx, *readTx); err != nil {
		t.Fatalf("WriteActive (CONFIRMED) failed: %v", err)
	}

	updatedTx, err := j.ReadActive(ctx)
	if err != nil {
		t.Fatalf("ReadActive failed: %v", err)
	}
	if updatedTx.State != domain.TxStateConfirmed {
		t.Errorf("expected state CONFIRMED, got %s", updatedTx.State)
	}
	if updatedTx.ConfirmedAt == nil {
		t.Errorf("expected confirmedAt to be set")
	}

	// 5. ClearActive removes the journal
	if err := j.ClearActive(ctx); err != nil {
		t.Fatalf("ClearActive failed: %v", err)
	}

	clearedTx, err := j.ReadActive(ctx)
	if err != nil {
		t.Fatalf("ReadActive after clear failed: %v", err)
	}
	if clearedTx != nil {
		t.Errorf("expected nil after ClearActive, got: %+v", clearedTx)
	}
}

func TestFileTransactionJournal_CorruptAndTruncatedData(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		data []byte
	}{
		{"malformed JSON syntax", []byte("{not-valid-json")},
		{"truncated JSON", []byte(`{"operationId":"op-1","state":"PLAN`)},
		{"missing operationId", []byte(`{"state":"PLANNED","artifactDirectory":"/dir","artifactFingerprint":"abc"}`)},
		{"missing state", []byte(`{"operationId":"op-1","artifactDirectory":"/dir","artifactFingerprint":"abc"}`)},
		{"invalid/unrecognized state", []byte(`{"operationId":"op-1","state":"INVALID_STATE","artifactDirectory":"/dir","artifactFingerprint":"abc"}`)},
		{"missing artifact directory", []byte(`{"operationId":"op-1","state":"PLANNED","artifactFingerprint":"abc"}`)},
		{"missing artifact fingerprint", []byte(`{"operationId":"op-1","state":"PLANNED","artifactDirectory":"/dir"}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			journalDir := t.TempDir()
			j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

			activeFile := filepath.Join(journalDir, "active-transaction.json")
			if err := os.WriteFile(activeFile, tc.data, 0600); err != nil {
				t.Fatal(err)
			}

			_, err := j.ReadActive(ctx)
			if err == nil {
				t.Fatalf("expected ErrCorruptedJournal for %s, got nil", tc.name)
			}
			if !errors.Is(err, domain.ErrCorruptedJournal) {
				t.Fatalf("expected error wrapping ErrCorruptedJournal for %s, got: %v", tc.name, err)
			}
		})
	}
}

func TestFileTransactionJournal_RefusesOverwriteOnCorruptedJournal(t *testing.T) {
	ctx := context.Background()
	journalDir := t.TempDir()
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Write corrupted journal
	activeFile := filepath.Join(journalDir, "active-transaction.json")
	corruptContent := []byte("{corrupt-content-that-must-be-preserved")
	if err := os.WriteFile(activeFile, corruptContent, 0600); err != nil {
		t.Fatal(err)
	}

	// Attempt to write new active transaction
	newTx := domain.ActiveTransaction{
		OperationID:         "op-new",
		State:               domain.TxStatePlanned,
		ArtifactDirectory:   "/dir",
		ArtifactFingerprint: "fingerprint",
	}

	err := j.WriteActive(ctx, newTx)
	if err == nil {
		t.Fatal("expected WriteActive to fail and refuse overwriting corrupted journal, got nil")
	}
	if !errors.Is(err, domain.ErrRecoveryRequired) {
		t.Fatalf("expected ErrRecoveryRequired on refusing overwrite, got: %v", err)
	}

	// Verify corrupted file was NOT overwritten and preserves evidence
	data, readErr := os.ReadFile(activeFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != string(corruptContent) {
		t.Errorf("corrupted journal evidence was altered or overwritten!")
	}
}

func TestFileTransactionJournal_RejectsIllegalStateTransitions(t *testing.T) {
	ctx := context.Background()
	journalDir := t.TempDir()
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Initial PLANNED state
	tx := domain.ActiveTransaction{
		OperationID:         "op-trans",
		State:               domain.TxStatePlanned,
		ArtifactDirectory:   "/dir",
		ArtifactFingerprint: "fingerprint",
	}
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatalf("initial WriteActive failed: %v", err)
	}

	// Attempt illegal transition: PLANNED -> EXECUTING
	illegalTx := tx
	illegalTx.State = domain.TxStateExecuting
	err := j.WriteActive(ctx, illegalTx)
	if err == nil {
		t.Fatal("expected WriteActive to reject illegal transition PLANNED -> EXECUTING, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidTransactionState) {
		t.Fatalf("expected ErrInvalidTransactionState, got: %v", err)
	}

	// Attempt to overwrite active transaction with another operation ID
	diffOpTx := domain.ActiveTransaction{
		OperationID:         "op-different",
		State:               domain.TxStatePlanned,
		ArtifactDirectory:   "/dir2",
		ArtifactFingerprint: "fingerprint2",
	}
	err = j.WriteActive(ctx, diffOpTx)
	if err == nil {
		t.Fatal("expected WriteActive to reject overwriting active operation ID, got nil")
	}
	if !errors.Is(err, domain.ErrInvalidTransactionState) {
		t.Fatalf("expected ErrInvalidTransactionState, got: %v", err)
	}
}
