package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/setupforge/setupforge/internal/domain"
)

func createSampleArtifact(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	txJson := `{
  "rpms":[
    {
      "nevra":"libmaxminddb-1.14.1-1.fc44.x86_64",
      "action":"Upgrade",
      "reason":"External User",
      "repo_id":"@stored_transaction(updates)",
      "package_path":".\/packages\/libmaxminddb-1.14.1-1.fc44.x86_64.rpm"
    },
    {
      "nevra":"libmaxminddb-1.14.0-1.fc44.x86_64",
      "action":"Replaced",
      "reason":"External User",
      "repo_id":"@System"
    }
  ],
  "version":"1.0"
}`
	if err := os.WriteFile(filepath.Join(dir, "transaction.json"), []byte(txJson), 0600); err != nil {
		t.Fatalf("failed to write transaction.json: %v", err)
	}

	pkgDir := filepath.Join(dir, "packages")
	if err := os.MkdirAll(pkgDir, 0700); err != nil {
		t.Fatalf("failed to create packages dir: %v", err)
	}

	rpmContent := []byte("FAKE_RPM_HEADER_PAYLOAD_BYTES_12345")
	if err := os.WriteFile(filepath.Join(pkgDir, "libmaxminddb-1.14.1-1.fc44.x86_64.rpm"), rpmContent, 0600); err != nil {
		t.Fatalf("failed to write rpm payload: %v", err)
	}

	return dir
}

func TestComputeArtifactFingerprint_IdenticalArtifacts(t *testing.T) {
	dir1 := createSampleArtifact(t)
	dir2 := createSampleArtifact(t)

	fp1, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("ComputeArtifactFingerprint(dir1) error: %v", err)
	}
	fp2, err := ComputeArtifactFingerprint(dir2)
	if err != nil {
		t.Fatalf("ComputeArtifactFingerprint(dir2) error: %v", err)
	}

	if len(fp1) != 64 {
		t.Errorf("expected 64-char hex digest, got %d (%s)", len(fp1), fp1)
	}
	if fp1 != fp2 {
		t.Errorf("expected identical fingerprints for identical artifacts, got %s vs %s", fp1, fp2)
	}
}

func TestComputeArtifactFingerprint_ChangedTransactionJSON(t *testing.T) {
	dir1 := createSampleArtifact(t)
	dir2 := createSampleArtifact(t)

	// Alter transaction.json in dir2
	alteredJson := `{"version":"1.0","rpms":[]}`
	if err := os.WriteFile(filepath.Join(dir2, "transaction.json"), []byte(alteredJson), 0600); err != nil {
		t.Fatalf("failed to alter transaction.json: %v", err)
	}

	fp1, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fp1 error: %v", err)
	}
	fp2, err := ComputeArtifactFingerprint(dir2)
	if err != nil {
		t.Fatalf("fp2 error: %v", err)
	}

	if fp1 == fp2 {
		t.Errorf("fingerprint did not detect changed transaction.json! Both: %s", fp1)
	}
}

func TestComputeArtifactFingerprint_ChangedRPMContent(t *testing.T) {
	dir1 := createSampleArtifact(t)
	dir2 := createSampleArtifact(t)

	// Alter RPM in dir2
	rpmPath := filepath.Join(dir2, "packages", "libmaxminddb-1.14.1-1.fc44.x86_64.rpm")
	if err := os.WriteFile(rpmPath, []byte("MODIFIED_RPM_PAYLOAD_TAMPERED"), 0600); err != nil {
		t.Fatalf("failed to alter rpm: %v", err)
	}

	fp1, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fp1 error: %v", err)
	}
	fp2, err := ComputeArtifactFingerprint(dir2)
	if err != nil {
		t.Fatalf("fp2 error: %v", err)
	}

	if fp1 == fp2 {
		t.Errorf("fingerprint did not detect tampered RPM! Both: %s", fp1)
	}
}

