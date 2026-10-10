package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/journal"
)

type mockUpdatePlanner struct {
	planFunc     func(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error)
	validateFunc func(artifact domain.TransactionArtifact) error
	calls        int32
}

func (m *mockUpdatePlanner) Plan(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error) {
	atomic.AddInt32(&m.calls, 1)
	if m.planFunc != nil {
		return m.planFunc(ctx, opID)
	}
	return domain.TransactionArtifact{OperationID: opID, Directory: "/tmp/mock", Fingerprint: "abc"},
		domain.UpdatePlan{OperationID: opID, UpgradeCount: 2}, nil
}

func (m *mockUpdatePlanner) ValidateArtifact(artifact domain.TransactionArtifact) error {
	if m.validateFunc != nil {
		return m.validateFunc(artifact)
	}
	return nil
}

func TestUpdateService_PlanUpdate_Success(t *testing.T) {
	mockChecker := &mockUpdateChecker{}
	mockPlanner := &mockUpdatePlanner{}

	svc := app.NewUpdateService(mockChecker, app.WithUpdatePlanner(mockPlanner))

	if !svc.HasPlanner() {
		t.Fatal("expected HasPlanner to be true")
	}

	ctx := context.Background()
	artifact, plan, err := svc.PlanUpdate(ctx, "op-1")
	if err != nil {
		t.Fatalf("PlanUpdate failed: %v", err)
	}

	if artifact.OperationID != "op-1" {
		t.Errorf("expected opID op-1, got %s", artifact.OperationID)
	}
	if plan.UpgradeCount != 2 {
		t.Errorf("expected 2 upgrades in plan, got %d", plan.UpgradeCount)
	}
	if atomic.LoadInt32(&mockPlanner.calls) != 1 {
		t.Errorf("expected 1 call to planner, got %d", atomic.LoadInt32(&mockPlanner.calls))
	}
}

func TestUpdateService_PlanUpdate_NoPlanner(t *testing.T) {
	mockChecker := &mockUpdateChecker{}
	svc := app.NewUpdateService(mockChecker)

	if svc.HasPlanner() {
		t.Fatal("expected HasPlanner to be false")
	}

	ctx := context.Background()
	_, _, err := svc.PlanUpdate(ctx, "op-no-planner")
	if err == nil {
		t.Fatal("expected error when no planner is configured, got nil")
	}
}

func TestUpdateService_PlanUpdate_ErrorPropagation(t *testing.T) {
	mockChecker := &mockUpdateChecker{}
	mockPlanner := &mockUpdatePlanner{
		planFunc: func(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error) {
			return domain.TransactionArtifact{}, domain.UpdatePlan{}, domain.ErrPlanningFailed
		},
	}

	svc := app.NewUpdateService(mockChecker, app.WithUpdatePlanner(mockPlanner))

	ctx := context.Background()
	_, _, err := svc.PlanUpdate(ctx, "op-err")
	if !errors.Is(err, domain.ErrPlanningFailed) {
		t.Fatalf("expected ErrPlanningFailed to be wrapped/propagated, got: %v", err)
	}
}

func TestUpdateService_PlanUpdate_ConcurrencySerialization(t *testing.T) {
	mockChecker := &mockUpdateChecker{}

	var activeConcurrent int32
	var maxConcurrent int32

	mockPlanner := &mockUpdatePlanner{
		planFunc: func(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error) {
			curr := atomic.AddInt32(&activeConcurrent, 1)
			for {
				oldMax := atomic.LoadInt32(&maxConcurrent)
				if curr <= oldMax || atomic.CompareAndSwapInt32(&maxConcurrent, oldMax, curr) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&activeConcurrent, -1)
			return domain.TransactionArtifact{OperationID: opID}, domain.UpdatePlan{OperationID: opID}, nil
		},
	}

	svc := app.NewUpdateService(mockChecker, app.WithUpdatePlanner(mockPlanner))

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _, _ = svc.PlanUpdate(context.Background(), "op-concurrent")
		}(i)
	}
	wg.Wait()

	if maxConcurrent != 1 {
		t.Errorf("expected max concurrent planning runs to be 1, got %d", maxConcurrent)
	}
}

func TestUpdateService_ValidateArtifact(t *testing.T) {
	mockChecker := &mockUpdateChecker{}

	// Case 1: No planner configured
	svcNoPlanner := app.NewUpdateService(mockChecker)
	err := svcNoPlanner.ValidateArtifact(context.Background(), domain.TransactionArtifact{})
	if err == nil {
		t.Fatal("expected error when validating without planner")
	}

	// Case 2: Planner returns validation error
	mockPlanner := &mockUpdatePlanner{
		validateFunc: func(artifact domain.TransactionArtifact) error {
			if artifact.OperationID == "corrupt" {
				return domain.ErrTransactionArtifactCorrupted
			}
			return nil
		},
	}
	svc := app.NewUpdateService(mockChecker, app.WithUpdatePlanner(mockPlanner))

	err = svc.ValidateArtifact(context.Background(), domain.TransactionArtifact{OperationID: "corrupt"})
	if !errors.Is(err, domain.ErrTransactionArtifactCorrupted) {
		t.Errorf("expected ErrTransactionArtifactCorrupted, got: %v", err)
	}

	// Case 3: Valid artifact
	err = svc.ValidateArtifact(context.Background(), domain.TransactionArtifact{OperationID: "valid"})
	if err != nil {
		t.Errorf("expected nil error for valid artifact, got: %v", err)
	}
}

