package privilege

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
)

var (
	// ErrEmptyExecutable is returned when a privileged command specifies an empty executable.
	ErrEmptyExecutable = errors.New("privileged command executable is empty")

	// ErrSudoNotFound is returned when the sudo binary cannot be located on the host.
	ErrSudoNotFound = errors.New("sudo authorization executable not found")
)

// RawCommandExecutor abstracts process invocation for deterministic testing without external dependencies.
type RawCommandExecutor func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error)

// DefaultRawExecutor invokes binaries directly with explicit argument vectors using os/exec without any shell.
func DefaultRawExecutor(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // Command completed execution; exitCode reflects status
		} else {
			exitCode = -1
		}
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitCode, err
}

// SudoRunnerOption defines functional options for configuring SudoRunner.
type SudoRunnerOption func(*SudoRunner)

// WithNonInteractive sets whether sudo runs with -n (--non-interactive).
// By default, this is true to prevent background operations from freezing on password prompts.
func WithNonInteractive(nonInteractive bool) SudoRunnerOption {
	return func(s *SudoRunner) {
		s.nonInteractive = nonInteractive
	}
}

// WithRawExecutor configures a custom low-level process executor.
func WithRawExecutor(executor RawCommandExecutor) SudoRunnerOption {
	return func(s *SudoRunner) {
		s.executor = executor
	}
}

// WithGracePeriods configures the SIGINT and SIGTERM escalation grace periods.
func WithGracePeriods(sigintGrace, sigtermGrace time.Duration) SudoRunnerOption {
	return func(s *SudoRunner) {
		s.sigintGrace = sigintGrace
		s.sigtermGrace = sigtermGrace
	}
}

// SudoRunner executes privileged system commands via sudo, or directly if already root.
// It manages process group isolation and bounded signal escalation upon cancellation.
type SudoRunner struct {
	sudoPath       string
	nonInteractive bool
	executor       RawCommandExecutor
	geteuid        func() int
	sigintGrace    time.Duration
	sigtermGrace   time.Duration
}

// NewSudoRunner constructs a SudoRunner using the host's sudo binary.
func NewSudoRunner(opts ...SudoRunnerOption) *SudoRunner {
	path, err := exec.LookPath("sudo")
	if err != nil {
		path = "/usr/bin/sudo"
	}

	runner := &SudoRunner{
		sudoPath:       path,
		nonInteractive: true,
		geteuid:        os.Geteuid,
		sigintGrace:    30 * time.Second,
		sigtermGrace:   10 * time.Second,
	}

	for _, opt := range opts {
		opt(runner)
	}

	return runner
}

// NewCustomSudoRunner creates a SudoRunner with an explicit executable, executor, and geteuid provider.
// Useful for hermetic unit testing.
func NewCustomSudoRunner(sudoPath string, executor RawCommandExecutor, geteuid func() int, opts ...SudoRunnerOption) *SudoRunner {
	if geteuid == nil {
		geteuid = os.Geteuid
	}

	runner := &SudoRunner{
		sudoPath:       sudoPath,
		nonInteractive: true,
		executor:       executor,
		geteuid:        geteuid,
		sigintGrace:    30 * time.Second,
		sigtermGrace:   10 * time.Second,
	}

	for _, opt := range opts {
		opt(runner)
	}

	return runner
}

