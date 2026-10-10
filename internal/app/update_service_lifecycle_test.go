package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/journal"
	"github.com/setupforge/setupforge/internal/infra/storage"
	"golang.org/x/sys/unix"
)

type fakeUpdatePlannerLifecycle struct {
	artifact domain.TransactionArtifact
	plan     domain.UpdatePlan
	err      error
}

func (f *fakeUpdatePlannerLifecycle) Plan(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error) {
	if f.err != nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, f.err
	}
	art := f.artifact
	art.OperationID = opID
	return art, f.plan, nil
}

func (f *fakeUpdatePlannerLifecycle) ValidateArtifact(artifact domain.TransactionArtifact) error {
	if artifact.Fingerprint == "tampered" {
		return domain.ErrTransactionArtifactCorrupted
	}
	return nil
}

func TestUpdateService_LifecycleAndDurability(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Create a real staged artifact using store
	stagingDir, err := store.CreateStaging(ctx, "op-durability")
	if err != nil {
		t.Fatal(err)
	}

	txJSON := `{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("RPM_PAYLOAD"))

	artifact, err := store.Seal(ctx, stagingDir, "op-durability")
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	planner := &fakeUpdatePlannerLifecycle{
		artifact: artifact,
		plan: domain.UpdatePlan{
			OperationID: "op-durability",
			Artifact:    artifact,
			Packages: []domain.PlannedPackage{
				{Name: "pkg", Action: domain.ActionUpgrade, SizeBytes: 11},
			},
			UpgradeCount: 1,
		},
	}

	svc := app.NewUpdateService(
		nil,
		app.WithUpdatePlanner(planner),
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// 1. PlanUpdate -> persists PLANNED in active-transaction.json
	resArt, resPlan, err := svc.PlanUpdate(ctx, "op-durability")
	if err != nil {
		t.Fatalf("PlanUpdate failed: %v", err)
	}
	if resArt.OperationID != "op-durability" || resPlan.TotalPackageCount() != 1 {
		t.Errorf("unexpected plan result: %+v", resPlan)
	}

	active, err := svc.GetActiveTransaction(ctx)
	if err != nil || active == nil {
		t.Fatalf("expected active transaction in journal, got: %v", err)
	}
	if active.State != domain.TxStatePlanned {
		t.Errorf("expected state PLANNED, got %s", active.State)
	}
	if active.ArtifactFingerprint != artifact.Fingerprint {
		t.Errorf("expected fingerprint %s, got %s", artifact.Fingerprint, active.ArtifactFingerprint)
	}

	// 2. ConfirmActiveTransaction -> transitions to CONFIRMED
	decision := domain.ConfirmationDecision{
		Status:              domain.ConfirmationStatusConfirmed,
		OperationID:         "op-durability",
		ArtifactDirectory:   artifact.Directory,
		ArtifactFingerprint: artifact.Fingerprint,
		DecidedAt:           time.Now(),
	}

	if err := svc.ConfirmActiveTransaction(ctx, decision); err != nil {
		t.Fatalf("ConfirmActiveTransaction failed: %v", err)
	}

	confirmedActive, err := svc.GetActiveTransaction(ctx)
	if err != nil || confirmedActive == nil {
		t.Fatalf("expected active transaction, got: %v", err)
	}
	if confirmedActive.State != domain.TxStateConfirmed {
		t.Errorf("expected state CONFIRMED, got %s", confirmedActive.State)
	}
	if confirmedActive.ConfirmedAt == nil {
		t.Errorf("expected ConfirmedAt timestamp to be set")
	}

	// 3. Confirm with tampered fingerprint -> fails with ErrTransactionArtifactCorrupted
	badDecision := decision
	badDecision.ArtifactFingerprint = "bad-fingerprint"
	err = svc.ConfirmActiveTransaction(ctx, badDecision)
	if !errors.Is(err, domain.ErrTransactionArtifactCorrupted) {
		t.Fatalf("expected ErrTransactionArtifactCorrupted on fingerprint mismatch, got: %v", err)
	}

	// 4. CancelActiveTransaction -> purges directory and clears journal
	if err := svc.CancelActiveTransaction(ctx, "op-durability"); err != nil {
		t.Fatalf("CancelActiveTransaction failed: %v", err)
	}

	clearedActive, err := svc.GetActiveTransaction(ctx)
	if err != nil {
		t.Fatalf("error getting active transaction: %v", err)
	}
	if clearedActive != nil {
		t.Errorf("expected nil active transaction after cancellation, got: %+v", clearedActive)
	}

	// Verify artifact directory was physically purged from disk
	if osExists(artifact.Directory) {
		t.Errorf("expected artifact directory %s to be purged from disk after cancellation", artifact.Directory)
	}
}

func TestUpdateService_ConservativeCrashRecovery(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Create a real staged artifact using store
	stagingDir, err := store.CreateStaging(ctx, "op-crashed")
	if err != nil {
		t.Fatal(err)
	}
	txJSON := `{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("RPM_PAYLOAD"))

	art, err := store.Seal(ctx, stagingDir, "op-crashed")
	if err != nil {
		t.Fatal(err)
	}

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// Simulate system crash during EXECUTING state:
	// Transition through legal states to reach EXECUTING: PLANNED -> CONFIRMED -> AUTHORIZED -> EXECUTING
	tx := domain.ActiveTransaction{
		OperationID:         "op-crashed",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   art.Directory,
		ArtifactFingerprint: art.Fingerprint,
		CreatedAt:           time.Now().Add(-10 * time.Minute),
		UpdatedAt:           time.Now().Add(-10 * time.Minute),
	}
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}

	tx.State = domain.TxStateConfirmed
	confirmTime := time.Now().Add(-8 * time.Minute)
	tx.ConfirmedAt = &confirmTime
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}

	tx.State = domain.TxStateAuthorized
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}

	tx.State = domain.TxStateExecuting
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}

	// Call RecoverOrReconcile
	recoveredTx, recErr := svc.RecoverOrReconcile(ctx)

	// CRITICAL INVARIANTS:
	// 1. Must return ErrRecoveryRequired (indicating reconciliation is required)
	// 2. State must be RECOVERY_REQUIRED (eligible for evidence reconciliation)
	// 3. Must NOT be prematurely transitioned to INTERRUPTED or MANUAL_RECOVERY
	// 4. Original OperationID, ArtifactDirectory, and ArtifactFingerprint must be preserved
	// 5. Artifact must NOT be purged
	// 6. Must NEVER be COMPLETED
	if !errors.Is(recErr, domain.ErrRecoveryRequired) {
		t.Fatalf("expected ErrRecoveryRequired, got: %v", recErr)
	}
	if recoveredTx == nil {
		t.Fatal("expected non-nil recovered transaction")
	}
	if recoveredTx.State != domain.TxStateRecoveryRequired {
		t.Errorf("expected state RECOVERY_REQUIRED, got %s", recoveredTx.State)
	}
	if recoveredTx.State == domain.TxStateInterrupted || recoveredTx.State == domain.TxStateManualRecovery {
		t.Errorf("transaction was prematurely transitioned to terminal state %s", recoveredTx.State)
	}
	if recoveredTx.OperationID != "op-crashed" {
		t.Errorf("expected OperationID op-crashed, got %s", recoveredTx.OperationID)
	}
	if recoveredTx.ArtifactDirectory != art.Directory {
		t.Errorf("expected ArtifactDirectory %s, got %s", art.Directory, recoveredTx.ArtifactDirectory)
	}
	if recoveredTx.ArtifactFingerprint != art.Fingerprint {
		t.Errorf("expected ArtifactFingerprint %s, got %s", art.Fingerprint, recoveredTx.ArtifactFingerprint)
	}

	// Verify artifact directory was PRESERVED on disk for reconciliation
	if !osExists(art.Directory) {
		t.Errorf("artifact directory %s was unexpectedly deleted during crash recovery", art.Directory)
	}
}

