package journal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/setupforge/setupforge/internal/domain"
	"golang.org/x/sys/unix"
)

// EngineLockOption configures FlockedEngineLock.
type EngineLockOption func(*FlockedEngineLock)

// WithLockPath overrides the lock file location.
func WithLockPath(path string) EngineLockOption {
	return func(l *FlockedEngineLock) {
		l.lockPath = path
	}
}

// FlockedEngineLock implements domain.EngineLock using Linux kernel advisory flock(2).
// Guarantees that only a single SetupForge transaction engine runs per user workspace.
//
// Invariant: The lock is maintained in the Linux kernel file table. If SetupForge crashes,
// receives SIGKILL, or terminates unexpectedly, the kernel automatically releases the lock.
type FlockedEngineLock struct {
	lockPath string
	mu       sync.Mutex
	fd       int
	isHeld   bool
}

// NewEngineLock creates an advisory kernel engine lock rooted at /var/tmp/setupforge-$UID/engine.lock.
func NewEngineLock(opts ...EngineLockOption) *FlockedEngineLock {
	l := &FlockedEngineLock{
		lockPath: defaultLockPath(),
		fd:       -1,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

func defaultLockPath() string {
	return filepath.Join("/var/tmp", fmt.Sprintf("setupforge-%d", os.Getuid()), "engine.lock")
}

// TryLock attempts to acquire the exclusive advisory kernel lock without blocking.
// Returns false, domain.ErrEngineLocked if another instance currently holds the lock.
func (l *FlockedEngineLock) TryLock() (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isHeld {
		return true, nil
	}

	dir := filepath.Dir(l.lockPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, fmt.Errorf("creating lock directory %s: %w", dir, err)
	}

	// Open or create the lock file with private 0600 permissions
	fd, err := unix.Open(l.lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC, 0600)
	if err != nil {
		return false, fmt.Errorf("opening lock file %s: %w", l.lockPath, err)
	}

	// Non-blocking exclusive advisory lock
	err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		_ = unix.Close(fd)
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return false, domain.ErrEngineLocked
		}
		return false, fmt.Errorf("acquiring advisory flock on %s: %w", l.lockPath, err)
	}

	l.fd = fd
	l.isHeld = true
	return true, nil
}

// Unlock releases the advisory lock and closes the lock file descriptor.
func (l *FlockedEngineLock) Unlock() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isHeld {
		return nil
	}

	// Release kernel lock and close descriptor
	_ = unix.Flock(l.fd, unix.LOCK_UN)
	err := unix.Close(l.fd)
	l.fd = -1
	l.isHeld = false

	if err != nil {
		return fmt.Errorf("closing engine lock file: %w", err)
	}

	return nil
}

// IsHeld reports whether the advisory lock is currently held by this process.
func (l *FlockedEngineLock) IsHeld() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.isHeld
}
