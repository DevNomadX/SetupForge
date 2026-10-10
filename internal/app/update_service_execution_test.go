package app_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/journal"
	"github.com/setupforge/setupforge/internal/infra/storage"
)

type mockAuthProvider struct {
	authorizeErr error
	called       bool
}

func (m *mockAuthProvider) Authorize(ctx context.Context) error {
	m.called = true
	return m.authorizeErr
}

type mockPrivilegedRunner struct {
	called  bool
	command domain.PrivilegedCommand
	result  domain.ProcessResult
	err     error
	runHook func(ctx context.Context, cmd domain.PrivilegedCommand)
}

func (m *mockPrivilegedRunner) Run(ctx context.Context, cmd domain.PrivilegedCommand) (domain.ProcessResult, error) {
	m.called = true
	m.command = cmd
	if m.runHook != nil {
		m.runHook(ctx, cmd)
	}
	return m.result, m.err
}

type mockEngineLock struct {
	held bool
}

func (m *mockEngineLock) TryLock() (bool, error) {
	m.held = true
	return true, nil
}

func (m *mockEngineLock) Unlock() error {
	m.held = false
	return nil
}

func (m *mockEngineLock) IsHeld() bool {
	return m.held
}

type failingJournal struct {
	domain.TransactionJournal
	failOnExecuting bool
}

func (f *failingJournal) WriteActive(ctx context.Context, tx domain.ActiveTransaction) error {
	if f.failOnExecuting && tx.State == domain.TxStateExecuting {
		return errors.New("simulated disk full writing EXECUTING state")
	}
	return f.TransactionJournal.WriteActive(ctx, tx)
}

func setupExecutionEnvironment(t *testing.T, opID string) (*storage.LocalFilesystemStore, *journal.FileTransactionJournal, domain.TransactionArtifact, domain.ConfirmationDecision) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	stagingDir, err := store.CreateStaging(ctx, opID)
	if err != nil {
		t.Fatalf("CreateStaging failed: %v", err)
	}

	txJSON := `{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`
	if err := os.WriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(stagingDir, "packages"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("RPM_PAYLOAD"), 0o600); err != nil {
		t.Fatal(err)
	}

	artifact, err := store.Seal(ctx, stagingDir, opID)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	planner := &fakeUpdatePlannerLifecycle{
		artifact: artifact,
		plan: domain.UpdatePlan{
			OperationID: opID,
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

	// Step 1: Plan
	_, _, err = svc.PlanUpdate(ctx, opID)
	if err != nil {
		t.Fatalf("PlanUpdate failed: %v", err)
	}

	// Step 2: Confirm
	decision := domain.ConfirmationDecision{
		Status:              domain.ConfirmationStatusConfirmed,
		OperationID:         opID,
		ArtifactDirectory:   artifact.Directory,
		ArtifactFingerprint: artifact.Fingerprint,
		DecidedAt:           time.Now(),
	}
	if err := svc.ConfirmActiveTransaction(ctx, decision); err != nil {
		t.Fatalf("ConfirmActiveTransaction failed: %v", err)
	}

	return store, j, artifact, decision
}

func TestUpdateService_ExecuteActiveTransaction_Success(t *testing.T) {
	opID := "op-exec-success"
	store, j, artifact, decision := setupExecutionEnvironment(t, opID)
	ctx := context.Background()

	mockAuth := &mockAuthProvider{}
	mockRunner := &mockPrivilegedRunner{
		result: domain.ProcessResult{
			ExitCode:       0,
			ExitedNormally: true,
			Stdout:         []byte("Replay completed successfully\n"),
			Duration:       50 * time.Millisecond,
		},
	}

	svc := app.NewUpdateService(
		nil,
		app.WithTransactionJournal(j),
		app.WithArtifactStore(store),
		app.WithAuthorizationProvider(mockAuth),
		app.WithPrivilegedRunner(mockRunner),
		app.WithDNF5Path("/usr/bin/dnf5"),
	)

	res, err := svc.ExecuteActiveTransaction(ctx, decision)
	if err != nil {
		t.Fatalf("ExecuteActiveTransaction failed unexpectedly: %v", err)
	}

	if !mockAuth.called {
		t.Error("expected authorization provider to be called")
	}
	if !mockRunner.called {
		t.Error("expected runner to be called")
	}

	// Verify command construction
	if mockRunner.command.Executable != "/usr/bin/dnf5" {
		t.Errorf("expected executable /usr/bin/dnf5, got %s", mockRunner.command.Executable)
	}
	expectedArgs := []string{"--comment=setupforge:" + opID, "-y", "replay", artifact.Directory}
	if len(mockRunner.command.Args) != len(expectedArgs) {
		t.Errorf("expected %d args, got %v", len(expectedArgs), mockRunner.command.Args)
	} else {
		for i, arg := range expectedArgs {
			if mockRunner.command.Args[i] != arg {
				t.Errorf("arg[%d] expected %s, got %s", i, arg, mockRunner.command.Args[i])
			}
		}
	}

	// Verify process result
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}

	// Invariant: Exit code 0 DOES NOT equal COMPLETED.
	// Journal state MUST remain EXECUTING until Slice 2 DNF5 history reconciliation.
	active, err := svc.GetActiveTransaction(ctx)
	if err != nil || active == nil {
		t.Fatalf("failed to read active transaction: %v", err)
	}
	if active.State != domain.TxStateExecuting {
		t.Errorf("expected journal state EXECUTING, got %s", active.State)
	}
}

