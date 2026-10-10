package app_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/journal"
	"github.com/setupforge/setupforge/internal/infra/storage"
)

type mockHistorySource struct {
	records []domain.HistoryRecord
	err     error
	calls   int
}

func (m *mockHistorySource) QueryHistory(ctx context.Context, criteria domain.HistoryCriteria) ([]domain.HistoryRecord, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.records, nil
}

func TestUpdateService_CollectEvidence_NoHistorySource(t *testing.T) {
	service := app.NewUpdateService(nil)
	if service.HasHistorySource() {
		t.Fatal("expected HasHistorySource to be false")
	}

	ev, err := service.CollectEvidence(context.Background(), domain.ExecutionBaseline{OperationID: "op-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Classification != domain.EvidenceUnavailable {
		t.Fatalf("expected EvidenceUnavailable, got %s", ev.Classification)
	}
}

func TestUpdateService_CollectEvidence_HistorySourceError(t *testing.T) {
	mockHist := &mockHistorySource{
		err: errors.New("cannot read sqlite"),
	}
	service := app.NewUpdateService(nil, app.WithHistorySource(mockHist))
	if !service.HasHistorySource() {
		t.Fatal("expected HasHistorySource to be true")
	}

	ev, err := service.CollectEvidence(context.Background(), domain.ExecutionBaseline{OperationID: "op-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Classification != domain.EvidenceUnavailable {
		t.Fatalf("expected EvidenceUnavailable on source error, got %s", ev.Classification)
	}
	if mockHist.calls != 1 {
		t.Fatalf("expected 1 call to history source, got %d", mockHist.calls)
	}
}

func TestUpdateService_CollectEvidence_CorrelatedSuccess(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Second)

	mockHist := &mockHistorySource{
		records: []domain.HistoryRecord{
			{
				ID:                100,
				State:             "Ok",
				StateID:           2,
				StartTime:         now,
				EndTime:           &later,
				RPMDBVersionBegin: "begin-hash",
				RPMDBVersionEnd:   "end-hash",
				Comment:           "setupforge:op-success",
			},
		},
	}

	service := app.NewUpdateService(nil, app.WithHistorySource(mockHist))
	baseline := domain.ExecutionBaseline{
		OperationID:         "op-success",
		BaselineLastTransID: 99,
		BaselineStartTime:   now,
	}

	ev, err := service.CollectEvidence(context.Background(), baseline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Classification != domain.EvidenceProvenSuccess {
		t.Fatalf("expected EvidenceProvenSuccess, got %s (diagnostic: %s)", ev.Classification, ev.Diagnostic)
	}
	if ev.Record == nil || ev.Record.ID != 100 {
		t.Fatalf("expected record ID 100, got %+v", ev.Record)
	}
}

func TestUpdateService_CollectEvidence_ReadOnlyInvariants(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	journalDir := filepath.Join(testDir, "journal")
	storeBase := filepath.Join(testDir, "transactions")

	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))
	st := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))

	// Prepare an active transaction following the legal transition sequence: PLANNED -> CONFIRMED -> EXECUTING
	activeTx := domain.ActiveTransaction{
		OperationID:         "op-executing",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   filepath.Join(storeBase, "op-executing"),
		ArtifactFingerprint: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := j.WriteActive(ctx, activeTx); err != nil {
		t.Fatalf("failed to write PLANNED active transaction: %v", err)
	}

	activeTx.State = domain.TxStateConfirmed
	activeTx.UpdatedAt = time.Now()
	if err := j.WriteActive(ctx, activeTx); err != nil {
		t.Fatalf("failed to write CONFIRMED active transaction: %v", err)
	}

	activeTx.State = domain.TxStateExecuting
	activeTx.UpdatedAt = time.Now()
	if err := j.WriteActive(ctx, activeTx); err != nil {
		t.Fatalf("failed to write EXECUTING active transaction: %v", err)
	}

	mockHist := &mockHistorySource{
		records: []domain.HistoryRecord{
			{
				ID:      10,
				State:   "Started",
				StateID: 1,
				Comment: "setupforge:op-executing",
			},
		},
	}

	service := app.NewUpdateService(nil,
		app.WithTransactionJournal(j),
		app.WithArtifactStore(st),
		app.WithHistorySource(mockHist),
	)

	baseline := domain.ExecutionBaseline{
		OperationID: "op-executing",
	}

	ev, err := service.CollectEvidence(ctx, baseline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Classification != domain.EvidenceInconclusive {
		t.Fatalf("expected EvidenceInconclusive for Started state, got %s", ev.Classification)
	}

	// Assert active journal was NOT modified or cleared
	current, err := j.ReadActive(ctx)
	if err != nil {
		t.Fatalf("unexpected error reading active journal: %v", err)
	}
	if current == nil {
		t.Fatal("expected active journal to still exist, got nil")
	}
	if current.State != domain.TxStateExecuting {
		t.Fatalf("expected journal state to remain EXECUTING, got %s", current.State)
	}
	if current.OperationID != "op-executing" {
		t.Fatalf("expected OperationID op-executing, got %s", current.OperationID)
	}
}
