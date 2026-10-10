package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
)

// dnf5TransactionJSON matches the authoritative schema produced by DNF5 in transaction.json.
type dnf5TransactionJSON struct {
	Version string               `json:"version"`
	RPMs    []dnf5TransactionRPM `json:"rpms"`
}

// dnf5TransactionRPM represents an individual RPM entry in DNF5's transaction.json.
type dnf5TransactionRPM struct {
	NEVRA       string `json:"nevra"`
	Action      string `json:"action"`
	Reason      string `json:"reason"`
	RepoID      string `json:"repo_id"`
	PackagePath string `json:"package_path,omitempty"`
}

// ParsedNEVRA contains the decomposed parts of a standard RPM NEVRA string.
type ParsedNEVRA struct {
	Name    string
	Epoch   string
	Version string
	Release string
	Arch    string
	EVR     string
}

// ParseNEVRA decomposes a standard RPM NEVRA string (name-[epoch:]version-release.arch)
// into its constituent components.
func ParseNEVRA(nevra string) (ParsedNEVRA, error) {
	trimmed := strings.TrimSpace(nevra)
	if trimmed == "" {
		return ParsedNEVRA{}, fmt.Errorf("empty NEVRA string")
	}

	// Arch is after the last '.'
	dotIdx := strings.LastIndex(trimmed, ".")
	if dotIdx == -1 || dotIdx == len(trimmed)-1 {
		return ParsedNEVRA{}, fmt.Errorf("missing architecture in NEVRA: %s", nevra)
	}
	arch := trimmed[dotIdx+1:]
	rem := trimmed[:dotIdx]

	// Release is after the last '-' in the remainder
	lastHyphen := strings.LastIndex(rem, "-")
	if lastHyphen == -1 || lastHyphen == len(rem)-1 {
		return ParsedNEVRA{}, fmt.Errorf("missing release in NEVRA: %s", nevra)
	}
	release := rem[lastHyphen+1:]
	rem = rem[:lastHyphen]

	// Version/Epoch is after the last '-' in the remaining string
	verHyphen := strings.LastIndex(rem, "-")
	if verHyphen == -1 || verHyphen == len(rem)-1 {
		return ParsedNEVRA{}, fmt.Errorf("missing version in NEVRA: %s", nevra)
	}
	verEpoch := rem[verHyphen+1:]
	name := rem[:verHyphen]

	if name == "" {
		return ParsedNEVRA{}, fmt.Errorf("missing package name in NEVRA: %s", nevra)
	}

	epoch := "0"
	version := verEpoch
	if colonIdx := strings.Index(verEpoch, ":"); colonIdx != -1 {
		epoch = verEpoch[:colonIdx]
		version = verEpoch[colonIdx+1:]
	}

	evr := version + "-" + release
	if epoch != "0" && epoch != "" {
		evr = epoch + ":" + version + "-" + release
	}

	return ParsedNEVRA{
		Name:    name,
		Epoch:   epoch,
		Version: version,
		Release: release,
		Arch:    arch,
		EVR:     evr,
	}, nil
}

// MapAction converts a DNF5 raw transaction action string into a typed domain.PackageAction.
// Fails closed if the action is unrecognized.
func MapAction(raw string) (domain.PackageAction, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "upgrade":
		return domain.ActionUpgrade, nil
	case "install":
		return domain.ActionInstall, nil
	case "remove":
		return domain.ActionRemove, nil
	case "replaced":
		return domain.ActionReplaced, nil
	case "downgrade":
		return domain.ActionDowngrade, nil
	default:
		return "", fmt.Errorf("%w: %q", domain.ErrUnknownAction, raw)
	}
}