func TestUpdateService_ExecuteActiveTransaction_EngineLockNotHeld(t *testing.T) {
	opID := "op-exec-lock"
	store, j, _, decision := setupExecutionEnvironment(t, opID)
	ctx := context.Background()

	lock := &mockEngineLock{held: false} // Not held

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
		app.WithEngineLock(lock),
	)

	_, err := svc.ExecuteActiveTransaction(ctx, decision)
	if !errors.Is(err, domain.ErrEngineLocked) {
		t.Errorf("expected ErrEngineLocked, got %v", err)
	}
}

func TestUpdateService_ExecuteActiveTransaction_UnconfirmedDecision(t *testing.T) {
	opID := "op-exec-unconfirmed"
	store, j, artifact, _ := setupExecutionEnvironment(t, opID)
	ctx := context.Background()

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	unconfirmedDecision := domain.ConfirmationDecision{
		Status:              domain.ConfirmationStatusCancelled,
		OperationID:         opID,
		ArtifactDirectory:   artifact.Directory,
		ArtifactFingerprint: artifact.Fingerprint,
		DecidedAt:           time.Now(),
	}

	_, err := svc.ExecuteActiveTransaction(ctx, unconfirmedDecision)
	if !errors.Is(err, domain.ErrInvalidTransactionState) {
		t.Errorf("expected ErrInvalidTransactionState, got %v", err)
	}
}

func TestUpdateService_ExecuteActiveTransaction_InvalidPreconditionState(t *testing.T) {
	ctx := context.Background()
	testDir := t.TempDir()
	storeBase := filepath.Join(testDir, "transactions")
	journalDir := filepath.Join(testDir, "journal")

	store := storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(storeBase))
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// In PLANNED state (not confirmed)
	tx := domain.ActiveTransaction{
		OperationID:         "op-precond",
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   "/some/dir",
		ArtifactFingerprint: "fingerprint123",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatal(err)
	}

	decision := domain.ConfirmationDecision{
		Status:              domain.ConfirmationStatusConfirmed,
		OperationID:         "op-precond",
		ArtifactDirectory:   "/some/dir",
		ArtifactFingerprint: "fingerprint123",
	}

	_, err := svc.ExecuteActiveTransaction(ctx, decision)
	if !errors.Is(err, domain.ErrInvalidTransactionState) {
		t.Errorf("expected ErrInvalidTransactionState when in PLANNED state, got %v", err)
	}
}

