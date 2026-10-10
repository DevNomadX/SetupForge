package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/setupforge/setupforge/internal/domain"
)

func TestDNF5UpdatePlanner_UnprivilegedArguments(t *testing.T) {
	baseDir := t.TempDir()

	var recordedPath string
	var recordedArgs []string

	fakeExec := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		recordedPath = name
		recordedArgs = args

		// Simulate DNF5 creating transaction.json in the storeDir
		var storeDir string
		for _, a := range args {
			if strings.HasPrefix(a, "--store=") {
				storeDir = strings.TrimPrefix(a, "--store=")
				break
			}
		}

		if storeDir != "" {
			txJson := `{"version":"1.0","rpms":[{"nevra":"pkg-1.0-1.fc44.x86_64","action":"Install","reason":"User","repo_id":"@stored_transaction(updates)","package_path":"./packages/pkg.rpm"}]}`
			_ = os.WriteFile(filepath.Join(storeDir, "transaction.json"), []byte(txJson), 0600)
			_ = os.MkdirAll(filepath.Join(storeDir, "packages"), 0700)
			_ = os.WriteFile(filepath.Join(storeDir, "packages", "pkg.rpm"), []byte("RPM_PAYLOAD"), 0600)
		}

		return []byte("Repositories loaded."), nil, 0, nil
	}

	planner := NewDNF5UpdatePlanner(
		WithPlannerRawExecutor(fakeExec),
		WithPlannerEuidGetter(func() int { return 1000 }), // Unprivileged
		WithPlannerBaseDir(baseDir),
	)

	ctx := context.Background()
	artifact, plan, err := planner.Plan(ctx, "op-test-1")
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	// Verify command and arguments
	if recordedPath != "/usr/bin/dnf5" {
		t.Errorf("expected executable /usr/bin/dnf5, got %s", recordedPath)
	}

	hasUpgrade := false
	hasStore := false
	hasYes := false
	hasSystemStateDir := false

	for _, a := range recordedArgs {
		if a == "upgrade" {
			hasUpgrade = true
		}
		if strings.HasPrefix(a, "--store=") {
			hasStore = true
		}
		if a == "-y" {
			hasYes = true
		}
		if strings.HasPrefix(a, "--setopt=system_state_dir=") {
			hasSystemStateDir = true
		}
	}

	if !hasUpgrade || !hasStore || !hasYes || !hasSystemStateDir {
		t.Errorf("missing expected unprivileged args in %v", recordedArgs)
	}

	// Verify artifact properties
	if artifact.OperationID != "op-test-1" {
		t.Errorf("expected opID op-test-1, got %s", artifact.OperationID)
	}
	if len(artifact.Fingerprint) != 64 {
		t.Errorf("expected 64-char fingerprint, got %s", artifact.Fingerprint)
	}
	if artifact.PackageCount != 1 {
		t.Errorf("expected 1 package in artifact, got %d", artifact.PackageCount)
	}
	if plan.TotalPackageCount() != 1 {
		t.Errorf("expected 1 package in plan, got %d", plan.TotalPackageCount())
	}
}

func TestDNF5UpdatePlanner_RootArguments(t *testing.T) {
	baseDir := t.TempDir()

	var recordedArgs []string
	fakeExec := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		recordedArgs = args
		for _, a := range args {
			if strings.HasPrefix(a, "--store=") {
				storeDir := strings.TrimPrefix(a, "--store=")
				txJson := `{"version":"1.0","rpms":[{"nevra":"pkg-1.0-1.fc44.x86_64","action":"Install","reason":"User","repo_id":"@stored_transaction(updates)"}]}`
				_ = os.WriteFile(filepath.Join(storeDir, "transaction.json"), []byte(txJson), 0600)
				break
			}
		}
		return []byte("Repositories loaded."), nil, 0, nil
	}

	planner := NewDNF5UpdatePlanner(
		WithPlannerRawExecutor(fakeExec),
		WithPlannerEuidGetter(func() int { return 0 }), // Root
		WithPlannerBaseDir(baseDir),
	)

	_, _, err := planner.Plan(context.Background(), "op-root")
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	for _, a := range recordedArgs {
		if strings.HasPrefix(a, "--setopt=system_state_dir=") {
			t.Errorf("system_state_dir should not be set when running as root, got %s", a)
		}
	}
}

