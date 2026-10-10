package update_test

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/privilege"
	"github.com/setupforge/setupforge/internal/infra/update"
)

type fakePrivilegedRunner struct {
	runFn func(ctx context.Context, cmd privilege.PrivilegedCommand) (privilege.CommandResult, error)
}

func (f *fakePrivilegedRunner) Run(ctx context.Context, cmd privilege.PrivilegedCommand) (privilege.CommandResult, error) {
	if f.runFn != nil {
		return f.runFn(ctx, cmd)
	}
	return privilege.CommandResult{}, nil
}

func TestDNF5UpdateChecker_Success_Unprivileged_WithUpdates(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		capturedName = name
		capturedArgs = args
		stdout := []byte(`{"upgrades": [{"name": "pkgA", "arch": "x86_64", "evr": "1.0-1.fc44", "repository": "updates"}]}`)
		return stdout, nil, 0, nil
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		nil,
		update.WithRawExecutor(mockRaw),
		update.WithEuidGetter(func() int { return 1000 }),
	)

	status, err := checker.Check(context.Background(), domain.CheckOptions{RefreshMetadata: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedName != "/usr/bin/dnf5" {
		t.Errorf("expected dnf5 binary path, got %q", capturedName)
	}

	expectedPrefix := []string{"check-upgrade", "--json"}
	if len(capturedArgs) < 3 || !reflect.DeepEqual(capturedArgs[:2], expectedPrefix) {
		t.Errorf("expected args prefix %v, got %v", expectedPrefix, capturedArgs)
	}
	if !strings.HasPrefix(capturedArgs[2], "--setopt=system_state_dir=") {
		t.Errorf("expected system_state_dir override, got %q", capturedArgs[2])
	}

	if status.State != domain.UpdateStateAvailable {
		t.Errorf("expected state %q, got %q", domain.UpdateStateAvailable, status.State)
	}
	if status.UpdateCount != 1 {
		t.Errorf("expected 1 update, got %d", status.UpdateCount)
	}
}

func TestDNF5UpdateChecker_Success_Root_WithUpdates_Exit100(t *testing.T) {
	var capturedArgs []string

	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		capturedArgs = args
		stdout := []byte(`{"upgrades": [{"name": "pkgA", "arch": "x86_64", "evr": "1.0-1.fc44", "repository": "updates"}]}`)
		return stdout, nil, 100, nil // DNF exit 100 indicates updates available
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		nil,
		update.WithRawExecutor(mockRaw),
		update.WithEuidGetter(func() int { return 0 }), // Root
	)

	status, err := checker.Check(context.Background(), domain.CheckOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedArgs := []string{"check-upgrade", "--json"}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Errorf("expected root args %v, got %v", expectedArgs, capturedArgs)
	}

	if status.State != domain.UpdateStateAvailable {
		t.Errorf("expected state %q, got %q", domain.UpdateStateAvailable, status.State)
	}
	if status.UpdateCount != 1 {
		t.Errorf("expected 1 update, got %d", status.UpdateCount)
	}
}

func TestDNF5UpdateChecker_Success_UpToDate_Exit0(t *testing.T) {
	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("{}"), nil, 0, nil
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		nil,
		update.WithRawExecutor(mockRaw),
	)

	status, err := checker.Check(context.Background(), domain.CheckOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.State != domain.UpdateStateUpToDate {
		t.Errorf("expected state %q, got %q", domain.UpdateStateUpToDate, status.State)
	}
	if status.UpdateCount != 0 {
		t.Errorf("expected 0 updates, got %d", status.UpdateCount)
	}
}

func TestDNF5UpdateChecker_WithRefresh(t *testing.T) {
	var capturedArgs []string
	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		capturedArgs = args
		return []byte("{}"), nil, 0, nil
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		nil,
		update.WithRawExecutor(mockRaw),
		update.WithEuidGetter(func() int { return 0 }),
	)

	_, err := checker.Check(context.Background(), domain.CheckOptions{RefreshMetadata: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedArgs := []string{"check-upgrade", "--json", "--refresh"}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Errorf("expected args with --refresh %v, got %v", expectedArgs, capturedArgs)
	}
}