func TestUpdateService_ExecuteActiveTransaction_ArtifactMismatch(t *testing.T) {
	opID := "op-exec-mismatch"
	store, j, _, decision := setupExecutionEnvironment(t, opID)
	ctx := context.Background()

	svc := app.NewUpdateService(
		nil,
		app.WithArtifactStore(store),
		app.WithTransactionJournal(j),
	)

	// 1. Mismatched OperationID
	badOpDecision := decision
	badOpDecision.OperationID = "wrong-op"
	_, err := svc.ExecuteActiveTransaction(ctx, badOpDecision)
	if !errors.Is(err, domain.ErrArtifactMismatch) {
		t.Errorf("expected ErrArtifactMismatch on mismatched op ID, got %v", err)
	}

	// 2. Mismatched Directory
	badDirDecision := decision
	badDirDecision.ArtifactDirectory = "/var/tmp/setupforge-fake/transactions/fake"
	_, err = svc.ExecuteActiveTransaction(ctx, badDirDecision)
	if !errors.Is(err, domain.ErrArtifactMismatch) {
		t.Errorf("expected ErrArtifactMismatch on mismatched directory, got %v", err)
	}

	// 3. Mismatched Fingerprint
	badFpDecision := decision
	badFpDecision.ArtifactFingerprint = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = svc.ExecuteActiveTransaction(ctx, badFpDecision)
	if !errors.Is(err, domain.ErrArtifactMismatch) {
		t.Errorf("expected ErrArtifactMismatch on mismatched fingerprint, got %v", err)
	}
}

