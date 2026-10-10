package storage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/storage"
	"golang.org/x/sys/unix"
)

func TestLocalFilesystemStore_CheckCapacity(t *testing.T) {
	ctx := context.Background()

	t.Run("sufficient storage passes", func(t *testing.T) {
		mockStatfs := func(path string, stat *unix.Statfs_t) error {
			stat.Bsize = 4096
			stat.Bavail = 1000000 // ~4 GB
			return nil
		}

		store := storage.NewLocalFilesystemStore(
			storage.WithStoreBaseDir(t.TempDir()),
			storage.WithStoreStatfs(mockStatfs),
		)

		err := store.CheckCapacity(ctx, 100*1024*1024) // 100 MB
		if err != nil {
			t.Fatalf("expected capacity check to pass, got: %v", err)
		}
	})

	t.Run("insufficient storage fails with ErrInsufficientStorage", func(t *testing.T) {
		mockStatfs := func(path string, stat *unix.Statfs_t) error {
			stat.Bsize = 4096
			stat.Bavail = 1000 // ~4 MB
			return nil
		}

		store := storage.NewLocalFilesystemStore(
			storage.WithStoreBaseDir(t.TempDir()),
			storage.WithStoreStatfs(mockStatfs),
		)

		err := store.CheckCapacity(ctx, 50*1024*1024) // requires 50*2 + 100MB = 200MB
		if !errors.Is(err, domain.ErrInsufficientStorage) {
			t.Fatalf("expected ErrInsufficientStorage, got: %v", err)
		}
	})

	t.Run("statfs error propagates", func(t *testing.T) {
		mockStatfs := func(path string, stat *unix.Statfs_t) error {
			return unix.EIO
		}

		store := storage.NewLocalFilesystemStore(
			storage.WithStoreBaseDir(t.TempDir()),
			storage.WithStoreStatfs(mockStatfs),
		)

		err := store.CheckCapacity(ctx, 100)
		if err == nil || !errors.Is(err, unix.EIO) {
			t.Fatalf("expected EIO error, got: %v", err)
		}
	})
}

func TestLocalFilesystemStore_StagingAndSeal(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()

	store := storage.NewLocalFilesystemStore(
		storage.WithStoreBaseDir(baseDir),
	)

	stagingDir, err := store.CreateStaging(ctx, "op-test-seal")
	if err != nil {
		t.Fatalf("CreateStaging failed: %v", err)
	}

	info, err := os.Stat(stagingDir)
	if err != nil {
		t.Fatalf("staging dir not accessible: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Errorf("expected 0700 permissions, got %o", info.Mode().Perm())
	}

	// Create valid transaction files
	txJSON := `{"rpms":[{"package_path":"./packages/pkg1.rpm"},{"package_path":"./packages/pkg2.rpm"}]}`
	if err := os.WriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON), 0600); err != nil {
		t.Fatal(err)
	}

	pkgsDir := filepath.Join(stagingDir, "packages")
	if err := os.MkdirAll(pkgsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgsDir, "pkg1.rpm"), []byte("RPM_PAYLOAD_1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgsDir, "pkg2.rpm"), []byte("RPM_PAYLOAD_2"), 0600); err != nil {
		t.Fatal(err)
	}

	artifact, err := store.Seal(ctx, stagingDir, "op-test-seal")
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	if artifact.OperationID != "op-test-seal" {
		t.Errorf("expected opID op-test-seal, got %s", artifact.OperationID)
	}
	if artifact.State != domain.ArtifactStateSealed {
		t.Errorf("expected state SEALED, got %s", artifact.State)
	}
	if artifact.PackageCount != 2 {
		t.Errorf("expected 2 packages, got %d", artifact.PackageCount)
	}
	if len(artifact.Fingerprint) != 64 {
		t.Errorf("expected 64-char fingerprint, got %s", artifact.Fingerprint)
	}
	if artifact.Descriptor.MediaType != domain.MediaTypeTransactionManifest {
		t.Errorf("expected descriptor mediaType %s, got %s", domain.MediaTypeTransactionManifest, artifact.Descriptor.MediaType)
	}

	// Validate artifact passes
	if err := store.Validate(ctx, artifact); err != nil {
		t.Fatalf("Validate failed on valid sealed artifact: %v", err)
	}

	// Alter file -> Validate must fail with ErrTransactionArtifactCorrupted
	if err := os.WriteFile(filepath.Join(pkgsDir, "pkg1.rpm"), []byte("TAMPERED"), 0600); err != nil {
		t.Fatal(err)
	}

	err = store.Validate(ctx, artifact)
	if !errors.Is(err, domain.ErrTransactionArtifactCorrupted) {
		t.Fatalf("expected ErrTransactionArtifactCorrupted for tampered file, got: %v", err)
	}
}