func TestDNF5UpdateChecker_FallbackToPrivRunner(t *testing.T) {
	// Raw executor fails with exit code 1 (simulating unprivileged permission failure)
	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return nil, []byte("permission denied"), 1, nil
	}

	var capturedCmd privilege.PrivilegedCommand
	fakeRunner := &fakePrivilegedRunner{
		runFn: func(ctx context.Context, cmd privilege.PrivilegedCommand) (privilege.CommandResult, error) {
			capturedCmd = cmd
			return privilege.CommandResult{
				Stdout:   []byte(`{"upgrades": [{"name": "pkgB", "arch": "x86_64", "evr": "2.0-1.fc44", "repository": "updates"}]}`),
				ExitCode: 0,
			}, nil
		},
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		fakeRunner,
		update.WithRawExecutor(mockRaw),
		update.WithEuidGetter(func() int { return 1000 }),
	)

	status, err := checker.Check(context.Background(), domain.CheckOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCmd.Executable != "/usr/bin/dnf5" {
		t.Errorf("expected fallback invocation of /usr/bin/dnf5, got %q", capturedCmd.Executable)
	}
	if status.State != domain.UpdateStateAvailable || status.UpdateCount != 1 {
		t.Errorf("expected 1 update from fallback, got state %q, count %d", status.State, status.UpdateCount)
	}
}

func TestDNF5UpdateChecker_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		t.Fatal("raw executor should not be invoked when context is cancelled")
		return nil, nil, 0, nil
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		nil,
		update.WithRawExecutor(mockRaw),
	)

	status, err := checker.Check(ctx, domain.CheckOptions{})
	if err == nil {
		t.Fatal("expected context cancelled error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if status.State != domain.UpdateStateFailed {
		t.Errorf("expected state failed, got %q", status.State)
	}
}

func TestDNF5UpdateChecker_CommandFailure_Exit1(t *testing.T) {
	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return nil, []byte("Error: Failed to synchronize cache for repo 'updates'"), 1, nil
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		nil,
		update.WithRawExecutor(mockRaw),
		update.WithEuidGetter(func() int { return 0 }),
	)

	status, err := checker.Check(context.Background(), domain.CheckOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrUpdateCheckFailed) {
		t.Errorf("expected ErrUpdateCheckFailed, got %v", err)
	}
	if !strings.Contains(err.Error(), "Failed to synchronize cache") {
		t.Errorf("expected stderr in error message, got %v", err)
	}
	if status.State != domain.UpdateStateFailed {
		t.Errorf("expected state %q, got %q", domain.UpdateStateFailed, status.State)
	}
}

func TestDNF5UpdateChecker_BinaryNotFound(t *testing.T) {
	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return nil, nil, -1, exec.ErrNotFound
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		nil,
		update.WithRawExecutor(mockRaw),
		update.WithEuidGetter(func() int { return 0 }),
	)

	status, err := checker.Check(context.Background(), domain.CheckOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrDNF5NotFound) {
		t.Errorf("expected ErrDNF5NotFound, got %v", err)
	}
	if status.State != domain.UpdateStateFailed {
		t.Errorf("expected state %q, got %q", domain.UpdateStateFailed, status.State)
	}
}

func TestDNF5UpdateChecker_MalformedOutput(t *testing.T) {
	mockRaw := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return []byte("this is not json"), nil, 0, nil
	}

	checker := update.NewCustomDNF5UpdateChecker(
		"/usr/bin/dnf5",
		nil,
		update.WithRawExecutor(mockRaw),
	)

	status, err := checker.Check(context.Background(), domain.CheckOptions{})
	if err == nil {
		t.Fatal("expected error for malformed json, got nil")
	}
	if !errors.Is(err, domain.ErrMalformedUpdateOutput) {
		t.Errorf("expected ErrMalformedUpdateOutput, got %v", err)
	}
	if status.State != domain.UpdateStateFailed {
		t.Errorf("expected state %q, got %q", domain.UpdateStateFailed, status.State)
	}
}

func TestDNF5UpdateChecker_EmptyPath(t *testing.T) {
	checker := update.NewCustomDNF5UpdateChecker("", nil)
	status, err := checker.Check(context.Background(), domain.CheckOptions{})
	if err == nil {
		t.Fatal("expected error for empty binary path, got nil")
	}
	if !errors.Is(err, domain.ErrDNF5NotFound) {
		t.Errorf("expected ErrDNF5NotFound, got %v", err)
	}
	if status.State != domain.UpdateStateFailed {
		t.Errorf("expected state %q, got %q", domain.UpdateStateFailed, status.State)
	}
}
