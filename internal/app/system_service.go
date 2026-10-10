package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/setupforge/setupforge/internal/domain"
)

// SystemService orchestrates host system discovery and environment verification.
// It sits between the Bubble Tea presentation layer and the platform infrastructure.
type SystemService struct {
	detector domain.OSDetector

	mu     sync.RWMutex
	cached *domain.SystemInfo
}

// NewSystemService constructs a SystemService with the provided OSDetector.
func NewSystemService(detector domain.OSDetector) *SystemService {
	return &SystemService{
		detector: detector,
	}
}

// GetSystemInfo retrieves host system information, returning cached data if available.
func (s *SystemService) GetSystemInfo(ctx context.Context) (domain.SystemInfo, error) {
	s.mu.RLock()
	if s.cached != nil {
		info := *s.cached
		s.mu.RUnlock()
		return info, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check cache under write lock
	if s.cached != nil {
		return *s.cached, nil
	}

	info, err := s.detector.Detect(ctx)
	if err != nil {
		return domain.SystemInfo{}, fmt.Errorf("detecting host system: %w", err)
	}

	s.cached = &info
	return info, nil
}

// RefreshSystemInfo forces a re-detection of host system information.
func (s *SystemService) RefreshSystemInfo(ctx context.Context) (domain.SystemInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := s.detector.Detect(ctx)
	if err != nil {
		return domain.SystemInfo{}, fmt.Errorf("refreshing host system: %w", err)
	}

	s.cached = &info
	return info, nil
}

// VerifyHost checks whether the current system is supported for SetupForge operations.
func (s *SystemService) VerifyHost(ctx context.Context) error {
	info, err := s.GetSystemInfo(ctx)
	if err != nil {
		return err
	}
	if !info.IsSupported {
		return fmt.Errorf("%w: detected %s %s", domain.ErrUnsupportedDistribution, info.OS.Name, info.OS.Version)
	}
	return nil
}