func TestComputeArtifactFingerprint_AddedAndRemovedFile(t *testing.T) {
	dir1 := createSampleArtifact(t)
	dir2 := createSampleArtifact(t)

	fp1, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fp1 error: %v", err)
	}

	// Add an extra replay-relevant package to dir2
	extraRpm := filepath.Join(dir2, "packages", "extra-1.0.0-1.fc44.x86_64.rpm")
	if err := os.WriteFile(extraRpm, []byte("EXTRA_PACKAGE_BYTES"), 0600); err != nil {
		t.Fatalf("failed to write extra rpm: %v", err)
	}

	fp2, err := ComputeArtifactFingerprint(dir2)
	if err != nil {
		t.Fatalf("fp2 error: %v", err)
	}

	if fp1 == fp2 {
		t.Errorf("fingerprint did not detect added package file! Both: %s", fp1)
	}

	// Remove transaction.json and verify error
	_ = os.Remove(filepath.Join(dir2, "transaction.json"))
	_ = os.Remove(extraRpm)
	_ = os.Remove(filepath.Join(dir2, "packages", "libmaxminddb-1.14.1-1.fc44.x86_64.rpm"))

	_, err = ComputeArtifactFingerprint(dir2)
	if err == nil || !errors.Is(err, domain.ErrInvalidArtifact) {
		t.Errorf("expected ErrInvalidArtifact when directory contains no regular files, got: %v", err)
	}
}

func TestComputeArtifactFingerprint_DirectoryTraversalIndependence(t *testing.T) {
	// Create two identical directories with multiple files created in reverse order
	dirA := t.TempDir()
	dirB := t.TempDir()

	_ = os.MkdirAll(filepath.Join(dirA, "packages"), 0700)
	_ = os.MkdirAll(filepath.Join(dirB, "packages"), 0700)

	// DirA: write file1 then file2 then file3
	_ = os.WriteFile(filepath.Join(dirA, "transaction.json"), []byte("TX"), 0600)
	_ = os.WriteFile(filepath.Join(dirA, "packages", "aaa.rpm"), []byte("AAA"), 0600)
	_ = os.WriteFile(filepath.Join(dirA, "packages", "zzz.rpm"), []byte("ZZZ"), 0600)

	// DirB: write file3 then file1 then file2
	_ = os.WriteFile(filepath.Join(dirB, "packages", "zzz.rpm"), []byte("ZZZ"), 0600)
	_ = os.WriteFile(filepath.Join(dirB, "transaction.json"), []byte("TX"), 0600)
	_ = os.WriteFile(filepath.Join(dirB, "packages", "aaa.rpm"), []byte("AAA"), 0600)

	fpA, err := ComputeArtifactFingerprint(dirA)
	if err != nil {
		t.Fatalf("fpA error: %v", err)
	}
	fpB, err := ComputeArtifactFingerprint(dirB)
	if err != nil {
		t.Fatalf("fpB error: %v", err)
	}

	if fpA != fpB {
		t.Errorf("fingerprint is not independent of creation/traversal order: %s != %s", fpA, fpB)
	}
}

func TestComputeArtifactFingerprint_IncludesAllRegularFiles_EvenWithTmpOrLockExtension(t *testing.T) {
	dir1 := createSampleArtifact(t)
	fpBase, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fpBase error: %v", err)
	}

	// 1. Adding a .lock file must change the fingerprint
	lockPath := filepath.Join(dir1, "artifact.lock")
	if err := os.WriteFile(lockPath, []byte("exclusive_lock_marker"), 0600); err != nil {
		t.Fatalf("failed to write lock file: %v", err)
	}

	fpWithLock, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fpWithLock error: %v", err)
	}
	if fpWithLock == fpBase {
		t.Errorf("fingerprint failed to include .lock regular file; digest did not change: %s", fpBase)
	}

	// 2. Modifying the .lock file must change the fingerprint
	if err := os.WriteFile(lockPath, []byte("modified_lock_marker"), 0600); err != nil {
		t.Fatalf("failed to modify lock file: %v", err)
	}
	fpWithModLock, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fpWithModLock error: %v", err)
	}
	if fpWithModLock == fpWithLock {
		t.Errorf("fingerprint failed to detect modified .lock file content: %s", fpWithLock)
	}

	// Remove lock file and verify we return to base fingerprint
	_ = os.Remove(lockPath)
	fpRestored, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fpRestored error: %v", err)
	}
	if fpRestored != fpBase {
		t.Errorf("expected restored fingerprint %s, got %s", fpBase, fpRestored)
	}

	// 3. Adding a .tmp file must change the fingerprint
	tmpPath := filepath.Join(dir1, "packages", "download.rpm.tmp")
	if err := os.WriteFile(tmpPath, []byte("temporary_partial_download"), 0600); err != nil {
		t.Fatalf("failed to write tmp file: %v", err)
	}

	fpWithTmp, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fpWithTmp error: %v", err)
	}
	if fpWithTmp == fpBase {
		t.Errorf("fingerprint failed to include .tmp regular file; digest did not change: %s", fpBase)
	}

	// 4. Modifying the .tmp file must change the fingerprint
	if err := os.WriteFile(tmpPath, []byte("altered_temporary_download"), 0600); err != nil {
		t.Fatalf("failed to modify tmp file: %v", err)
	}
	fpWithModTmp, err := ComputeArtifactFingerprint(dir1)
	if err != nil {
		t.Fatalf("fpWithModTmp error: %v", err)
	}
	if fpWithModTmp == fpWithTmp {
		t.Errorf("fingerprint failed to detect modified .tmp file content: %s", fpWithTmp)
	}
}