func TestDNF5UpdatePlanner_EmptyTransaction(t *testing.T) {
	baseDir := t.TempDir()

	fakeExec := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		// Output "Nothing to do" with exit code 0 and no files created
		return []byte("Updating and loading repositories:\nRepositories loaded.\nNothing to do.\n"), nil, 0, nil
	}

	planner := NewDNF5UpdatePlanner(
		WithPlannerRawExecutor(fakeExec),
		WithPlannerBaseDir(baseDir),
	)

	_, _, err := planner.Plan(context.Background(), "op-empty")
	if !errors.Is(err, domain.ErrEmptyTransaction) {
		t.Fatalf("expected ErrEmptyTransaction, got: %v", err)
	}
}

func TestDNF5UpdatePlanner_CommandFailureAndCleanup(t *testing.T) {
	baseDir := t.TempDir()

	var createdStoreDir string
	fakeExec := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		for _, a := range args {
			if strings.HasPrefix(a, "--store=") {
				createdStoreDir = strings.TrimPrefix(a, "--store=")
				break
			}
		}
		return nil, []byte("Curl error (6): Couldn't resolve host"), 1, errors.New("exit status 1")
	}

	planner := NewDNF5UpdatePlanner(
		WithPlannerRawExecutor(fakeExec),
		WithPlannerBaseDir(baseDir),
	)

	_, _, err := planner.Plan(context.Background(), "op-fail")
	if !errors.Is(err, domain.ErrPlanningFailed) {
		t.Fatalf("expected ErrPlanningFailed, got: %v", err)
	}

	// Verify temporary directory was cleaned up
	if createdStoreDir != "" {
		if _, statErr := os.Stat(createdStoreDir); !os.IsNotExist(statErr) {
			t.Errorf("expected failed store directory to be removed, but it exists: %s", createdStoreDir)
		}
	}
}

func TestDNF5UpdatePlanner_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel upfront

	planner := NewDNF5UpdatePlanner()
	_, _, err := planner.Plan(ctx, "op-cancel")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestDNF5UpdatePlanner_ValidateArtifact(t *testing.T) {
	planner := NewDNF5UpdatePlanner()

	// 1. Zero artifact
	err := planner.ValidateArtifact(domain.TransactionArtifact{})
	if !errors.Is(err, domain.ErrPlanUnavailable) {
		t.Errorf("expected ErrPlanUnavailable for zero artifact, got: %v", err)
	}

	// 2. Non-existent directory
	missingDir := filepath.Join(t.TempDir(), "non-existent")
	err = planner.ValidateArtifact(domain.TransactionArtifact{
		OperationID: "op-missing",
		Directory:   missingDir,
		Fingerprint: strings.Repeat("a", 64),
	})
	if !errors.Is(err, domain.ErrPlanUnavailable) {
		t.Errorf("expected ErrPlanUnavailable for missing directory, got: %v", err)
	}

	// 3. Valid sample artifact
	sampleDir := createSampleArtifact(t)
	fp, err := ComputeArtifactFingerprint(sampleDir)
	if err != nil {
		t.Fatalf("failed to compute sample artifact fingerprint: %v", err)
	}

	validArtifact := domain.TransactionArtifact{
		OperationID: "op-valid",
		Directory:   sampleDir,
		Fingerprint: fp,
	}
	err = planner.ValidateArtifact(validArtifact)
	if err != nil {
		t.Errorf("expected valid artifact to pass validation, got: %v", err)
	}

	// 4. Corrupted fingerprint (tampered artifact)
	corruptArtifact := validArtifact
	corruptArtifact.Fingerprint = strings.Repeat("f", 64)
	err = planner.ValidateArtifact(corruptArtifact)
	if !errors.Is(err, domain.ErrTransactionArtifactCorrupted) {
		t.Errorf("expected ErrTransactionArtifactCorrupted for mismatched digest, got: %v", err)
	}
}
