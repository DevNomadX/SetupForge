package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/setupforge/setupforge/internal/domain"
)

// IsReplayRelevantArtifactFile determines whether a filesystem entry within the artifact
// root is replay-relevant and must be included in the deterministic fingerprint.
//
// Based on verified DNF5 5.4.6 behavior, all regular files within the isolated artifact
// directory are replay-relevant. DNF5 does not create temporary or lock files in the store directory,
// and no filename-based exclusions (.tmp, .lock) are permitted so that any unexpected, added,
// or altered regular file will immediately trigger an integrity mismatch under Reviewed == Executed.
func IsReplayRelevantArtifactFile(d fs.DirEntry) bool {
	return d.Type().IsRegular()
}

// ComputeArtifactFingerprint calculates a deterministic Merkle-style streaming SHA-256
// digest covering all regular files within the artifact root directory.
//
// Properties guaranteed:
// 1. Complete coverage: Every regular file within the artifact root is hashed (Case A). No filename-based exclusions.
// 2. Filesystem enumeration independence: Relative paths are sorted in strict lexicographical byte order.
// 3. Cross-platform path normalization: Relative paths use canonical forward slashes ('/').
// 4. Collision-resistant framing: File paths and sizes are length-prefixed (PATH:<len>:<path>\nSIZE:<size>\n).
// 5. Memory efficiency: File contents are streamed in buffered chunks without buffering large RPMs in RAM.
// 6. Excludes non-regular filesystem objects (fifos, sockets, pipes, character/block devices).
func ComputeArtifactFingerprint(artifactDir string) (string, error) {
	info, err := os.Stat(artifactDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: directory does not exist: %s", domain.ErrInvalidArtifact, artifactDir)
		}
		return "", fmt.Errorf("%w: stating artifact directory: %v", domain.ErrUnreadableArtifact, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: artifact path is not a directory: %s", domain.ErrInvalidArtifact, artifactDir)
	}

	var relPaths []string
	err = filepath.WalkDir(artifactDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		if !IsReplayRelevantArtifactFile(d) {
			return nil
		}

		rel, err := filepath.Rel(artifactDir, path)
		if err != nil {
			return err
		}

		// Canonical forward-slash relative path
		normRel := filepath.ToSlash(rel)
		relPaths = append(relPaths, normRel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("%w: walking artifact directory: %v", domain.ErrUnreadableArtifact, err)
	}

	if len(relPaths) == 0 {
		return "", fmt.Errorf("%w: artifact directory contains no regular files: %s", domain.ErrInvalidArtifact, artifactDir)
	}

	// Deterministic lexicographical sort ensures directory traversal order has zero effect on the digest
	sort.Strings(relPaths)

	hasher := sha256.New()

	for _, relPath := range relPaths {
		fullPath := filepath.Join(artifactDir, filepath.FromSlash(relPath))

		f, err := os.Open(fullPath)
		if err != nil {
			return "", fmt.Errorf("%w: opening artifact file %s: %v", domain.ErrUnreadableArtifact, relPath, err)
		}

		fInfo, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return "", fmt.Errorf("%w: stating artifact file %s: %v", domain.ErrUnreadableArtifact, relPath, err)
		}

		// Framing: PATH:<len>:<path>\nSIZE:<size>\n
		fmt.Fprintf(hasher, "PATH:%d:%s\n", len(relPath), relPath)
		fmt.Fprintf(hasher, "SIZE:%d\n", fInfo.Size())

		// Stream content in buffered chunks (zero RAM bloat for large RPMs)
		if _, err := io.Copy(hasher, f); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("%w: reading artifact file %s: %v", domain.ErrUnreadableArtifact, relPath, err)
		}

		// Delimiter framing
		fmt.Fprint(hasher, "\nEND\n")

		if err := f.Close(); err != nil {
			return "", fmt.Errorf("%w: closing artifact file %s: %v", domain.ErrUnreadableArtifact, relPath, err)
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ValidateArtifactStructure verifies that transaction.json exists and that all required package
// files referenced by the transaction exist on disk with non-zero size.
func ValidateArtifactStructure(artifactDir string, packagePaths []string) error {
	info, err := os.Stat(artifactDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: artifact directory not found: %s", domain.ErrInvalidArtifact, artifactDir)
		}
		return fmt.Errorf("%w: stating artifact directory: %v", domain.ErrUnreadableArtifact, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: artifact path is not a directory: %s", domain.ErrInvalidArtifact, artifactDir)
	}

	// transaction.json must exist and be a non-empty regular file
	txJsonPath := filepath.Join(artifactDir, "transaction.json")
	txInfo, err := os.Stat(txJsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: missing transaction.json in %s", domain.ErrInvalidArtifact, artifactDir)
		}
		return fmt.Errorf("%w: reading transaction.json: %v", domain.ErrUnreadableArtifact, err)
	}
	if !txInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: transaction.json is not a regular file", domain.ErrInvalidArtifact)
	}
	if txInfo.Size() == 0 {
		return fmt.Errorf("%w: transaction.json is zero bytes", domain.ErrInvalidArtifact)
	}

	// Validate each expected package file
	for _, pkgPath := range packagePaths {
		if pkgPath == "" {
			continue
		}

		fullPkgPath := filepath.Join(artifactDir, filepath.FromSlash(pkgPath))

		// Prevent path traversal outside artifact root
		rel, err := filepath.Rel(artifactDir, fullPkgPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("%w: package path escapes artifact root: %s", domain.ErrInvalidArtifact, pkgPath)
		}

		pInfo, err := os.Stat(fullPkgPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%w: missing package payload %s", domain.ErrIncompleteArtifact, pkgPath)
			}
			return fmt.Errorf("%w: reading package payload %s: %v", domain.ErrUnreadableArtifact, pkgPath, err)
		}
		if !pInfo.Mode().IsRegular() {
			return fmt.Errorf("%w: package payload %s is not a regular file", domain.ErrIncompleteArtifact, pkgPath)
		}
		if pInfo.Size() == 0 {
			return fmt.Errorf("%w: package payload %s has zero bytes", domain.ErrIncompleteArtifact, pkgPath)
		}
	}

	return nil
}