func TestComputeArtifactFingerprint_ExcludesNonRegularFiles(t *testing.T) {
	dir := createSampleArtifact(t)
	fpBase, err := ComputeArtifactFingerprint(dir)
	if err != nil {
		t.Fatalf("fpBase error: %v", err)
	}

	// Create a symlink pointing to an existing file
	symlinkPath := filepath.Join(dir, "tx_symlink.json")
	if err := os.Symlink(filepath.Join(dir, "transaction.json"), symlinkPath); err != nil {
		t.Skipf("symlink creation not permitted or supported in test environment: %v", err)
	}
	defer os.Remove(symlinkPath)

	fpWithSymlink, err := ComputeArtifactFingerprint(dir)
	if err != nil {
		t.Fatalf("ComputeArtifactFingerprint with symlink returned error: %v", err)
	}

	// Symlink is a non-regular file, so IsReplayRelevantArtifactFile ignores it;
	// digest must match fpBase.
	if fpWithSymlink != fpBase {
		t.Errorf("expected symlink (non-regular file) to be excluded from fingerprint, got %s vs base %s", fpWithSymlink, fpBase)
	}
}

func TestIsReplayRelevantArtifactFile(t *testing.T) {
	dir := t.TempDir()

	regPath := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(regPath, []byte("regular"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if !IsReplayRelevantArtifactFile(entries[0]) {
		t.Errorf("expected regular file to be replay-relevant")
	}

	// Symlink
	symlinkPath := filepath.Join(dir, "symlink.txt")
	if err := os.Symlink(regPath, symlinkPath); err == nil {
		symEntries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range symEntries {
				if e.Name() == "symlink.txt" {
					if IsReplayRelevantArtifactFile(e) {
						t.Errorf("expected symlink to NOT be replay-relevant")
					}
				}
			}
		}
	}
}

func TestValidateArtifactStructure(t *testing.T) {
	dir := createSampleArtifact(t)

	// Valid artifact
	err := ValidateArtifactStructure(dir, []string{"./packages/libmaxminddb-1.14.1-1.fc44.x86_64.rpm"})
	if err != nil {
		t.Errorf("expected valid artifact structure, got: %v", err)
	}

	// Missing transaction.json
	noTxDir := t.TempDir()
	err = ValidateArtifactStructure(noTxDir, nil)
	if !errors.Is(err, domain.ErrInvalidArtifact) {
		t.Errorf("expected ErrInvalidArtifact for missing transaction.json, got: %v", err)
	}

	// Zero-byte transaction.json
	zeroTxDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(zeroTxDir, "transaction.json"), []byte(""), 0600)
	err = ValidateArtifactStructure(zeroTxDir, nil)
	if !errors.Is(err, domain.ErrInvalidArtifact) {
		t.Errorf("expected ErrInvalidArtifact for zero-byte transaction.json, got: %v", err)
	}

	// Missing required package
	err = ValidateArtifactStructure(dir, []string{"./packages/missing.rpm"})
	if !errors.Is(err, domain.ErrIncompleteArtifact) {
		t.Errorf("expected ErrIncompleteArtifact for missing package payload, got: %v", err)
	}

	// Zero-byte package payload
	zeroRpmPath := filepath.Join(dir, "packages", "zero.rpm")
	_ = os.WriteFile(zeroRpmPath, []byte(""), 0600)
	err = ValidateArtifactStructure(dir, []string{"./packages/zero.rpm"})
	if !errors.Is(err, domain.ErrIncompleteArtifact) {
		t.Errorf("expected ErrIncompleteArtifact for zero-byte package payload, got: %v", err)
	}

	// Path traversal attempt escaping root
	err = ValidateArtifactStructure(dir, []string{"../../etc/passwd"})
	if !errors.Is(err, domain.ErrInvalidArtifact) {
		t.Errorf("expected ErrInvalidArtifact for path traversal escaping artifact, got: %v", err)
	}
}
