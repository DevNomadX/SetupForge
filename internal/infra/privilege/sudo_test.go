package privilege_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/infra/privilege"
)

func TestSudoRunner_UserExecution(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		capturedName = name
		capturedArgs = args
		return []byte("output ok"), nil, 0, nil
	}

	geteuidUser := func() int { return 1000 }
	runner := privilege.NewCustomSudoRunner("/usr/bin/sudo", fakeExecutor, geteuidUser)

	cmd := privilege.PrivilegedCommand{
		Executable: "/usr/bin/dnf5",
		Args:       []string{"check-upgrade", "--json"},
	}

	res, err := runner.Run(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedName != "/usr/bin/sudo" {
		t.Errorf("expected sudo path, got %q", capturedName)
	}

	expectedArgs := []string{"-n", "/usr/bin/dnf5", "check-upgrade", "--json"}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Errorf("expected args %v, got %v", expectedArgs, capturedArgs)
	}

	if string(res.Stdout) != "output ok" {
		t.Errorf("expected output ok, got %q", string(res.Stdout))
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
}

func TestSudoRunner_RootExecution(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		capturedName = name
		capturedArgs = args
		return []byte("root ok"), nil, 0, nil
	}

	geteuidRoot := func() int { return 0 }
	runner := privilege.NewCustomSudoRunner("/usr/bin/sudo", fakeExecutor, geteuidRoot)

	cmd := privilege.PrivilegedCommand{
		Executable: "/usr/bin/dnf5",
		Args:       []string{"check-upgrade", "--json"},
	}

	res, err := runner.Run(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When running as root, sudo should NOT be invoked
	if capturedName != "/usr/bin/dnf5" {
		t.Errorf("expected dnf5 to be called directly, got %q", capturedName)
	}

	expectedArgs := []string{"check-upgrade", "--json"}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Errorf("expected args %v, got %v", expectedArgs, capturedArgs)
	}

	if string(res.Stdout) != "root ok" {
		t.Errorf("expected root ok, got %q", string(res.Stdout))
	}
}

func TestSudoRunner_InteractiveOption(t *testing.T) {
	var capturedArgs []string

	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		capturedArgs = args
		return []byte("ok"), nil, 0, nil
	}

	runner := privilege.NewCustomSudoRunner(
		"/usr/bin/sudo",
		fakeExecutor,
		func() int { return 1000 },
		privilege.WithNonInteractive(false),
	)

	cmd := privilege.PrivilegedCommand{
		Executable: "/usr/bin/dnf5",
		Args:       []string{"check-upgrade", "--json"},
	}

	_, err := runner.Run(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedArgs := []string{"/usr/bin/dnf5", "check-upgrade", "--json"}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Errorf("expected args %v without -n, got %v", expectedArgs, capturedArgs)
	}
}

func TestSudoRunner_EmptyExecutable(t *testing.T) {
	runner := privilege.NewCustomSudoRunner("/usr/bin/sudo", nil, nil)
	cmd := privilege.PrivilegedCommand{
		Executable: "",
		Args:       nil,
	}

	_, err := runner.Run(context.Background(), cmd)
	if !errors.Is(err, privilege.ErrEmptyExecutable) {
		t.Errorf("expected ErrEmptyExecutable, got %v", err)
	}
}

func TestSudoRunner_MissingSudo(t *testing.T) {
	runner := privilege.NewCustomSudoRunner("", nil, func() int { return 1000 })
	cmd := privilege.PrivilegedCommand{
		Executable: "/usr/bin/dnf5",
		Args:       []string{"check-upgrade"},
	}

	_, err := runner.Run(context.Background(), cmd)
	if !errors.Is(err, privilege.ErrSudoNotFound) {
		t.Errorf("expected ErrSudoNotFound, got %v", err)
	}
}

func TestSudoRunner_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		t.Fatal("executor should not run on cancelled context")
		return nil, nil, 0, nil
	}

	runner := privilege.NewCustomSudoRunner("/usr/bin/sudo", fakeExecutor, func() int { return 1000 })
	cmd := privilege.PrivilegedCommand{
		Executable: "/usr/bin/dnf5",
		Args:       []string{"check-upgrade"},
	}

	_, err := runner.Run(ctx, cmd)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestSudoRunner_ExitCodePreserved(t *testing.T) {
	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("stdout info"), []byte("stderr warning"), 100, nil
	}

	runner := privilege.NewCustomSudoRunner("/usr/bin/sudo", fakeExecutor, func() int { return 1000 })
	cmd := privilege.PrivilegedCommand{
		Executable: "/usr/bin/dnf5",
		Args:       []string{"check-upgrade", "--json"},
	}

	res, err := runner.Run(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != 100 {
		t.Errorf("expected exit code 100, got %d", res.ExitCode)
	}
	if string(res.Stdout) != "stdout info" {
		t.Errorf("expected stdout info, got %q", string(res.Stdout))
	}
	if string(res.Stderr) != "stderr warning" {
		t.Errorf("expected stderr warning, got %q", string(res.Stderr))
	}
}

func TestDefaultRawExecutor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stdout, stderr, code, err := privilege.DefaultRawExecutor(ctx, "echo", "hello privilege")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(string(stdout)) != "hello privilege" {
		t.Errorf("expected 'hello privilege', got %q", string(stdout))
	}
	if len(stderr) != 0 {
		t.Errorf("expected empty stderr, got %q", string(stderr))
	}
}

func TestExecuteWithProcessGroup_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := privilege.ExecuteWithProcessGroup(
		ctx,
		"/usr/bin/echo",
		[]string{"process group test"},
		[]string{"TEST_VAR=1"},
		"/tmp",
		1*time.Second,
		1*time.Second,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
	if !res.ExitedNormally {
		t.Errorf("expected ExitedNormally true")
	}
	if strings.TrimSpace(string(res.Stdout)) != "process group test" {
		t.Errorf("expected 'process group test', got %q", string(res.Stdout))
	}
	if res.Signaled {
		t.Errorf("expected Signaled false")
	}
}

func TestExecuteWithProcessGroup_CancellationEscalation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run sleep 10 with fast escalation grace periods (50ms)
	res, err := privilege.ExecuteWithProcessGroup(
		ctx,
		"/usr/bin/sleep",
		[]string{"10"},
		nil,
		"",
		50*time.Millisecond,
		50*time.Millisecond,
	)

	if err == nil {
		t.Fatal("expected error on cancelled command, got nil")
	}
	if !res.Cancelled && !res.TimedOut {
		t.Errorf("expected Cancelled or TimedOut to be true, got res: %+v", res)
	}
	if res.ExitedNormally {
		t.Errorf("expected ExitedNormally false due to termination signal")
	}
}

func TestExecuteWithProcessGroup_PreCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already cancelled

	res, err := privilege.ExecuteWithProcessGroup(
		ctx,
		"/usr/bin/echo",
		[]string{"should not run"},
		nil,
		"",
		1*time.Second,
		1*time.Second,
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if !res.Cancelled {
		t.Errorf("expected Cancelled true")
	}
	if res.ExitCode != -1 {
		t.Errorf("expected exit code -1, got %d", res.ExitCode)
	}
}
