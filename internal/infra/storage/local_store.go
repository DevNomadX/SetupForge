package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
	"golang.org/x/sys/unix"
)

// LocalStoreOption configures a LocalFilesystemStore.
type LocalStoreOption func(*LocalFilesystemStore)

// WithStoreBaseDir overrides the transactions base directory.
func WithStoreBaseDir(baseDir string) LocalStoreOption {
	return func(s *LocalFilesystemStore) {
		s.baseDir = baseDir
	}
}

// WithStoreStatfs overrides the filesystem statfs query function for hermetic testing.
func WithStoreStatfs(statfs func(path string, stat *unix.Statfs_t) error) LocalStoreOption {
	return func(s *LocalFilesystemStore) {
		s.statfsFunc = statfs
	}
}

// LocalFilesystemStore manages physical transaction artifact storage on the local disk.
// Rooted under /var/tmp/setupforge-$UID/transactions by default with private 0700 permissions.
type LocalFilesystemStore struct {
	baseDir    string
	statfsFunc func(path string, stat *unix.Statfs_t) error
}

// NewLocalFilesystemStore creates an ArtifactStore for managing stored transaction bundles.
func NewLocalFilesystemStore(opts ...LocalStoreOption) *LocalFilesystemStore {
	store := &LocalFilesystemStore{
		baseDir:    defaultBaseDir(),
		statfsFunc: unix.Statfs,
	}

	for _, opt := range opts {
		opt(store)
	}

	return store
}

func defaultBaseDir() string {
	return filepath.Join("/var/tmp", fmt.Sprintf("setupforge-%d", os.Getuid()), "transactions")
}

// CheckCapacity performs preflight admission validation using statfs.
// INVARIANT: This is an admission guard, NOT an absolute immunity against runtime ENOSPC.
func (s *LocalFilesystemStore) CheckCapacity(ctx context.Context, estimatedBytes int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Find the nearest existing directory ancestor to query statfs
	checkPath := s.baseDir
	for {
		if _, err := os.Stat(checkPath); err == nil {
			break
		}
		parent := filepath.Dir(checkPath)
		if parent == checkPath {
			break
		}
		checkPath = parent
	}

	var stat unix.Statfs_t
	if err := s.statfsFunc(checkPath, &stat); err != nil {
		return fmt.Errorf("statfs %s: %w", checkPath, err)
	}

	availableBytes := int64(stat.Bavail) * int64(stat.Bsize)

	// Provisional formula: double payload size for extraction/unpack buffer + 100 MiB safety margin
	requiredBytes := estimatedBytes*2 + (100 * 1024 * 1024)
	if estimatedBytes <= 0 {
		// Baseline admission check when estimated size is not yet resolved: 500 MiB
		requiredBytes = 500 * 1024 * 1024
	}

	if availableBytes < requiredBytes {
		return fmt.Errorf("%w: filesystem at %s has %d bytes available, requires %d bytes",
			domain.ErrInsufficientStorage, checkPath, availableBytes, requiredBytes)
	}

	return nil
}

// CreateStaging creates an isolated, unprivileged staging directory for planning.
func (s *LocalFilesystemStore) CreateStaging(ctx context.Context, opID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if err := os.MkdirAll(s.baseDir, 0700); err != nil {
		return "", fmt.Errorf("%w: creating transactions base dir: %v", domain.ErrPlanningFailed, err)
	}

	stagingDir, err := os.MkdirTemp(s.baseDir, "tx-"+opID+"-*")
	if err != nil {
		return "", fmt.Errorf("%w: creating transaction temp dir: %v", domain.ErrPlanningFailed, err)
	}

	if err := os.Chmod(stagingDir, 0700); err != nil {
		_ = os.RemoveAll(stagingDir)
		return "", fmt.Errorf("%w: configuring staging directory permissions: %v", domain.ErrPlanningFailed, err)
	}

	return stagingDir, nil
}