func TestLocalFilesystemStore_SymlinkRejection(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()

	store := storage.NewLocalFilesystemStore(
		storage.WithStoreBaseDir(baseDir),
	)

	stagingDir, err := store.CreateStaging(ctx, "op-symlink-test")
	if err != nil {
		t.Fatal(err)
	}

	txJSON := `{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`
	_ = os.WriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON), 0600)
	_ = os.MkdirAll(filepath.Join(stagingDir, "packages"), 0700)
	_ = os.WriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("REAL_RPM"), 0600)

	// Add an attack symlink pointing to /etc/passwd
	symlinkPath := filepath.Join(stagingDir, "packages", "evil-link.rpm")
	if err := os.Symlink("/etc/passwd", symlinkPath); err != nil {
		t.Skip("symlinks not supported on this filesystem")
	}

	// Seal MUST detect and reject the symlink
	_, err = store.Seal(ctx, stagingDir, "op-symlink-test")
	if !errors.Is(err, domain.ErrTransactionArtifactCorrupted) {
		t.Fatalf("expected ErrTransactionArtifactCorrupted when symlink is present, got: %v", err)
	}
}

func TestLocalFilesystemStore_MissingOrEmptyPayload(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()

	store := storage.NewLocalFilesystemStore(
		storage.WithStoreBaseDir(baseDir),
	)

	t.Run("missing transaction.json", func(t *testing.T) {
		stagingDir, err := store.CreateStaging(ctx, "op-no-json")
		if err != nil {
			t.Fatal(err)
		}
		_, err = store.Seal(ctx, stagingDir, "op-no-json")
		if !errors.Is(err, domain.ErrInvalidArtifact) {
			t.Fatalf("expected ErrInvalidArtifact, got: %v", err)
		}
	})

	t.Run("missing package file", func(t *testing.T) {
		stagingDir, err := store.CreateStaging(ctx, "op-missing-pkg")
		if err != nil {
			t.Fatal(err)
		}
		txJSON := `{"rpms":[{"package_path":"./packages/nonexistent.rpm"}]}`
		_ = os.WriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON), 0600)

		_, err = store.Seal(ctx, stagingDir, "op-missing-pkg")
		if !errors.Is(err, domain.ErrIncompleteArtifact) {
			t.Fatalf("expected ErrIncompleteArtifact, got: %v", err)
		}
	})

	t.Run("zero-byte package file", func(t *testing.T) {
		stagingDir, err := store.CreateStaging(ctx, "op-empty-pkg")
		if err != nil {
			t.Fatal(err)
		}
		txJSON := `{"rpms":[{"package_path":"./packages/empty.rpm"}]}`
		_ = os.WriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON), 0600)
		_ = os.MkdirAll(filepath.Join(stagingDir, "packages"), 0700)
		_ = os.WriteFile(filepath.Join(stagingDir, "packages", "empty.rpm"), []byte(""), 0600)

		_, err = store.Seal(ctx, stagingDir, "op-empty-pkg")
		if !errors.Is(err, domain.ErrIncompleteArtifact) {
			t.Fatalf("expected ErrIncompleteArtifact for 0-byte package, got: %v", err)
		}
	})
}

func TestLocalFilesystemStore_PurgeAndGarbageCollection(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()

	store := storage.NewLocalFilesystemStore(
		storage.WithStoreBaseDir(baseDir),
	)

	staging1, _ := store.CreateStaging(ctx, "tx1")
	staging2, _ := store.CreateStaging(ctx, "tx2")
	staging3, _ := store.CreateStaging(ctx, "tx3")

	// Direct purge of staging1
	if err := store.Purge(ctx, staging1); err != nil {
		t.Fatalf("Purge failed: %v", err)
	}
	if _, err := os.Stat(staging1); !os.IsNotExist(err) {
		t.Errorf("expected staging1 to be deleted, but it still exists")
	}

	// Garbage collect: preserve staging2, clean up staging3
	purged, err := store.CollectGarbage(ctx, []string{staging2})
	if err != nil {
		t.Fatalf("CollectGarbage failed: %v", err)
	}
	if purged != 1 {
		t.Errorf("expected 1 directory purged, got %d", purged)
	}

	if _, err := os.Stat(staging2); os.IsNotExist(err) {
		t.Errorf("active directory staging2 was unexpectedly deleted")
	}
	if _, err := os.Stat(staging3); !os.IsNotExist(err) {
		t.Errorf("abandoned directory staging3 was not deleted")
	}
}