// Run dispatches the PrivilegedCommand via sudo or directly if euid is 0.
func (s *SudoRunner) Run(ctx context.Context, cmd PrivilegedCommand) (CommandResult, error) {
	if err := ctx.Err(); err != nil {
		return CommandResult{ExitCode: -1, Cancelled: true}, err
	}

	if cmd.Executable == "" {
		return CommandResult{ExitCode: -1}, ErrEmptyExecutable
	}

	var execName string
	var execArgs []string

	if s.geteuid() == 0 {
		// Already root: execute the command directly without elevation
		execName = cmd.Executable
		execArgs = cmd.Args
	} else {
		if s.sudoPath == "" {
			return CommandResult{ExitCode: -1}, ErrSudoNotFound
		}
		execName = s.sudoPath
		if s.nonInteractive {
			execArgs = append([]string{"-n", cmd.Executable}, cmd.Args...)
		} else {
			execArgs = append([]string{cmd.Executable}, cmd.Args...)
		}
	}

	if s.executor != nil {
		startTime := time.Now()
		stdout, stderr, exitCode, err := s.executor(ctx, execName, execArgs...)
		duration := time.Since(startTime)

		res := CommandResult{
			Stdout:         stdout,
			Stderr:         stderr,
			ExitCode:       exitCode,
			ExitedNormally: (exitCode >= 0),
			Duration:       duration,
		}

		if ctx.Err() != nil {
			res.Cancelled = true
			res.TimedOut = errors.Is(ctx.Err(), context.DeadlineExceeded)
			return res, ctx.Err()
		}

		if err != nil && errors.Is(err, exec.ErrNotFound) {
			if execName == s.sudoPath {
				res.ExitCode = -1
				return res, ErrSudoNotFound
			}
			res.ExitCode = -1
			return res, fmt.Errorf("executable not found: %s", cmd.Executable)
		}

		return res, err
	}

	// Real execution: isolated process group with bounded signal escalation
	return ExecuteWithProcessGroup(ctx, execName, execArgs, cmd.Env, cmd.Dir, s.sigintGrace, s.sigtermGrace)
}

// ExecuteWithProcessGroup runs a command in an isolated process group with bounded signal escalation upon cancellation.
// Invariant: Signal handling does not establish transaction success or failure; it merely bounds process lifetime.
func ExecuteWithProcessGroup(ctx context.Context, name string, args []string, env []string, dir string, sigintGrace, sigtermGrace time.Duration) (domain.ProcessResult, error) {
	if sigintGrace <= 0 {
		sigintGrace = 30 * time.Second
	}
	if sigtermGrace <= 0 {
		sigtermGrace = 10 * time.Second
	}

	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 5 * time.Second

	if dir != "" {
		cmd.Dir = dir
	} else {
		cmd.Dir = "/"
	}

	if len(env) > 0 {
		cmd.Env = env
	} else {
		cmd.Env = []string{
			"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
			"LC_ALL=C.UTF-8",
			"LANG=C.UTF-8",
		}
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	if err := ctx.Err(); err != nil {
		return domain.ProcessResult{
			ExitCode:       -1,
			ExitedNormally: false,
			Cancelled:      true,
			TimedOut:       errors.Is(err, context.DeadlineExceeded),
		}, err
	}

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return domain.ProcessResult{
			ExitCode:       -1,
			ExitedNormally: false,
			Duration:       time.Since(startTime),
		}, err
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		pgid = cmd.Process.Pid
	}

	done := make(chan struct{})
	var cancelMu sync.Mutex
	var wasCancelled, wasTimedOut bool

	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			cancelMu.Lock()
			wasCancelled = true
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				wasTimedOut = true
			}
			cancelMu.Unlock()

			// 1. Deliver SIGINT to process group
			_ = syscall.Kill(-pgid, syscall.SIGINT)

			// 2. Wait bounded grace period for clean exit
			select {
			case <-done:
				return
			case <-time.After(sigintGrace):
			}

			// 3. Escalate to SIGTERM
			_ = syscall.Kill(-pgid, syscall.SIGTERM)

			// 4. Wait bounded grace period
			select {
			case <-done:
				return
			case <-time.After(sigtermGrace):
			}

			// 5. Escalate to SIGKILL as last resort
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
	}()

	waitErr := cmd.Wait()
	close(done)
	duration := time.Since(startTime)

	exitCode := 0
	exitedNormally := true
	var wasSignaled bool
	var signalName string

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if ws.Signaled() {
					wasSignaled = true
					signalName = ws.Signal().String()
					exitedNormally = false
				}
			}
		} else {
			exitCode = -1
			exitedNormally = false
		}
	}

	cancelMu.Lock()
	cancelled := wasCancelled
	timedOut := wasTimedOut
	cancelMu.Unlock()

	return domain.ProcessResult{
		ExitCode:       exitCode,
		ExitedNormally: exitedNormally,
		Signaled:       wasSignaled,
		Signal:         signalName,
		Cancelled:      cancelled,
		TimedOut:       timedOut,
		Stdout:         stdoutBuf.Bytes(),
		Stderr:         stderrBuf.Bytes(),
		Duration:       duration,
	}, waitErr
}