func TestUpdateService_PlanUpdate_BlockedOnManualRecovery(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	journalDir := filepath.Join(testDir, "journal")

	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Persist an active transaction in MANUAL_RECOVERY state
	tx := domain.ActiveTransaction{
		OperationID:         "op-unrecovered-123",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   filepath.Join(testDir, "tx-unrecovered"),
		ArtifactFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CreatedAt:           time.Now().Add(-1 * time.Hour),
		UpdatedAt:           time.Now().Add(-1 * time.Hour),
	}
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateConfirmed
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateExecuting
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateRecoveryRequired
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateManualRecovery
	tx.FailureReason = "inconclusive DNF5 execution evidence"
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}

	mockPlanner := &mockUpdatePlanner{}
	svc := app.NewUpdateService(
		nil,
		app.WithUpdatePlanner(mockPlanner),
		app.WithTransactionJournal(j),
	)

	// Attempt to create a new update plan while MANUAL_RECOVERY is unacknowledged/unresolved
	_, _, err := svc.PlanUpdate(ctx, "op-attempt-new-plan")
	if err == nil {
		t.Fatal("expected PlanUpdate to be blocked on MANUAL_RECOVERY, got nil error")
	}
	if !errors.Is(err, domain.ErrRecoveryRequired) {
		t.Fatalf("expected error wrapping ErrRecoveryRequired, got: %v", err)
	}

	// Invariant: The update planner must NOT have been invoked
	if atomic.LoadInt32(&mockPlanner.calls) != 0 {
		t.Errorf("planner was unexpectedly invoked (%d calls) while transaction was in MANUAL_RECOVERY", atomic.LoadInt32(&mockPlanner.calls))
	}

	// Invariant: The active journal must NOT have been overwritten or modified
	active, readErr := j.ReadActive(ctx)
	if readErr != nil {
		t.Fatalf("failed reading active journal: %v", readErr)
	}
	if active == nil || active.State != domain.TxStateManualRecovery || active.OperationID != "op-unrecovered-123" {
		t.Fatalf("active journal was unexpectedly modified: %+v", active)
	}
}

func TestUpdateService_PlanUpdate_BlockedOnLegacyInterrupted(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	journalDir := filepath.Join(testDir, "journal")

	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Step into RECOVERY_REQUIRED then directly write the legacy INTERRUPTED journal to disk
	tx := domain.ActiveTransaction{
		OperationID:         "op-legacy-interrupted-456",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   filepath.Join(testDir, "tx-interrupted"),
		ArtifactFingerprint: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CreatedAt:           time.Now().Add(-2 * time.Hour),
		UpdatedAt:           time.Now().Add(-2 * time.Hour),
	}
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateConfirmed
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateExecuting
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateRecoveryRequired
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}

	// Write legacy INTERRUPTED JSON directly to active-transaction.json
	tx.State = domain.TxStateInterrupted
	tx.FailureReason = "legacy phase 2B.3 interrupted state"
	legacyData, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(journalDir, "active-transaction.json"), legacyData); err != nil {
		t.Fatal(err)
	}

	mockPlanner := &mockUpdatePlanner{}
	svc := app.NewUpdateService(
		nil,
		app.WithUpdatePlanner(mockPlanner),
		app.WithTransactionJournal(j),
	)

	// Attempt to create a new update plan while legacy INTERRUPTED is active
	_, _, err = svc.PlanUpdate(ctx, "op-attempt-new-plan-interrupted")
	if err == nil {
		t.Fatal("expected PlanUpdate to be blocked on INTERRUPTED, got nil error")
	}
	if !errors.Is(err, domain.ErrRecoveryRequired) {
		t.Fatalf("expected error wrapping ErrRecoveryRequired, got: %v", err)
	}

	// Invariant: The update planner must NOT have been invoked
	if atomic.LoadInt32(&mockPlanner.calls) != 0 {
		t.Errorf("planner was unexpectedly invoked (%d calls) while transaction was in INTERRUPTED", atomic.LoadInt32(&mockPlanner.calls))
	}

	// Invariant: The active journal must remain in INTERRUPTED
	active, readErr := j.ReadActive(ctx)
	if readErr != nil {
		t.Fatalf("failed reading active journal: %v", readErr)
	}
	if active == nil || active.State != domain.TxStateInterrupted || active.OperationID != "op-legacy-interrupted-456" {
		t.Fatalf("active journal was unexpectedly modified: %+v", active)
	}
}