func TestUpdateService_JournalCorruption_FailsClosed(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Create valid artifact on disk
	stagingDir, err := store.CreateStaging(ctx, "op-corrupt-test")
	if err != nil {
		t.Fatal(err)
	}
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(`{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("PAYLOAD"))
	art, err := store.Seal(ctx, stagingDir, "op-corrupt-test")
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the journal file
	activeFile := filepath.Join(journalDir, "active-transaction.json")
	_ = osMkdirAll(journalDir)
	_ = osWriteFile(activeFile, []byte("{broken-json-that-cannot-be-parsed"))

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// 1. RecoverOrReconcile must FAIL CLOSED with ErrRecoveryRequired
	_, recErr := svc.RecoverOrReconcile(ctx)
	if !errors.Is(recErr, domain.ErrRecoveryRequired) {
		t.Fatalf("expected ErrRecoveryRequired for corrupt journal, got: %v", recErr)
	}

	// 2. Invariant: Corrupted journal file and artifact evidence MUST NOT be deleted
	if !osExists(activeFile) {
		t.Errorf("corrupted journal file was deleted instead of being preserved")
	}
	if !osExists(art.Directory) {
		t.Errorf("artifact directory was deleted instead of being preserved")
	}

	// 3. PlanUpdate must FAIL CLOSED and refuse to overwrite corrupted state
	planner := &fakeUpdatePlannerLifecycle{artifact: art}
	svcWithPlanner := app.NewUpdateService(
		nil,
		app.WithUpdatePlanner(planner),
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)
	_, _, planErr := svcWithPlanner.PlanUpdate(ctx, "op-new")
	if !errors.Is(planErr, domain.ErrRecoveryRequired) {
		t.Fatalf("expected PlanUpdate to fail closed with ErrRecoveryRequired, got: %v", planErr)
	}
}

func TestUpdateService_MissingJournal_WithActiveSealedArtifact(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Create an intact sealed artifact in store
	stagingDir, err := store.CreateStaging(ctx, "op-orphan-sealed")
	if err != nil {
		t.Fatal(err)
	}
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(`{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("PAYLOAD"))
	art, err := store.Seal(ctx, stagingDir, "op-orphan-sealed")
	if err != nil {
		t.Fatal(err)
	}

	// Ensure NO journal file exists
	_ = os.Remove(filepath.Join(journalDir, "active-transaction.json"))

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// RecoverOrReconcile MUST detect the orphan sealed artifact and fail closed
	_, recErr := svc.RecoverOrReconcile(ctx)
	if !errors.Is(recErr, domain.ErrRecoveryRequired) {
		t.Fatalf("expected ErrRecoveryRequired for missing journal with active sealed artifact, got: %v", recErr)
	}

	// Artifact must NOT be purged
	if !osExists(art.Directory) {
		t.Errorf("active sealed artifact was unexpectedly deleted when journal was missing")
	}
}

func TestUpdateService_JournalArtifactMismatch(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Create real sealed artifact
	stagingDir, err := store.CreateStaging(ctx, "op-mismatch")
	if err != nil {
		t.Fatal(err)
	}
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(`{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("PAYLOAD"))
	art, err := store.Seal(ctx, stagingDir, "op-mismatch")
	if err != nil {
		t.Fatal(err)
	}

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// 1. Journal points to non-existent directory -> fail closed
	txMissingDir := domain.ActiveTransaction{
		OperationID:         "op-mismatch",
		State:               domain.TxStateConfirmed,
		ArtifactDirectory:   filepath.Join(storeBase, "tx-does-not-exist"),
		ArtifactFingerprint: art.Fingerprint,
	}
	// Step through states to reach CONFIRMED
	txMissingDir.State = domain.TxStatePlanned
	_ = j.WriteActive(ctx, txMissingDir)
	txMissingDir.State = domain.TxStateConfirmed
	_ = j.WriteActive(ctx, txMissingDir)

	_, err = svc.RecoverOrReconcile(ctx)
	if !errors.Is(err, domain.ErrRecoveryRequired) {
		t.Fatalf("expected ErrRecoveryRequired for journal pointing to non-existent directory, got: %v", err)
	}

	// 2. Journal fingerprint != artifact fingerprint
	txBadFp := domain.ActiveTransaction{
		OperationID:         "op-mismatch",
		State:               domain.TxStatePlanned,
		ArtifactDirectory:   art.Directory,
		ArtifactFingerprint: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	_ = j.ClearActive(ctx)
	_ = j.WriteActive(ctx, txBadFp)
	txBadFp.State = domain.TxStateConfirmed
	_ = j.WriteActive(ctx, txBadFp)

	_, err = svc.RecoverOrReconcile(ctx)
	if !errors.Is(err, domain.ErrRecoveryRequired) {
		t.Fatalf("expected ErrRecoveryRequired for fingerprint mismatch, got: %v", err)
	}
}

func TestUpdateService_ActiveArtifactsProtectedFromGC(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Create active planned artifact
	staging1, _ := store.CreateStaging(ctx, "op-active-planned")
	_ = osWriteFile(filepath.Join(staging1, "transaction.json"), []byte(`{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`))
	_ = osMkdirAll(filepath.Join(staging1, "packages"))
	_ = osWriteFile(filepath.Join(staging1, "packages", "pkg.rpm"), []byte("PAYLOAD"))
	art1, _ := store.Seal(ctx, staging1, "op-active-planned")

	// Create abandoned unsealed staging debris
	stagingDebris, _ := store.CreateStaging(ctx, "op-debris")
	_ = osWriteFile(filepath.Join(stagingDebris, "partial.tmp"), []byte("debris"))

	// Journal records art1 as PLANNED
	tx := domain.ActiveTransaction{
		OperationID:         "op-active-planned",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   art1.Directory,
		ArtifactFingerprint: art1.Fingerprint,
	}
	_ = j.WriteActive(ctx, tx)

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// RecoverOrReconcile should preserve active planned artifact and purge debris
	active, err := svc.RecoverOrReconcile(ctx)
	if err != nil {
		t.Fatalf("RecoverOrReconcile failed: %v", err)
	}
	if active == nil || active.OperationID != "op-active-planned" {
		t.Fatalf("expected active planned transaction to survive")
	}

	// art1 must survive
	if !osExists(art1.Directory) {
		t.Errorf("active planned artifact was unexpectedly deleted")
	}

	// debris must be purged
	if osExists(stagingDebris) {
		t.Errorf("abandoned staging debris was not purged")
	}
}

func TestUpdateService_CapacityAndWriteFailure(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")

	// Store with capacity mock that simulates ENOSPC
	insufficientStatfs := func(path string, stat *unix.Statfs_t) error {
		stat.Bsize = 4096
		stat.Bavail = 10 // ~40 KB available
		return nil
	}
	store := storage.NewLocalFilesystemStore(
		storage.WithStoreBaseDir(storeBase),
		storage.WithStoreStatfs(insufficientStatfs),
	)

	planner := &fakeUpdatePlannerLifecycle{}
	svc := app.NewUpdateService(
		nil,
		app.WithUpdatePlanner(planner),
		app.WithArtifactStore(store),
	)

	// PlanUpdate must fail admission capacity preflight before executing planner
	_, _, err := svc.PlanUpdate(ctx, "op-nospace")
	if !errors.Is(err, domain.ErrInsufficientStorage) {
		t.Fatalf("expected ErrInsufficientStorage, got: %v", err)
	}
}

func TestUpdateService_RepeatedStartup_PreservesRecoveryRequired(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	stagingDir, err := store.CreateStaging(ctx, "op-repeated-recovery")
	if err != nil {
		t.Fatal(err)
	}
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(`{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("PAYLOAD"))
	art, err := store.Seal(ctx, stagingDir, "op-repeated-recovery")
	if err != nil {
		t.Fatal(err)
	}

	// Step into RECOVERY_REQUIRED
	tx := domain.ActiveTransaction{
		OperationID:         "op-repeated-recovery",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   art.Directory,
		ArtifactFingerprint: art.Fingerprint,
		CreatedAt:           time.Now().Add(-10 * time.Minute),
		UpdatedAt:           time.Now().Add(-10 * time.Minute),
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

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// First startup pass: must recognize RECOVERY_REQUIRED and return ErrRecoveryRequired
	recovered1, err1 := svc.RecoverOrReconcile(ctx)
	if !errors.Is(err1, domain.ErrRecoveryRequired) {
		t.Fatalf("pass 1: expected ErrRecoveryRequired, got: %v", err1)
	}
	if recovered1 == nil || recovered1.State != domain.TxStateRecoveryRequired {
		t.Fatalf("pass 1: expected state RECOVERY_REQUIRED, got: %v", recovered1)
	}
	if recovered1.OperationID != "op-repeated-recovery" || recovered1.ArtifactDirectory != art.Directory || recovered1.ArtifactFingerprint != art.Fingerprint {
		t.Fatalf("pass 1: identity mismatch: %+v", recovered1)
	}

	// Second startup pass: must preserve RECOVERY_REQUIRED without mutating or replaying
	recovered2, err2 := svc.RecoverOrReconcile(ctx)
	if !errors.Is(err2, domain.ErrRecoveryRequired) {
		t.Fatalf("pass 2: expected ErrRecoveryRequired, got: %v", err2)
	}
	if recovered2 == nil || recovered2.State != domain.TxStateRecoveryRequired {
		t.Fatalf("pass 2: expected state RECOVERY_REQUIRED, got: %v", recovered2)
	}
	if recovered2.OperationID != "op-repeated-recovery" {
		t.Fatalf("pass 2: OperationID changed: %s", recovered2.OperationID)
	}

	// Verify artifact directory was retained on disk across repeated startups
	if !osExists(art.Directory) {
		t.Errorf("artifact directory %s was unexpectedly deleted during repeated recovery passes", art.Directory)
	}
}

func TestUpdateService_LegacyInterrupted_PreservedWithoutMutation(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	stagingDir, err := store.CreateStaging(ctx, "op-legacy-interrupted")
	if err != nil {
		t.Fatal(err)
	}
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(`{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("PAYLOAD"))
	art, err := store.Seal(ctx, stagingDir, "op-legacy-interrupted")
	if err != nil {
		t.Fatal(err)
	}

	// Step into legacy INTERRUPTED state
	tx := domain.ActiveTransaction{
		OperationID:         "op-legacy-interrupted",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   art.Directory,
		ArtifactFingerprint: art.Fingerprint,
		CreatedAt:           time.Now().Add(-15 * time.Minute),
		UpdatedAt:           time.Now().Add(-15 * time.Minute),
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

	// 1. Verify that new transitions into legacy INTERRUPTED are strictly rejected by the journal/domain
	tx.State = domain.TxStateInterrupted
	tx.FailureReason = "legacy phase 2B.3 interrupted state"
	if err := j.WriteActive(ctx, tx); err == nil {
		t.Fatal("expected transition RECOVERY_REQUIRED -> INTERRUPTED to be rejected, but it succeeded")
	}

	// 2. Simulate pre-existing Phase 2B.3 disk state by writing the legacy journal JSON directly
	legacyJSON, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := osWriteFile(filepath.Join(journalDir, "active-transaction.json"), legacyJSON); err != nil {
		t.Fatal(err)
	}

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// Startup recovery must preserve legacy INTERRUPTED, protect artifact, and return ErrInterruptedTransaction
	recovered, err := svc.RecoverOrReconcile(ctx)
	if !errors.Is(err, domain.ErrInterruptedTransaction) {
		t.Fatalf("expected ErrInterruptedTransaction, got: %v", err)
	}
	if recovered == nil || recovered.State != domain.TxStateInterrupted {
		t.Fatalf("expected state INTERRUPTED to be preserved, got: %v", recovered)
	}

	// Verify journal on disk was NOT mutated or rewritten to another state
	diskTx, readErr := j.ReadActive(ctx)
	if readErr != nil {
		t.Fatalf("reading active journal: %v", readErr)
	}
	if diskTx.State != domain.TxStateInterrupted {
		t.Fatalf("expected disk journal state to remain INTERRUPTED, got: %s", diskTx.State)
	}

	// Verify artifact directory is intact
	if !osExists(art.Directory) {
		t.Errorf("artifact directory %s was unexpectedly deleted", art.Directory)
	}
}

func TestUpdateService_ManualRecovery_PreservedWithoutMutation(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	stagingDir, err := store.CreateStaging(ctx, "op-manual-recovery")
	if err != nil {
		t.Fatal(err)
	}
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(`{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("PAYLOAD"))
	art, err := store.Seal(ctx, stagingDir, "op-manual-recovery")
	if err != nil {
		t.Fatal(err)
	}

	// Step into MANUAL_RECOVERY state
	tx := domain.ActiveTransaction{
		OperationID:         "op-manual-recovery",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   art.Directory,
		ArtifactFingerprint: art.Fingerprint,
		CreatedAt:           time.Now().Add(-15 * time.Minute),
		UpdatedAt:           time.Now().Add(-15 * time.Minute),
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

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// Startup recovery must preserve MANUAL_RECOVERY, protect artifact, and return ErrInterruptedTransaction
	recovered, err := svc.RecoverOrReconcile(ctx)
	if !errors.Is(err, domain.ErrInterruptedTransaction) {
		t.Fatalf("expected ErrInterruptedTransaction, got: %v", err)
	}
	if recovered == nil || recovered.State != domain.TxStateManualRecovery {
		t.Fatalf("expected state MANUAL_RECOVERY to be preserved, got: %v", recovered)
	}

	// Verify journal on disk remains MANUAL_RECOVERY
	diskTx, readErr := j.ReadActive(ctx)
	if readErr != nil {
		t.Fatalf("reading active journal: %v", readErr)
	}
	if diskTx.State != domain.TxStateManualRecovery {
		t.Fatalf("expected disk journal state to remain MANUAL_RECOVERY, got: %s", diskTx.State)
	}

	if !osExists(art.Directory) {
		t.Errorf("artifact directory %s was unexpectedly deleted", art.Directory)
	}
}

type failingWriteJournalWrapper struct {
	domain.TransactionJournal
	failOnWrite bool
}

func (w *failingWriteJournalWrapper) WriteActive(ctx context.Context, tx domain.ActiveTransaction) error {
	if w.failOnWrite {
		return errors.New("simulated journal write I/O failure")
	}
	return w.TransactionJournal.WriteActive(ctx, tx)
}

func TestUpdateService_JournalPublicationFailure_FailsClosedAndRetainsArtifact(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	rawJournal := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	stagingDir, err := store.CreateStaging(ctx, "op-write-fail")
	if err != nil {
		t.Fatal(err)
	}
	_ = osWriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(`{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`))
	_ = osMkdirAll(filepath.Join(stagingDir, "packages"))
	_ = osWriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("PAYLOAD"))
	art, err := store.Seal(ctx, stagingDir, "op-write-fail")
	if err != nil {
		t.Fatal(err)
	}

	tx := domain.ActiveTransaction{
		OperationID:         "op-write-fail",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   art.Directory,
		ArtifactFingerprint: art.Fingerprint,
	}
	if err := rawJournal.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateConfirmed
	if err := rawJournal.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}
	tx.State = domain.TxStateExecuting
	if err := rawJournal.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}

	// Wrap journal to simulate I/O failure during the EXECUTING -> RECOVERY_REQUIRED write
	failingJournal := &failingWriteJournalWrapper{
		TransactionJournal: rawJournal,
		failOnWrite:        true,
	}

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(failingJournal),
	)

	// RecoverOrReconcile must fail closed with an error wrapping ErrRecoveryRequired
	recovered, err := svc.RecoverOrReconcile(ctx)
	if err == nil {
		t.Fatal("expected failure when journal write fails, got nil")
	}
	if !errors.Is(err, domain.ErrRecoveryRequired) {
		t.Fatalf("expected error wrapping ErrRecoveryRequired, got: %v", err)
	}
	if recovered != nil {
		t.Fatalf("expected nil recovered transaction on journal write failure, got: %v", recovered)
	}

	// Forensic invariant: artifact directory must NOT be purged on journal publication failure
	if !osExists(art.Directory) {
		t.Errorf("artifact directory %s was unexpectedly purged when journal write failed", art.Directory)
	}
}

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}

func osMkdirAll(path string) error {
	return os.MkdirAll(path, 0700)
}

func osExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
