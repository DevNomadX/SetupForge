package privilege

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/setupforge/setupforge/internal/domain"
)

// SudoAuthOption defines functional options for SudoAuthorizationProvider.
type SudoAuthOption func(*SudoAuthorizationProvider)

// WithAuthExecutor overrides the raw command executor for authorization testing.
func WithAuthExecutor(exec RawCommandExecutor) SudoAuthOption {
	return func(p *SudoAuthorizationProvider) {
		p.executor = exec
	}
}

// WithAuthEuidGetter overrides the geteuid provider for authorization testing.
func WithAuthEuidGetter(geteuid func() int) SudoAuthOption {
	return func(p *SudoAuthorizationProvider) {
		p.geteuid = geteuid
	}
}

// WithAuthSudoPath overrides the sudo binary path.
func WithAuthSudoPath(path string) SudoAuthOption {
	return func(p *SudoAuthorizationProvider) {
		p.sudoPath = path
	}
}

// SudoAuthorizationProvider implements domain.AuthorizationProvider using non-interactive sudo preflight.
type SudoAuthorizationProvider struct {
	sudoPath string
	executor RawCommandExecutor
	geteuid  func() int
}

// NewSudoAuthorizationProvider constructs a SudoAuthorizationProvider.
func NewSudoAuthorizationProvider(opts ...SudoAuthOption) *SudoAuthorizationProvider {
	path, err := exec.LookPath("sudo")
	if err != nil {
		path = "/usr/bin/sudo"
	}

	p := &SudoAuthorizationProvider{
		sudoPath: path,
		geteuid:  os.Geteuid,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Authorize verifies whether root permissions are already held or cached in sudo credentials.
// It executes "sudo -n true" without prompting for passwords.
// If authorization fails, it returns domain.ErrAuthorizationFailed.
func (p *SudoAuthorizationProvider) Authorize(ctx context.Context) error {
	if p.geteuid != nil && p.geteuid() == 0 {
		// Already root: authorization trivially satisfied
		return nil
	}

	if p.sudoPath == "" {
		return fmt.Errorf("%w: sudo authorization executable not found", domain.ErrAuthorizationFailed)
	}

	var stderr []byte
	var exitCode int
	var err error

	if p.executor != nil {
		_, stderr, exitCode, err = p.executor(ctx, p.sudoPath, "-n", "true")
	} else {
		cmd := exec.CommandContext(ctx, p.sudoPath, "-n", "true")
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		err = cmd.Run()
		stderr = stderrBuf.Bytes()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if exitCode != 0 || err != nil {
		reason := strings.TrimSpace(string(stderr))
		if reason == "" && err != nil {
			reason = err.Error()
		}
		if reason == "" {
			reason = fmt.Sprintf("exit status %d", exitCode)
		}
		return fmt.Errorf("%w: %s", domain.ErrAuthorizationFailed, reason)
	}

	return nil
}