func TestLocalFilesystemStore_HardlinkRejection(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	outsideDir := t.TempDir()

	store := storage.NewLocalFilesystemStore(
		storage.WithStoreBaseDir(baseDir),
	)

	stagingDir, err := store.CreateStaging(ctx, "op-hardlink-test")
	if err != nil {
		t.Fatal(err)
	}

	txJSON := `{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`
	_ = os.WriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON), 0600)
	_ = os.MkdirAll(filepath.Join(stagingDir, "packages"), 0700)
	pkgFile := filepath.Join(stagingDir, "packages", "pkg.rpm")
	_ = os.WriteFile(pkgFile, []byte("REAL_RPM_PAYLOAD"), 0600)

	// Create hardlink outside the artifact:
	// artifact/packages/pkg.rpm <-> outside/pkg.rpm
	outsideFile := filepath.Join(outsideDir, "pkg.rpm")
	if err := os.Link(pkgFile, outsideFile); err != nil {
		t.Skipf("hardlinks not supported on this filesystem: %v", err)
	}

	// Seal MUST detect and reject the multi-link regular file (Nlink > 1)
	_, err = store.Seal(ctx, stagingDir, "op-hardlink-test")
	if !errors.Is(err, domain.ErrTransactionArtifactCorrupted) {
		t.Fatalf("expected ErrTransactionArtifactCorrupted when multi-link regular file is present, got: %v", err)
	}
}

func TestLocalFilesystemStore_GCProtectsSealedArtifacts(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()

	store := storage.NewLocalFilesystemStore(
		storage.WithStoreBaseDir(baseDir),
	)

	// Create an intact sealed artifact in baseDir
	stagingDir, err := store.CreateStaging(ctx, "op-sealed")
	if err != nil {
		t.Fatal(err)
	}
	txJSON := `{"rpms":[{"package_path":"./packages/pkg.rpm"}]}`
	_ = os.WriteFile(filepath.Join(stagingDir, "transaction.json"), []byte(txJSON), 0600)
	_ = os.MkdirAll(filepath.Join(stagingDir, "packages"), 0700)
	_ = os.WriteFile(filepath.Join(stagingDir, "packages", "pkg.rpm"), []byte("SEALED_PAYLOAD"), 0600)

	art, err := store.Seal(ctx, stagingDir, "op-sealed")
	if err != nil {
		t.Fatal(err)
	}

	// Create an abandoned incomplete staging directory
	stagingDebris, err := store.CreateStaging(ctx, "op-debris")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(stagingDebris, "partial.tmp"), []byte("partial"), 0600)

	// Run CollectGarbage with empty activeDirs (e.g. simulated journal mismatch or missing journal)
	purged, err := store.CollectGarbage(ctx, nil)
	if err != nil {
		t.Fatalf("CollectGarbage failed: %v", err)
	}

	// Incomplete staging debris should be purged
	if purged != 1 {
		t.Errorf("expected 1 abandoned directory purged, got %d", purged)
	}

	// The intact sealed artifact MUST survive even if unreferenced in activeDirs (preserves evidence)
	if _, err := os.Stat(art.Directory); os.IsNotExist(err) {
		t.Errorf("intact sealed artifact %s was unexpectedly deleted by GC", art.Directory)
	}

	// Verify FindSealedArtifacts detects it
	sealed, err := store.FindSealedArtifacts(ctx)
	if err != nil {
		t.Fatalf("FindSealedArtifacts failed: %v", err)
	}
	if len(sealed) != 1 || filepath.Clean(sealed[0]) != filepath.Clean(art.Directory) {
		t.Errorf("expected FindSealedArtifacts to return [%s], got: %v", art.Directory, sealed)
	}
}
