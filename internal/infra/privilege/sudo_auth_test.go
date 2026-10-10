package privilege_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/privilege"
)

func TestSudoAuthorizationProvider_RootBypass(t *testing.T) {
	called := false
	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		called = true
		return nil, nil, 0, nil
	}

	provider := privilege.NewSudoAuthorizationProvider(
		privilege.WithAuthSudoPath("/usr/bin/sudo"),
		privilege.WithAuthEuidGetter(func() int { return 0 }),
		privilege.WithAuthExecutor(fakeExecutor),
	)

	err := provider.Authorize(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for root user, got %v", err)
	}
	if called {
		t.Errorf("executor should not be invoked when running as root (uid 0)")
	}
}

func TestSudoAuthorizationProvider_NonRoot_Success(t *testing.T) {
	var capturedName string
	var capturedArgs []string

	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		capturedName = name
		capturedArgs = args
		return nil, nil, 0, nil
	}

	provider := privilege.NewSudoAuthorizationProvider(
		privilege.WithAuthSudoPath("/usr/bin/sudo"),
		privilege.WithAuthEuidGetter(func() int { return 1000 }),
		privilege.WithAuthExecutor(fakeExecutor),
	)

	err := provider.Authorize(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedName != "/usr/bin/sudo" {
		t.Errorf("expected /usr/bin/sudo, got %q", capturedName)
	}
	expectedArgs := []string{"-n", "true"}
	if !reflect.DeepEqual(capturedArgs, expectedArgs) {
		t.Errorf("expected args %v, got %v", expectedArgs, capturedArgs)
	}
}

func TestSudoAuthorizationProvider_NonRoot_PasswordRequired(t *testing.T) {
	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return nil, []byte("sudo: a password is required"), 1, errors.New("exit status 1")
	}

	provider := privilege.NewSudoAuthorizationProvider(
		privilege.WithAuthSudoPath("/usr/bin/sudo"),
		privilege.WithAuthEuidGetter(func() int { return 1000 }),
		privilege.WithAuthExecutor(fakeExecutor),
	)

	err := provider.Authorize(context.Background())
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
	if !errors.Is(err, domain.ErrAuthorizationFailed) {
		t.Errorf("expected ErrAuthorizationFailed, got %v", err)
	}
}

func TestSudoAuthorizationProvider_MissingSudoPath(t *testing.T) {
	provider := privilege.NewSudoAuthorizationProvider(
		privilege.WithAuthSudoPath(""),
		privilege.WithAuthEuidGetter(func() int { return 1000 }),
	)

	err := provider.Authorize(context.Background())
	if err == nil {
		t.Fatal("expected authorization error for missing sudo, got nil")
	}
	if !errors.Is(err, domain.ErrAuthorizationFailed) {
		t.Errorf("expected ErrAuthorizationFailed, got %v", err)
	}
}

func TestSudoAuthorizationProvider_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fakeExecutor := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		return nil, nil, -1, ctx.Err()
	}

	provider := privilege.NewSudoAuthorizationProvider(
		privilege.WithAuthSudoPath("/usr/bin/sudo"),
		privilege.WithAuthEuidGetter(func() int { return 1000 }),
		privilege.WithAuthExecutor(fakeExecutor),
	)

	err := provider.Authorize(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestSudoAuthorizationProvider_RealExecutionSafe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Uses default NewSudoAuthorizationProvider() without mocks
	provider := privilege.NewSudoAuthorizationProvider()
	err := provider.Authorize(ctx)
	// On this host, either it succeeds (passwordless sudo/cached credentials/root)
	// or returns ErrAuthorizationFailed. Crucially, it must NEVER hang or prompt for password!
	if err != nil && !errors.Is(err, domain.ErrAuthorizationFailed) {
		t.Errorf("expected nil or ErrAuthorizationFailed, got %v", err)
	}
}
