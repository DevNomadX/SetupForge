package update_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/update"
)

// TestRealFedora_DNF5UpdatePlanner executes a safe, read-only transaction planning operation
// on the actual Fedora host using the production DNF5UpdatePlanner.
// ZERO packages are installed, upgraded, or removed.
func TestRealFedora_DNF5UpdatePlanner(t *testing.T) {
	if _, err := exec.LookPath("dnf5"); err != nil {
		t.Skip("skipping real Fedora integration test: dnf5 binary not found")
	}

	planner := update.NewDNF5UpdatePlanner()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	artifact, plan, err := planner.Plan(ctx, "real-fedora-test")
	if err != nil {
		if errors.Is(err, domain.ErrEmptyTransaction) {
			t.Log("Real Fedora host is fully up to date: DNF5 resolved an empty transaction (safe).")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			t.Skip("skipping real Fedora integration test: DNF5 network mirror timed out")
		}
		t.Fatalf("Real Fedora planning failed unexpectedly: %v", err)
	}

	// Always clean up the generated test artifact directory after the test
	defer func() {
		if artifact.Directory != "" {
			_ = os.RemoveAll(artifact.Directory)
		}
	}()

	t.Logf("Real Fedora transaction artifact created successfully at: %s", artifact.Directory)
	t.Logf("Artifact Fingerprint: %s", artifact.Fingerprint)
	t.Logf("Artifact Package Count: %d, Total Bytes: %d", artifact.PackageCount, artifact.TotalBytes)
	t.Logf("Plan Summary: %d upgrades, %d installs, %d removes, %d replaced",
		plan.UpgradeCount, plan.InstallCount, plan.RemoveCount, plan.ReplacedCount)

	// Invariant checks
	if artifact.Directory == "" {
		t.Error("expected non-empty artifact directory")
	}
	if len(artifact.Fingerprint) != 64 {
		t.Errorf("expected 64-char hex fingerprint, got %d chars (%s)", len(artifact.Fingerprint), artifact.Fingerprint)
	}
	if plan.OperationID != "real-fedora-test" {
		t.Errorf("expected OperationID real-fedora-test, got %s", plan.OperationID)
	}
	if len(plan.Packages) == 0 {
		t.Error("expected planned packages when planning succeeded")
	}

	// Phase 2B.3 Invariant assertions: Persistent /var/tmp workspace & sealed state
	if !strings.Contains(artifact.Directory, "/var/tmp/setupforge-") {
		t.Errorf("expected artifact directory in /var/tmp/setupforge-*, got: %s", artifact.Directory)
	}
	if artifact.State != domain.ArtifactStateSealed {
		t.Errorf("expected artifact state SEALED, got: %s", artifact.State)
	}
	expectedDigest := "sha256:" + artifact.Fingerprint
	if artifact.Descriptor.Digest != expectedDigest {
		t.Errorf("expected descriptor digest %s, got: %s", expectedDigest, artifact.Descriptor.Digest)
	}

	// Validate artifact integrity on disk via planner (which delegates to ArtifactStore)
	if err := planner.ValidateArtifact(artifact); err != nil {
		t.Errorf("ValidateArtifact failed on newly generated real artifact: %v", err)
	}

	// Verify that each planned package has a non-empty name and valid action
	for _, pkg := range plan.Packages {
		if pkg.Name == "" {
			t.Errorf("planned package has empty name: %+v", pkg)
		}
		if pkg.Action == "" {
			t.Errorf("planned package has empty action: %+v", pkg)
		}
		if pkg.Action == domain.ActionUpgrade {
			t.Logf("  Planned Upgrade: %s (Old: %s -> New: %s, Repo: %s)", pkg.Name, pkg.OldEVR, pkg.EVR, pkg.RepoID)
		}
	}
}
