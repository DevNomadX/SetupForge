package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/privilege"
)

// DNF5CheckerOption configures DNF5UpdateChecker.
type DNF5CheckerOption func(*DNF5UpdateChecker)

// WithRawExecutor overrides the low-level process executor.
func WithRawExecutor(exec privilege.RawCommandExecutor) DNF5CheckerOption {
	return func(c *DNF5UpdateChecker) {
		c.rawExec = exec
	}
}

// WithEuidGetter overrides the EUID provider.
func WithEuidGetter(geteuid func() int) DNF5CheckerOption {
	return func(c *DNF5UpdateChecker) {
		c.geteuid = geteuid
	}
}

// DNF5UpdateChecker implements domain.UpdateChecker by querying DNF5.
// For unprivileged users, it executes least-privilege read-only inspections
// overriding system_state_dir to avoid root-only packages.toml locks, falling
// back to the privileged runner if necessary.
type DNF5UpdateChecker struct {
	dnf5Path   string
	privRunner privilege.PrivilegedRunner
	rawExec    privilege.RawCommandExecutor
	geteuid    func() int
}

// NewDNF5UpdateChecker creates an update checker pointing to the host's dnf5 binary.
func NewDNF5UpdateChecker() *DNF5UpdateChecker {
	path, err := exec.LookPath("dnf5")
	if err != nil {
		path = "/usr/bin/dnf5"
	}
	return &DNF5UpdateChecker{
		dnf5Path:   path,
		privRunner: privilege.NewSudoRunner(),
		rawExec:    privilege.DefaultRawExecutor,
		geteuid:    os.Geteuid,
	}
}

// NewCustomDNF5UpdateChecker creates a DNF5UpdateChecker with custom dependencies.
// Useful for hermetic unit testing.
func NewCustomDNF5UpdateChecker(dnf5Path string, privRunner privilege.PrivilegedRunner, opts ...DNF5CheckerOption) *DNF5UpdateChecker {
	c := &DNF5UpdateChecker{
		dnf5Path:   dnf5Path,
		privRunner: privRunner,
		rawExec:    privilege.DefaultRawExecutor,
		geteuid:    os.Geteuid,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Check queries DNF5 for available package updates.
func (c *DNF5UpdateChecker) Check(ctx context.Context, opts domain.CheckOptions) (domain.UpdateStatus, error) {
	if err := ctx.Err(); err != nil {
		return domain.UpdateStatus{
			State:    domain.UpdateStateFailed,
			ErrorMsg: err.Error(),
		}, err
	}

	if c.dnf5Path == "" {
		return domain.UpdateStatus{
			State:    domain.UpdateStateFailed,
			ErrorMsg: domain.ErrDNF5NotFound.Error(),
		}, domain.ErrDNF5NotFound
	}

	args := []string{"check-upgrade", "--json"}
	if opts.RefreshMetadata {
		args = append(args, "--refresh")
	}

	var res privilege.CommandResult
	var runErr error

	geteuid := c.geteuid
	if geteuid == nil {
		geteuid = os.Geteuid
	}
	rawExec := c.rawExec
	if rawExec == nil {
		rawExec = privilege.DefaultRawExecutor
	}

	if geteuid() == 0 {
		// Running as root: execute directly without modification
		stdout, stderr, exitCode, err := rawExec(ctx, c.dnf5Path, args...)
		res = privilege.CommandResult{
			Stdout:   stdout,
			Stderr:   stderr,
			ExitCode: exitCode,
		}
		runErr = err
	} else {
		// Running unprivileged: use least privilege inspection.
		// Point system_state_dir to /var/empty so libdnf5 reads system RPMs and repos
		// without failing on root-only /usr/lib/sysimage/libdnf5/packages.toml.
		stateDir := "/var/empty"
		if _, statErr := os.Stat(stateDir); statErr != nil {
			stateDir = os.TempDir()
		}
		unprivArgs := append(args, "--setopt=system_state_dir="+stateDir)

		stdout, stderr, exitCode, err := rawExec(ctx, c.dnf5Path, unprivArgs...)
		res = privilege.CommandResult{
			Stdout:   stdout,
			Stderr:   stderr,
			ExitCode: exitCode,
		}
		runErr = err

		// If direct unprivileged execution failed and a privileged runner exists,
		// attempt fallback via privileged runner (e.g. if root credentials are cached).
		if (res.ExitCode != 0 && res.ExitCode != 100) && c.privRunner != nil {
			cmd := privilege.PrivilegedCommand{
				Executable: c.dnf5Path,
				Args:       args,
			}
			fallbackRes, fallbackErr := c.privRunner.Run(ctx, cmd)
			if fallbackErr == nil && (fallbackRes.ExitCode == 0 || fallbackRes.ExitCode == 100) {
				res = fallbackRes
				runErr = nil
			}
		}
	}

	if ctx.Err() != nil {
		return domain.UpdateStatus{
			State:    domain.UpdateStateFailed,
			ErrorMsg: ctx.Err().Error(),
		}, ctx.Err()
	}

	if runErr != nil && (errors.Is(runErr, exec.ErrNotFound) || errors.Is(runErr, privilege.ErrSudoNotFound)) {
		return domain.UpdateStatus{
			State:    domain.UpdateStateFailed,
			ErrorMsg: domain.ErrDNF5NotFound.Error(),
		}, domain.ErrDNF5NotFound
	}

	// Exit codes:
	// 0: success (up to date, or updates available when running with --json in DNF5)
	// 100: traditional DNF exit code when updates are available
	// Any other code indicates an error (e.g. exit code 1 on repo sync failure or missing permissions)
	if res.ExitCode != 0 && res.ExitCode != 100 {
		errMsg := strings.TrimSpace(string(res.Stderr))
		if errMsg == "" && runErr != nil {
			errMsg = runErr.Error()
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("dnf5 exited with code %d", res.ExitCode)
		}
		return domain.UpdateStatus{
			State:       domain.UpdateStateFailed,
			LastChecked: time.Now(),
			ErrorMsg:    errMsg,
		}, fmt.Errorf("%w: %s", domain.ErrUpdateCheckFailed, errMsg)
	}

	return ParseDNF5CheckUpgradeJSON(res.Stdout)
}