// Seal validates the artifact structure, asserts zero symlinks/non-regular objects,
// computes the deterministic SHA-256 Merkle digest, and transitions the artifact to SEALED.
func (s *LocalFilesystemStore) Seal(ctx context.Context, stagingDir string, opID string) (domain.TransactionArtifact, error) {
	if err := ctx.Err(); err != nil {
		return domain.TransactionArtifact{}, err
	}

	info, err := os.Lstat(stagingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.TransactionArtifact{}, fmt.Errorf("%w: staging directory not found: %s", domain.ErrInvalidArtifact, stagingDir)
		}
		return domain.TransactionArtifact{}, fmt.Errorf("%w: stating staging directory: %v", domain.ErrUnreadableArtifact, err)
	}
	if !info.IsDir() || (info.Mode()&os.ModeSymlink != 0) {
		return domain.TransactionArtifact{}, fmt.Errorf("%w: staging path is not a regular directory: %s", domain.ErrInvalidArtifact, stagingDir)
	}

	// 1. Structural symlink & non-regular object defense
	if err := s.assertNoSymlinksOrSpecialObjects(stagingDir); err != nil {
		return domain.TransactionArtifact{}, err
	}

	// 2. Validate transaction.json
	txJsonPath := filepath.Join(stagingDir, "transaction.json")
	txData, err := os.ReadFile(txJsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.TransactionArtifact{}, fmt.Errorf("%w: missing transaction.json in %s", domain.ErrInvalidArtifact, stagingDir)
		}
		return domain.TransactionArtifact{}, fmt.Errorf("%w: reading transaction.json: %v", domain.ErrUnreadableArtifact, err)
	}

	pkgPaths, err := extractPackagePathsFromJSON(txData)
	if err != nil {
		return domain.TransactionArtifact{}, err
	}

	// 3. Validate expected package payloads
	var totalBytes int64
	for _, pkgPath := range pkgPaths {
		if pkgPath == "" {
			continue
		}
		fullPkgPath := filepath.Join(stagingDir, filepath.FromSlash(pkgPath))

		// Guard against directory traversal outside staging
		rel, err := filepath.Rel(stagingDir, fullPkgPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return domain.TransactionArtifact{}, fmt.Errorf("%w: package path escapes artifact root: %s", domain.ErrInvalidArtifact, pkgPath)
		}

		pInfo, err := os.Lstat(fullPkgPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return domain.TransactionArtifact{}, fmt.Errorf("%w: missing package payload %s", domain.ErrIncompleteArtifact, pkgPath)
			}
			return domain.TransactionArtifact{}, fmt.Errorf("%w: stating package payload %s: %v", domain.ErrUnreadableArtifact, pkgPath, err)
		}

		if !pInfo.Mode().IsRegular() {
			return domain.TransactionArtifact{}, fmt.Errorf("%w: package payload %s is not a regular file", domain.ErrIncompleteArtifact, pkgPath)
		}
		if pInfo.Size() == 0 {
			return domain.TransactionArtifact{}, fmt.Errorf("%w: package payload %s has zero bytes", domain.ErrIncompleteArtifact, pkgPath)
		}
		totalBytes += pInfo.Size()
	}

	// 4. Calculate deterministic SHA-256 Merkle root
	fingerprint, err := s.computeFingerprint(stagingDir)
	if err != nil {
		return domain.TransactionArtifact{}, fmt.Errorf("%w: computing fingerprint: %v", domain.ErrInvalidArtifact, err)
	}

	descriptor := domain.Descriptor{
		MediaType: domain.MediaTypeTransactionManifest,
		Digest:    "sha256:" + fingerprint,
		Size:      totalBytes,
		Annotations: map[string]string{
			"setupforge.io/operation-id":  opID,
			"setupforge.io/package-count": fmt.Sprintf("%d", len(pkgPaths)),
		},
	}

	return domain.TransactionArtifact{
		OperationID:  opID,
		Directory:    stagingDir,
		Fingerprint:  fingerprint,
		CreatedAt:    time.Now(),
		PackageCount: len(pkgPaths),
		TotalBytes:   totalBytes,
		State:        domain.ArtifactStateSealed,
		Descriptor:   descriptor,
	}, nil
}

