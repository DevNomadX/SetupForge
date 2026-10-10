package journal_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/infra/journal"
)

func TestFlockedEngineLock_ConcurrencyAndOwnership(t *testing.T) {
	lockDir := t.TempDir()
	lockPath := filepath.Join(lockDir, "test-engine.lock")

	engine1 := journal.NewEngineLock(journal.WithLockPath(lockPath))
	engine2 := journal.NewEngineLock(journal.WithLockPath(lockPath))

	// 1. Engine 1 acquires lock
	acquired, err := engine1.TryLock()
	if err != nil {
		t.Fatalf("engine 1 failed to acquire lock: %v", err)
	}
	if !acquired || !engine1.IsHeld() {
		t.Fatal("engine 1 did not report lock acquired")
	}

	// Re-calling TryLock on already held lock returns true
	reAcquired, err := engine1.TryLock()
	if err != nil || !reAcquired {
		t.Errorf("expected idempotent re-lock to succeed")
	}

	// 2. Engine 2 attempts to acquire the same lock -> must fail with ErrEngineLocked
	acquired2, err2 := engine2.TryLock()
	if acquired2 {
		t.Fatal("engine 2 acquired lock when engine 1 still held it")
	}
	if !errors.Is(err2, domain.ErrEngineLocked) {
		t.Fatalf("expected ErrEngineLocked for engine 2, got: %v", err2)
	}
	if engine2.IsHeld() {
		t.Errorf("engine 2 reported lock held despite failure")
	}

	// 3. Engine 1 unlocks
	if err := engine1.Unlock(); err != nil {
		t.Fatalf("engine 1 unlock failed: %v", err)
	}
	if engine1.IsHeld() {
		t.Errorf("engine 1 still reports held after unlock")
	}

	// 4. Engine 2 can now acquire lock
	acquired3, err3 := engine2.TryLock()
	if err3 != nil {
		t.Fatalf("engine 2 failed to acquire lock after engine 1 unlocked: %v", err3)
	}
	if !acquired3 || !engine2.IsHeld() {
		t.Fatal("engine 2 did not report lock acquired")
	}

	// Clean up
	_ = engine2.Unlock()
}
