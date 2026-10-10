package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/privilege"
	"github.com/setupforge/setupforge/internal/infra/storage"
)

// DNF5PlannerOption configures DNF5UpdatePlanner.
type DNF5PlannerOption func(*DNF5UpdatePlanner)

// WithPlannerRawExecutor overrides the command executor for testing.
func WithPlannerRawExecutor(exec privilege.RawCommandExecutor) DNF5PlannerOption {
	return func(p *DNF5UpdatePlanner) {
		p.rawExec = exec
	}
}

// WithPlannerEuidGetter overrides the EUID getter for testing.
func WithPlannerEuidGetter(geteuid func() int) DNF5PlannerOption {
	return func(p *DNF5UpdatePlanner) {
		p.geteuid = geteuid
	}
}

// WithPlannerBaseDir overrides the base directory where transaction artifacts are created.
func WithPlannerBaseDir(baseDir string) DNF5PlannerOption {
	return func(p *DNF5UpdatePlanner) {
		p.baseDir = baseDir
	}
}

// WithPlannerArtifactStore configures the ArtifactStore backing transaction persistence.
func WithPlannerArtifactStore(store domain.ArtifactStore) DNF5PlannerOption {
	return func(p *DNF5UpdatePlanner) {
		p.store = store
	}
}

// DNF5UpdatePlanner implements domain.UpdatePlanner using DNF5's stored transaction mechanism.
// It executes least-privilege unprivileged transaction resolution without mutating system packages.
type DNF5UpdatePlanner struct {
	dnf5Path string
	rawExec  privilege.RawCommandExecutor
	geteuid  func() int
	baseDir  string
	store    domain.ArtifactStore
}

// NewDNF5UpdatePlanner creates an update planner pointing to the host's dnf5 binary.
func NewDNF5UpdatePlanner(opts ...DNF5PlannerOption) *DNF5UpdatePlanner {
	path, err := exec.LookPath("dnf5")
	if err != nil {
		path = "/usr/bin/dnf5"
	}

	p := &DNF5UpdatePlanner{
		dnf5Path: path,
		rawExec:  privilege.DefaultRawExecutor,
		geteuid:  os.Geteuid,
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.store == nil {
		if p.baseDir != "" {
			p.store = storage.NewLocalFilesystemStore(storage.WithStoreBaseDir(p.baseDir))
		} else {
			p.store = storage.NewLocalFilesystemStore()
		}
	}

	return p
}

// Plan resolves the update transaction and stores it in an isolated artifact directory.
// Zero system packages are modified during this phase.
func (p *DNF5UpdatePlanner) Plan(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error) {
	if err := ctx.Err(); err != nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, err
	}

	if p.dnf5Path == "" {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, domain.ErrDNF5NotFound
	}

	if opID == "" {
		opID = fmt.Sprintf("sf-upd-%d", time.Now().UnixNano())
	}

	stagingDir, err := p.store.CreateStaging(ctx, opID)
	if err != nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, err
	}

	// Clean up staging directory on failure
	cleanup := true
	defer func() {
		if cleanup {
			_ = p.store.Purge(ctx, stagingDir)
		}
	}()

	// Build structured DNF5 arguments (never shell interpolation)
	args := []string{
		"upgrade",
		"--store=" + stagingDir,
		"-y",
	}

	geteuid := p.geteuid
	if geteuid == nil {
		geteuid = os.Geteuid
	}
	rawExec := p.rawExec
	if rawExec == nil {
		rawExec = privilege.DefaultRawExecutor
	}

	// Least-privilege unprivileged planning: point system_state_dir away from root-only packages.toml
	if geteuid() != 0 {
		stateDir := "/var/empty"
		if _, statErr := os.Stat(stateDir); statErr != nil {
			stateDir = os.TempDir()
		}
		args = append(args, "--setopt=system_state_dir="+stateDir)
	}

	stdout, stderr, exitCode, runErr := rawExec(ctx, p.dnf5Path, args...)

	if ctx.Err() != nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, ctx.Err()
	}

	if runErr != nil && (errors.Is(runErr, exec.ErrNotFound) || errors.Is(runErr, privilege.ErrSudoNotFound)) {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, domain.ErrDNF5NotFound
	}

	if exitCode != 0 {
		errMsg := strings.TrimSpace(string(stderr))
		if errMsg == "" && runErr != nil {
			errMsg = runErr.Error()
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("dnf5 upgrade --store exited with code %d", exitCode)
		}
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("%w: %s", domain.ErrPlanningFailed, errMsg)
	}

	txJsonPath := filepath.Join(stagingDir, "transaction.json")
	txData, err := os.ReadFile(txJsonPath)
	if err != nil {
		// If transaction.json is missing after exit code 0, check if system is already up to date
		stdoutStr := string(stdout)
		if strings.Contains(stdoutStr, "Nothing to do") {
			return domain.TransactionArtifact{}, domain.UpdatePlan{}, domain.ErrEmptyTransaction
		}
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, fmt.Errorf("%w: missing transaction.json after successful exit: %v", domain.ErrInvalidArtifact, err)
	}

	// Delegate structure validation, symlink rejection, and SHA-256 Merkle sealing to ArtifactStore
	artifact, err := p.store.Seal(ctx, stagingDir, opID)
	if err != nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, err
	}

	// Parse transaction.json into domain.UpdatePlan
	plan, err := ParseTransactionJSON(txData, stagingDir, opID)
	if err != nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, err
	}

	plan.Artifact = artifact

	// Planning succeeded; retain the sealed artifact directory for review and replay
	cleanup = false

	return artifact, plan, nil
}

// ValidateArtifact verifies that the stored artifact still exists on disk, has valid structure,
// and matches its recorded fingerprint without modification.
func (p *DNF5UpdatePlanner) ValidateArtifact(artifact domain.TransactionArtifact) error {
	if p.store == nil {
		return domain.ErrPlanUnavailable
	}
	return p.store.Validate(context.Background(), artifact)
}
