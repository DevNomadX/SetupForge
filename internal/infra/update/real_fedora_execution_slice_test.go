package update_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/journal"
	"github.com/setupforge/setupforge/internal/infra/privilege"
	"github.com/setupforge/setupforge/internal/infra/storage"
)

// TestRealFedora_ControlledExecutionSlice1 tests the end-to-end flow from planning through
// confirmation, exact artifact revalidation, authorization boundary, durable EXECUTING publication,
// and replay command generation on Fedora.
//
// SAFETY INVARIANT: ZERO host packages are modified. The privileged runner records and verifies
// the generated command vector without executing mutating commands against the host.
func TestRealFedora_ControlledExecutionSlice1(t *testing.T) {
	dnf5Path, err := exec.LookPath("dnf5")
	if err != nil {
		t.Skip("skipping real Fedora execution slice test: dnf5 binary not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opID := "real-fedora-slice1-test"
	testDir := t.TempDir()
	journalDir := filepath.Join(testDir, "journal")
	j := journal.NewFileTransactionJournal(journal.WithJournalDir(journalDir))

	// Create real store in /var/tmp per Phase 2B.3 contract
	store := storage.NewLocalFilesystemStore()

	// Step 1: Create an authoritative sealed artifact using production store
	stagingDir, sErr := store.CreateStaging(ctx, opID)
	if sErr != nil {
		t.Fatalf("CreateStaging failed: %v", sErr)
	}
	txJSON := `{"rpms":[{"package_path":"./packages/test-pkg.rpm"}]}`
	if wErr := os.WriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON), 0o600); wErr != nil {
		t.Fatal(wErr)
	}
	if mErr := os.MkdirAll(filepath.Join(stagingDir, "packages"), 0o700); mErr != nil {
		t.Fatal(mErr)
	}
	if wErr := os.WriteFile(filepath.Join(stagingDir, "packages", "test-pkg.rpm"), []byte("RPM_PAYLOAD_SAFE"), 0o600); wErr != nil {
		t.Fatal(wErr)
	}

	artifact, err := store.Seal(ctx, stagingDir, opID)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	plan := domain.UpdatePlan{
		OperationID:  opID,
		Artifact:     artifact,
		Packages:     []domain.PlannedPackage{{Name: "test-pkg", Action: domain.ActionUpgrade, SizeBytes: 16}},
		UpgradeCount: 1,
	}

	// Always clean up generated artifact directory after test
	defer func() {
		if artifact.Directory != "" {
			_ = os.RemoveAll(artifact.Directory)
		}
	}()

	t.Logf("Artifact created at %s (Fingerprint: %s)", artifact.Directory, artifact.Fingerprint)
	t.Logf("Plan has %d packages", plan.TotalPackageCount())

	// Step 2: Validate sealed artifact integrity
	if err := store.Validate(ctx, artifact); err != nil {
		t.Fatalf("Validate failed on newly sealed artifact: %v", err)
	}

	// Step 3: Wire UpdateService with real store, journal, and controlled runner
	var capturedCmd domain.PrivilegedCommand
	mockRunner := &mockSliceRunner{
		runFn: func(ctx context.Context, cmd domain.PrivilegedCommand) (domain.ProcessResult, error) {
			capturedCmd = cmd
			return domain.ProcessResult{
				ExitCode:       0,
				ExitedNormally: true,
				Stdout:         []byte("Mock DNF5 Replay executed safely\n"),
				Duration:       10 * time.Millisecond,
			}, nil
		},
	}

	// Real SudoAuthorizationProvider with root bypass or mock executor to avoid interactive stalls
	authProvider := privilege.NewSudoAuthorizationProvider(
		privilege.WithAuthEuidGetter(func() int { return 0 }), // Root bypass for safe test
	)

	svc := app.NewUpdateService(
		nil,
		app.WithTransactionJournal(j),
		app.WithArtifactStore(store),
		app.WithAuthorizationProvider(authProvider),
		app.WithPrivilegedRunner(mockRunner),
		app.WithDNF5Path(dnf5Path),
	)

	// Step 4: Write PLANNED state to journal
	tx := domain.ActiveTransaction{
		OperationID:         opID,
		State:               domain.TxStatePlanned,
		ArtifactState:       domain.ArtifactStateSealed,
		ArtifactDirectory:   artifact.Directory,
		ArtifactFingerprint: artifact.Fingerprint,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
		Descriptor:          artifact.Descriptor,
	}
	if err := j.WriteActive(ctx, tx); err != nil {
		t.Fatalf("WriteActive failed: %v", err)
	}

	// Step 5: User Confirmation
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

	// Verify journal state is CONFIRMED
	confirmedTx, err := svc.GetActiveTransaction(ctx)
	if err != nil || confirmedTx == nil {
		t.Fatalf("failed reading confirmed transaction: %v", err)
	}
	if confirmedTx.State != domain.TxStateConfirmed {
		t.Fatalf("expected state CONFIRMED, got %s", confirmedTx.State)
	}

	// Step 6: Execute Active Transaction (Slice 1 pipeline)
	res, err := svc.ExecuteActiveTransaction(ctx, decision)
	if err != nil {
		t.Fatalf("ExecuteActiveTransaction failed: %v", err)
	}

	// Step 7: Verify Invariants
	// 7a. Replay command construction
	if capturedCmd.Executable != dnf5Path {
		t.Errorf("expected executable %s, got %s", dnf5Path, capturedCmd.Executable)
	}
	expectedComment := "--comment=setupforge:" + opID
	hasComment := false
	hasReplay := false
	hasArtifactDir := false
	for _, arg := range capturedCmd.Args {
		if arg == expectedComment {
			hasComment = true
		}
		if arg == "replay" {
			hasReplay = true
		}
		if arg == artifact.Directory {
			hasArtifactDir = true
		}
		// Strict rejection of relaxation flags
		if arg == "--ignore-installed" || arg == "--skip-broken" {
			t.Errorf("illegal relaxation flag found in replay command: %s", arg)
		}
	}
	if !hasComment {
		t.Errorf("missing comment flag %s in args: %v", expectedComment, capturedCmd.Args)
	}
	if !hasReplay {
		t.Errorf("missing replay sub-command in args: %v", capturedCmd.Args)
	}
	if !hasArtifactDir {
		t.Errorf("missing exact artifact directory in args: %v", capturedCmd.Args)
	}

	// 7b. REVIEWED_ARTIFACT == EXECUTED_ARTIFACT invariant
	if capturedCmd.Args[len(capturedCmd.Args)-1] != decision.ArtifactDirectory {
		t.Errorf("executed artifact directory does not match decision: got %s, expected %s",
			capturedCmd.Args[len(capturedCmd.Args)-1], decision.ArtifactDirectory)
	}
	if decision.ArtifactFingerprint != artifact.Fingerprint {
		t.Errorf("reviewed fingerprint does not match artifact fingerprint: %s vs %s",
			decision.ArtifactFingerprint, artifact.Fingerprint)
	}

	// 7c. Exit code 0 != COMPLETED invariant: journal remains EXECUTING
	executingTx, err := svc.GetActiveTransaction(ctx)
	if err != nil || executingTx == nil {
		t.Fatalf("failed reading executing transaction: %v", err)
	}
	if executingTx.State != domain.TxStateExecuting {
		t.Errorf("expected journal state EXECUTING, got %s", executingTx.State)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}

	t.Log("Controlled Real Fedora Slice 1 verification PASSED: REVIEWED_ARTIFACT == EXECUTED_ARTIFACT verified.")
}

type mockSliceRunner struct {
	runFn func(ctx context.Context, cmd domain.PrivilegedCommand) (domain.ProcessResult, error)
}

func (m *mockSliceRunner) Run(ctx context.Context, cmd domain.PrivilegedCommand) (domain.ProcessResult, error) {
	if m.runFn != nil {
		return m.runFn(ctx, cmd)
	}
	return domain.ProcessResult{ExitCode: 0, ExitedNormally: true}, nil
}