func TestUpdateService_ExecuteActiveTransaction_ArtifactTamperedOnDisk(t *testing.T) {
	opID := "op-exec-tamper"
	store, j, artifact, decision := setupExecutionEnvironment(t, opID)
	ctx := context.Background()

	// Tamper with the artifact on disk by modifying transaction.json directly
	tamperPath := filepath.Join(artifact.Directory, "transaction.json")
	if err := os.WriteFile(tamperPath, []byte(`{"rpms":[],"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	mockRunner := &mockPrivilegedRunner{}
	svc := app.NewUpdateService(
		nil,
		app.WithTransactionJournal(j),
		app.WithArtifactStore(store),
		app.WithPrivilegedRunner(mockRunner),
	)

	_, err := svc.ExecuteActiveTransaction(ctx, decision)
	if err == nil {
		t.Fatal("expected error on tampered artifact, got nil")
	}
	if !errors.Is(err, domain.ErrTransactionArtifactCorrupted) {
		t.Errorf("expected ErrTransactionArtifactCorrupted, got %v", err)
	}
	if mockRunner.called {
		t.Error("runner must NEVER be called when artifact revalidation fails")
	}
}

func TestUpdateService_ExecuteActiveTransaction_AuthorizationFailure(t *testing.T) {
	opID := "op-exec-auth-fail"
	store, j, _, decision := setupExecutionEnvironment(t, opID)
	ctx := context.Background()

	mockAuth := &mockAuthProvider{
		authorizeErr: domain.ErrAuthorizationFailed,
	}
	mockRunner := &mockPrivilegedRunner{}

	svc := app.NewUpdateService(
		nil,
		app.WithTransactionJournal(j),
		app.WithArtifactStore(store),
		app.WithAuthorizationProvider(mockAuth),
		app.WithPrivilegedRunner(mockRunner),
	)

	_, err := svc.ExecuteActiveTransaction(ctx, decision)
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
	if !errors.Is(err, domain.ErrAuthorizationFailed) {
		t.Errorf("expected ErrAuthorizationFailed, got %v", err)
	}

	// Runner must NOT be invoked
	if mockRunner.called {
		t.Error("runner must NOT be invoked when authorization fails")
	}

	// Journal must transition to FAILED, NEVER EXECUTING
	active, err := svc.GetActiveTransaction(ctx)
	if err != nil || active == nil {
		t.Fatalf("failed to read active transaction: %v", err)
	}
	if active.State != domain.TxStateFailed {
		t.Errorf("expected journal state FAILED on auth failure, got %s", active.State)
	}
}

func TestUpdateService_ExecuteActiveTransaction_JournalWriteExecutingFailure(t *testing.T) {
	opID := "op-exec-journal-fail"
	store, j, _, decision := setupExecutionEnvironment(t, opID)
	ctx := context.Background()

	failJournal := &failingJournal{
		TransactionJournal: j,
		failOnExecuting:    true,
	}

	mockRunner := &mockPrivilegedRunner{}
	mockAuth := &mockAuthProvider{}

	svc := app.NewUpdateService(
		nil,
		app.WithTransactionJournal(failJournal),
		app.WithArtifactStore(store),
		app.WithAuthorizationProvider(mockAuth),
		app.WithPrivilegedRunner(mockRunner),
	)

	_, err := svc.ExecuteActiveTransaction(ctx, decision)
	if err == nil {
		t.Fatal("expected error on journal failure, got nil")
	}

	// If EXECUTING could not be durably published, runner must NEVER be invoked!
	if mockRunner.called {
		t.Error("runner must NOT be invoked if EXECUTING cannot be published to journal")
	}
}

func TestUpdateService_ExecuteActiveTransaction_RunnerFailurePreserved(t *testing.T) {
	opID := "op-exec-runner-fail"
	store, j, _, decision := setupExecutionEnvironment(t, opID)
	ctx := context.Background()

	mockAuth := &mockAuthProvider{}
	mockRunner := &mockPrivilegedRunner{
		result: domain.ProcessResult{
			ExitCode:       1,
			ExitedNormally: true,
			Stderr:         []byte("dnf5: transaction replay failed\n"),
			Duration:       100 * time.Millisecond,
		},
		err: errors.New("exit status 1"),
	}

	svc := app.NewUpdateService(
		nil,
		app.WithTransactionJournal(j),
		app.WithArtifactStore(store),
		app.WithAuthorizationProvider(mockAuth),
		app.WithPrivilegedRunner(mockRunner),
	)

	res, err := svc.ExecuteActiveTransaction(ctx, decision)
	if err == nil {
		t.Fatal("expected runner error, got nil")
	}
	if res.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", res.ExitCode)
	}

	// Journal remains EXECUTING (Slice 1 requirement)
	active, err := svc.GetActiveTransaction(ctx)
	if err != nil || active == nil {
		t.Fatalf("failed to read active transaction: %v", err)
	}
	if active.State != domain.TxStateExecuting {
		t.Errorf("expected state EXECUTING, got %s", active.State)
	}
}

func TestUpdateService_ExecuteActiveTransaction_ContextCancelledBeforeExecution(t *testing.T) {
	opID := "op-exec-cancel-before"
	store, j, _, decision := setupExecutionEnvironment(t, opID)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already cancelled

	mockAuth := &mockAuthProvider{}
	mockRunner := &mockPrivilegedRunner{}

	svc := app.NewUpdateService(
		nil,
		app.WithTransactionJournal(j),
		app.WithArtifactStore(store),
		app.WithAuthorizationProvider(mockAuth),
		app.WithPrivilegedRunner(mockRunner),
	)

	res, err := svc.ExecuteActiveTransaction(ctx, decision)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if !res.Cancelled {
		t.Errorf("expected res.Cancelled to be true")
	}
	if mockAuth.called || mockRunner.called {
		t.Errorf("neither auth nor runner should be called on cancelled context")
	}

	// Active transaction should remain CONFIRMED
	active, err := svc.GetActiveTransaction(context.Background())
	if err != nil || active == nil {
		t.Fatalf("failed reading active transaction: %v", err)
	}
	if active.State != domain.TxStateConfirmed {
		t.Errorf("expected state to remain CONFIRMED, got %s", active.State)
	}
}

func TestUpdateService_ExecuteActiveTransaction_CancellationDuringAuthorization(t *testing.T) {
	opID := "op-exec-cancel-auth"
	store, j, _, decision := setupExecutionEnvironment(t, opID)

	mockAuth := &mockAuthProvider{
		authorizeErr: context.Canceled,
	}
	mockRunner := &mockPrivilegedRunner{}

	svc := app.NewUpdateService(
		nil,
		app.WithTransactionJournal(j),
		app.WithArtifactStore(store),
		app.WithAuthorizationProvider(mockAuth),
		app.WithPrivilegedRunner(mockRunner),
	)

	_, err := svc.ExecuteActiveTransaction(context.Background(), decision)
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
	if !errors.Is(err, domain.ErrAuthorizationFailed) {
		t.Errorf("expected ErrAuthorizationFailed wrapping, got %v", err)
	}
	if mockRunner.called {
		t.Error("runner must NEVER be called when authorization is cancelled")
	}

	active, err := svc.GetActiveTransaction(context.Background())
	if err != nil || active == nil {
		t.Fatalf("failed reading active transaction: %v", err)
	}
	// Invariant: Journal transitions to FAILED, NEVER EXECUTING
	if active.State != domain.TxStateFailed {
		t.Errorf("expected state FAILED, got %s", active.State)
	}
}