// Validate verifies that a sealed artifact exists on disk, has intact files without symlinks,
// and matches its recorded fingerprint.
func (s *LocalFilesystemStore) Validate(ctx context.Context, artifact domain.TransactionArtifact) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if artifact.IsZero() {
		return domain.ErrPlanUnavailable
	}

	if err := artifact.Validate(); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidArtifact, err)
	}

	info, err := os.Lstat(artifact.Directory)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.ErrPlanUnavailable
		}
		return fmt.Errorf("%w: stating artifact directory: %v", domain.ErrUnreadableArtifact, err)
	}
	if !info.IsDir() || (info.Mode()&os.ModeSymlink != 0) {
		return fmt.Errorf("%w: artifact path is not a regular directory: %s", domain.ErrInvalidArtifact, artifact.Directory)
	}

	if err := s.assertNoSymlinksOrSpecialObjects(artifact.Directory); err != nil {
		return err
	}

	txJsonPath := filepath.Join(artifact.Directory, "transaction.json")
	txData, err := os.ReadFile(txJsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: missing transaction.json in %s", domain.ErrInvalidArtifact, artifact.Directory)
		}
		return fmt.Errorf("%w: reading transaction.json: %v", domain.ErrUnreadableArtifact, err)
	}

	pkgPaths, err := extractPackagePathsFromJSON(txData)
	if err != nil {
		return err
	}

	for _, pkgPath := range pkgPaths {
		if pkgPath == "" {
			continue
		}
		fullPkgPath := filepath.Join(artifact.Directory, filepath.FromSlash(pkgPath))

		rel, err := filepath.Rel(artifact.Directory, fullPkgPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("%w: package path escapes artifact root: %s", domain.ErrInvalidArtifact, pkgPath)
		}

		pInfo, err := os.Lstat(fullPkgPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%w: missing package payload %s", domain.ErrIncompleteArtifact, pkgPath)
			}
			return fmt.Errorf("%w: stating package payload %s: %v", domain.ErrUnreadableArtifact, pkgPath, err)
		}

		if !pInfo.Mode().IsRegular() || pInfo.Size() == 0 {
			return fmt.Errorf("%w: package payload %s invalid or empty", domain.ErrIncompleteArtifact, pkgPath)
		}
	}

	currentFingerprint, err := s.computeFingerprint(artifact.Directory)
	if err != nil {
		return fmt.Errorf("%w: computing fingerprint during validation: %v", domain.ErrInvalidArtifact, err)
	}

	if currentFingerprint != artifact.Fingerprint {
		return domain.ErrTransactionArtifactCorrupted
	}

	return nil
}

// Purge physically removes the artifact directory from disk.
func (s *LocalFilesystemStore) Purge(ctx context.Context, directory string) error {
	if directory == "" {
		return nil
	}

	// Path traversal protection: directory must reside under s.baseDir or match tx-* pattern
	cleanTarget := filepath.Clean(directory)
	cleanBase := filepath.Clean(s.baseDir)

	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return fmt.Errorf("refusing to purge directory outside transactions base: %s", directory)
	}

	if err := os.RemoveAll(cleanTarget); err != nil {
		return fmt.Errorf("purging artifact directory %s: %w", cleanTarget, err)
	}

	if _, err := os.Lstat(cleanTarget); err == nil {
		return fmt.Errorf("directory %s still exists after purge", cleanTarget)
	}

	return nil
}

// IsSealedArtifact checks whether a directory represents an intact sealed artifact bundle.
func (s *LocalFilesystemStore) IsSealedArtifact(dir string) bool {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || (info.Mode()&os.ModeSymlink != 0) {
		return false
	}

	if err := s.assertNoSymlinksOrSpecialObjects(dir); err != nil {
		return false
	}

	txJsonPath := filepath.Join(dir, "transaction.json")
	txData, err := os.ReadFile(txJsonPath)
	if err != nil {
		return false
	}

	pkgPaths, err := extractPackagePathsFromJSON(txData)
	if err != nil || len(pkgPaths) == 0 {
		return false
	}

	for _, pkgPath := range pkgPaths {
		if pkgPath == "" {
			continue
		}
		fullPkgPath := filepath.Join(dir, filepath.FromSlash(pkgPath))
		pInfo, err := os.Lstat(fullPkgPath)
		if err != nil || !pInfo.Mode().IsRegular() || pInfo.Size() == 0 {
			return false
		}
	}

	return true
}

// FindSealedArtifacts scans the base transactions directory and returns paths of all intact sealed artifacts.
func (s *LocalFilesystemStore) FindSealedArtifacts(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("reading base directory for sealed artifacts: %w", err)
	}

	var sealed []string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "tx-") {
			continue
		}
		fullPath := filepath.Join(s.baseDir, entry.Name())
		if s.IsSealedArtifact(fullPath) {
			sealed = append(sealed, fullPath)
		}
	}

	return sealed, nil
}

// CollectGarbage removes abandoned or unreferenced transaction directories based on
// ownership and journal state.
func (s *LocalFilesystemStore) CollectGarbage(ctx context.Context, activeDirs []string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return 0, nil
	}

	activeSet := make(map[string]bool)
	for _, d := range activeDirs {
		if d != "" {
			activeSet[filepath.Clean(d)] = true
		}
	}

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return 0, fmt.Errorf("reading base directory for garbage collection: %w", err)
	}

	purged := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "tx-") {
			continue
		}

		fullPath := filepath.Join(s.baseDir, entry.Name())
		if activeSet[filepath.Clean(fullPath)] {
			continue
		}

		// Invariant: An artifact referenced by an active transaction must never be reclaimed.
		// Furthermore, if an unreferenced directory is an intact sealed artifact (not abandoned staging debris),
		// preserve it as evidence of an unrecorded/mismatched transaction.
		if s.IsSealedArtifact(fullPath) {
			continue
		}

		// Safe to purge incomplete staging directories (no valid transaction.json or incomplete packages)
		if err := os.RemoveAll(fullPath); err == nil {
			purged++
		}
	}

	return purged, nil
}