// ParseTransactionJSON parses raw transaction.json data and builds a domain.UpdatePlan.
// If artifactDir is provided, package sizes are verified and extracted from disk.
func ParseTransactionJSON(data []byte, artifactDir string, opID string) (domain.UpdatePlan, error) {
	if len(data) == 0 {
		return domain.UpdatePlan{}, fmt.Errorf("%w: transaction.json is empty", domain.ErrInvalidArtifact)
	}

	var raw dnf5TransactionJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.UpdatePlan{}, fmt.Errorf("%w: decoding transaction.json: %v", domain.ErrInvalidArtifact, err)
	}

	if len(raw.RPMs) == 0 {
		return domain.UpdatePlan{}, domain.ErrEmptyTransaction
	}

	// First pass: index "Replaced" entries by package name to pair with "Upgrade"/"Downgrade"
	replacedMap := make(map[string]dnf5TransactionRPM)
	for _, item := range raw.RPMs {
		action, err := MapAction(item.Action)
		if err != nil {
			return domain.UpdatePlan{}, err
		}
		if action == domain.ActionReplaced {
			parsed, err := ParseNEVRA(item.NEVRA)
			if err != nil {
				return domain.UpdatePlan{}, fmt.Errorf("%w: invalid replaced NEVRA: %v", domain.ErrInvalidArtifact, err)
			}
			replacedMap[parsed.Name] = item
		}
	}

	var packages []domain.PlannedPackage
	packagePaths := make([]string, 0, len(raw.RPMs))

	for _, item := range raw.RPMs {
		action, err := MapAction(item.Action)
		if err != nil {
			return domain.UpdatePlan{}, err
		}

		parsed, err := ParseNEVRA(item.NEVRA)
		if err != nil {
			return domain.UpdatePlan{}, fmt.Errorf("%w: invalid RPM NEVRA %q: %v", domain.ErrInvalidArtifact, item.NEVRA, err)
		}

		var sizeBytes int64
		if item.PackagePath != "" {
			packagePaths = append(packagePaths, item.PackagePath)
			if artifactDir != "" {
				fullPath := filepath.Join(artifactDir, filepath.FromSlash(item.PackagePath))
				info, err := os.Stat(fullPath)
				if err == nil {
					sizeBytes = info.Size()
				}
			}
		}

		oldNEVRA := ""
		oldEVR := ""
		if action == domain.ActionUpgrade || action == domain.ActionDowngrade {
			if repl, ok := replacedMap[parsed.Name]; ok {
				oldNEVRA = repl.NEVRA
				if replParsed, err := ParseNEVRA(repl.NEVRA); err == nil {
					oldEVR = replParsed.EVR
				}
			}
		}

		cleanRepoID := strings.TrimPrefix(item.RepoID, "@stored_transaction(")
		cleanRepoID = strings.TrimSuffix(cleanRepoID, ")")

		packages = append(packages, domain.PlannedPackage{
			Name:        parsed.Name,
			Action:      action,
			NEVRA:       item.NEVRA,
			OldNEVRA:    oldNEVRA,
			Arch:        parsed.Arch,
			EVR:         parsed.EVR,
			OldEVR:      oldEVR,
			RepoID:      cleanRepoID,
			Reason:      item.Reason,
			PackagePath: item.PackagePath,
			SizeBytes:   sizeBytes,
		})
	}

	upgrades, installs, removes, replaced, downgrades, totalBytes, hasKernel, hasSystemd := domain.DeriveUpdatePlanMetadata(packages)

	return domain.UpdatePlan{
		OperationID:        opID,
		Packages:           packages,
		UpgradeCount:       upgrades,
		InstallCount:       installs,
		RemoveCount:        removes,
		ReplacedCount:      replaced,
		DowngradeCount:     downgrades,
		TotalDownloadBytes: totalBytes,
		HasKernelUpdate:    hasKernel,
		HasSystemdUpdate:   hasSystemd,
		CreatedAt:          time.Now(),
	}, nil
}

// ExtractPackagePaths returns the list of all package paths referenced in a transaction.json payload.
func ExtractPackagePaths(data []byte) ([]string, error) {
	var raw dnf5TransactionJSON
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
