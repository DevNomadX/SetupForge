package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/setupforge/setupforge/internal/domain"
)

func TestParseNEVRA(t *testing.T) {
	tests := []struct {
		nevra       string
		wantName    string
		wantEpoch   string
		wantVersion string
		wantRelease string
		wantArch    string
		wantEVR     string
		wantErr     bool
	}{
		{
			nevra:       "libmaxminddb-1.14.1-1.fc44.x86_64",
			wantName:    "libmaxminddb",
			wantEpoch:   "0",
			wantVersion: "1.14.1",
			wantRelease: "1.fc44",
			wantArch:    "x86_64",
			wantEVR:     "1.14.1-1.fc44",
		},
		{
			nevra:       "blivet-data-1:3.14.2-1.fc44.noarch",
			wantName:    "blivet-data",
			wantEpoch:   "1",
			wantVersion: "3.14.2",
			wantRelease: "1.fc44",
			wantArch:    "noarch",
			wantEVR:     "1:3.14.2-1.fc44",
		},
		{
			nevra:       "google-noto-sans-cjk-ttc-fonts-20230817-2.fc44.noarch",
			wantName:    "google-noto-sans-cjk-ttc-fonts",
			wantEpoch:   "0",
			wantVersion: "20230817",
			wantRelease: "2.fc44",
			wantArch:    "noarch",
			wantEVR:     "20230817-2.fc44",
		},
		{
			nevra:       "kernel-core-0:6.14.0-64.fc44.x86_64",
			wantName:    "kernel-core",
			wantEpoch:   "0",
			wantVersion: "6.14.0",
			wantRelease: "64.fc44",
			wantArch:    "x86_64",
			wantEVR:     "6.14.0-64.fc44",
		},
		{
			nevra:   "invalid",
			wantErr: true,
		},
		{
			nevra:   "missingarch",
			wantErr: true,
		},
		{
			nevra:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.nevra, func(t *testing.T) {
			got, err := ParseNEVRA(tt.nevra)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.nevra)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.nevra, err)
			}
			if got.Name != tt.wantName || got.Epoch != tt.wantEpoch || got.Version != tt.wantVersion ||
				got.Release != tt.wantRelease || got.Arch != tt.wantArch || got.EVR != tt.wantEVR {
				t.Errorf("ParseNEVRA(%q) = %+v, want name=%q epoch=%q version=%q release=%q arch=%q evr=%q",
					tt.nevra, got, tt.wantName, tt.wantEpoch, tt.wantVersion, tt.wantRelease, tt.wantArch, tt.wantEVR)
			}
		})
	}
}