// assertNoSymlinksOrSpecialObjects verifies that no symlinks, FIFOs, sockets, device nodes,
// or unexpected multi-link regular files exist within the directory tree.
//
// Invariant: An artifact regular file must not be externally mutable through another hardlink.
func (s *LocalFilesystemStore) assertNoSymlinksOrSpecialObjects(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Explicit Lstat to examine the actual directory entry without resolving targets
		info, lstatErr := os.Lstat(path)
		if lstatErr != nil {
			return fmt.Errorf("%w: lstat %s: %v", domain.ErrUnreadableArtifact, path, lstatErr)
		}

		mode := info.Mode()

		// Hard rejection of symlinks
		if mode&os.ModeSymlink != 0 || d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink rejected at %s", domain.ErrTransactionArtifactCorrupted, path)
		}

		// Hard rejection of sockets, pipes, devices
		if mode&(os.ModeSocket|os.ModeNamedPipe|os.ModeDevice|os.ModeCharDevice) != 0 {
			return fmt.Errorf("%w: non-regular filesystem object rejected at %s", domain.ErrTransactionArtifactCorrupted, path)
		}

		// Only directories and regular files are permitted
		if !info.IsDir() && !mode.IsRegular() {
			return fmt.Errorf("%w: unsupported filesystem entry type at %s", domain.ErrTransactionArtifactCorrupted, path)
		}

		// Hardlink defense on regular files: link count must be exactly 1
		// Invariant: An artifact regular file must not be externally mutable through another hardlink.
		if mode.IsRegular() {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				if stat.Nlink > 1 {
					return fmt.Errorf("%w: hardlink rejected on regular file at %s (link count %d > 1)",
						domain.ErrTransactionArtifactCorrupted, path, stat.Nlink)
				}
			}
		}

		return nil
	})
}

// computeFingerprint calculates the canonical deterministic SHA-256 Merkle root digest
// covering all regular files within the directory (ADR-008 Case A).
func (s *LocalFilesystemStore) computeFingerprint(dir string) (string, error) {
	var relPaths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		relPaths = append(relPaths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("%w: walking artifact directory: %v", domain.ErrUnreadableArtifact, err)
	}

	if len(relPaths) == 0 {
		return "", fmt.Errorf("%w: artifact directory contains no regular files: %s", domain.ErrInvalidArtifact, dir)
	}

	sort.Strings(relPaths)

	hasher := sha256.New()
	for _, relPath := range relPaths {
		fullPath := filepath.Join(dir, filepath.FromSlash(relPath))

		f, err := os.Open(fullPath)
		if err != nil {
			return "", fmt.Errorf("%w: opening artifact file %s: %v", domain.ErrUnreadableArtifact, relPath, err)
		}

		fInfo, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return "", fmt.Errorf("%w: stating artifact file %s: %v", domain.ErrUnreadableArtifact, relPath, err)
		}

		// Framing: PATH:<len>:<path>\nSIZE:<size>\n<content_bytes>\nEND\n
		fmt.Fprintf(hasher, "PATH:%d:%s\n", len(relPath), relPath)
		fmt.Fprintf(hasher, "SIZE:%d\n", fInfo.Size())

		if _, err := io.Copy(hasher, f); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("%w: reading artifact file %s: %v", domain.ErrUnreadableArtifact, relPath, err)
		}

		fmt.Fprint(hasher, "\nEND\n")

		if err := f.Close(); err != nil {
			return "", fmt.Errorf("%w: closing artifact file %s: %v", domain.ErrUnreadableArtifact, relPath, err)
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type rawTransactionJSON struct {
	RPMs []struct {
		PackagePath string `json:"package_path"`
	} `json:"rpms"`
}

func extractPackagePathsFromJSON(data []byte) ([]string, error) {
	var raw rawTransactionJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%w: decoding transaction.json: %v", domain.ErrInvalidArtifact, err)
	}

	var paths []string
	for _, item := range raw.RPMs {
		if item.PackagePath != "" {
			paths = append(paths, item.PackagePath)
		}
	}
	return paths, nil
}