func TestParseTransactionJSON_ValidMultiPackage(t *testing.T) {
	tempDir := t.TempDir()
	pkgDir := filepath.Join(tempDir, "packages")
	_ = os.MkdirAll(pkgDir, 0700)

	// Write mock RPMs to measure file sizes
	rpm1Path := filepath.Join(pkgDir, "firefox-156.0.1-2.fc44.x86_64.rpm")
	_ = os.WriteFile(rpm1Path, []byte("MOCK_FIREFOX_CONTENT_BYTES"), 0600)
	rpm2Path := filepath.Join(pkgDir, "kernel-core-6.14.0-64.fc44.x86_64.rpm")
	_ = os.WriteFile(rpm2Path, []byte("MOCK_KERNEL_CORE_BYTES_123"), 0600)

	txJson := `{
  "version": "1.0",
  "rpms": [
    {
      "nevra": "firefox-156.0.1-2.fc44.x86_64",
      "action": "Upgrade",
      "reason": "External User",
      "repo_id": "@stored_transaction(updates)",
      "package_path": "./packages/firefox-156.0.1-2.fc44.x86_64.rpm"
    },
    {
      "nevra": "firefox-156.0.1-1.fc44.x86_64",
      "action": "Replaced",
      "reason": "External User",
      "repo_id": "@System"
    },
    {
      "nevra": "kernel-core-6.14.0-64.fc44.x86_64",
      "action": "Install",
      "reason": "Dependency",
      "repo_id": "@stored_transaction(updates)",
      "package_path": "./packages/kernel-core-6.14.0-64.fc44.x86_64.rpm"
    },
    {
      "nevra": "old-tool-1.0.0-1.fc44.x86_64",
      "action": "Remove",
      "reason": "External User",
      "repo_id": "@System"
    }
  ]
}`

	plan, err := ParseTransactionJSON([]byte(txJson), tempDir, "test-op-123")
	if err != nil {
		t.Fatalf("ParseTransactionJSON failed: %v", err)
	}

	if plan.OperationID != "test-op-123" {
		t.Errorf("expected OperationID test-op-123, got %s", plan.OperationID)
	}
	if plan.UpgradeCount != 1 {
		t.Errorf("expected 1 upgrade, got %d", plan.UpgradeCount)
	}
	if plan.InstallCount != 1 {
		t.Errorf("expected 1 install, got %d", plan.InstallCount)
	}
	if plan.RemoveCount != 1 {
		t.Errorf("expected 1 remove, got %d", plan.RemoveCount)
	}
	if plan.ReplacedCount != 1 {
		t.Errorf("expected 1 replaced, got %d", plan.ReplacedCount)
	}

	// Verify pairing of Upgrade with Replaced
	var firefoxPkg *domain.PlannedPackage
	for i := range plan.Packages {
		if plan.Packages[i].Name == "firefox" && plan.Packages[i].Action == domain.ActionUpgrade {
			firefoxPkg = &plan.Packages[i]
			break
		}
	}
	if firefoxPkg == nil {
		t.Fatal("expected firefox upgrade package in plan")
	}
	if firefoxPkg.OldNEVRA != "firefox-156.0.1-1.fc44.x86_64" {
		t.Errorf("expected OldNEVRA to be paired from Replaced, got %s", firefoxPkg.OldNEVRA)
	}
	if firefoxPkg.OldEVR != "156.0.1-1.fc44" {
		t.Errorf("expected OldEVR 156.0.1-1.fc44, got %s", firefoxPkg.OldEVR)
	}
	if firefoxPkg.EVR != "156.0.1-2.fc44" {
		t.Errorf("expected EVR 156.0.1-2.fc44, got %s", firefoxPkg.EVR)
	}
	if firefoxPkg.RepoID != "updates" {
		t.Errorf("expected clean repo ID 'updates', got %s", firefoxPkg.RepoID)
	}
	if firefoxPkg.SizeBytes != int64(len("MOCK_FIREFOX_CONTENT_BYTES")) {
		t.Errorf("expected size %d, got %d", len("MOCK_FIREFOX_CONTENT_BYTES"), firefoxPkg.SizeBytes)
	}

	// Verify Kernel flag detection
	if !plan.HasKernelUpdate {
		t.Errorf("expected HasKernelUpdate to be true for kernel-core")
	}
	if plan.HasSystemdUpdate {
		t.Errorf("expected HasSystemdUpdate to be false when systemd is not in transaction")
	}

	// Total package count helper
	if plan.TotalPackageCount() != 3 {
		t.Errorf("expected TotalPackageCount() to be 3 (1 upgrade + 1 install + 1 remove), got %d", plan.TotalPackageCount())
	}
}

func TestParseTransactionJSON_ErrorHandling(t *testing.T) {
	// Empty data
	_, err := ParseTransactionJSON(nil, "", "op")
	if !errors.Is(err, domain.ErrInvalidArtifact) {
		t.Errorf("expected ErrInvalidArtifact on nil data, got %v", err)
	}

	// Malformed JSON
	_, err = ParseTransactionJSON([]byte("{bad-json"), "", "op")
	if !errors.Is(err, domain.ErrInvalidArtifact) {
		t.Errorf("expected ErrInvalidArtifact on malformed JSON, got %v", err)
	}

	// Empty RPM list
	_, err = ParseTransactionJSON([]byte(`{"version":"1.0","rpms":[]}`), "", "op")
	if !errors.Is(err, domain.ErrEmptyTransaction) {
		t.Errorf("expected ErrEmptyTransaction on empty rpms array, got %v", err)
	}

	// Unknown action
	unknownActionJson := `{
  "version": "1.0",
  "rpms": [{"nevra":"pkg-1.0-1.fc44.x86_64", "action":"SuperDestructiveOperation"}]
}`
	_, err = ParseTransactionJSON([]byte(unknownActionJson), "", "op")
	if !errors.Is(err, domain.ErrUnknownAction) {
		t.Errorf("expected ErrUnknownAction on unrecognized action, got %v", err)
	}
}

func TestExtractPackagePaths(t *testing.T) {
	txJson := `{
  "version": "1.0",
  "rpms": [
    {"nevra":"a-1-1.fc44.x86_64", "package_path":"./packages/a.rpm"},
    {"nevra":"b-1-1.fc44.x86_64"},
    {"nevra":"c-1-1.fc44.x86_64", "package_path":"./packages/c.rpm"}
  ]
}`
	paths, err := ExtractPackagePaths([]byte(txJson))
	if err != nil {
		t.Fatalf("ExtractPackagePaths failed: %v", err)
	}
	if len(paths) != 2 || paths[0] != "./packages/a.rpm" || paths[1] != "./packages/c.rpm" {
		t.Errorf("unexpected extracted paths: %v", paths)
	}
}
