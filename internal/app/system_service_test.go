package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
)

type mockOSDetector struct {
	calls int
	info  domain.SystemInfo
	err   error
}

func (m *mockOSDetector) Detect(ctx context.Context) (domain.SystemInfo, error) {
	m.calls++
	if m.err != nil {
		return domain.SystemInfo{}, m.err
	}
	return m.info, nil
}

func TestSystemService_GetSystemInfo_Caching(t *testing.T) {
	mock := &mockOSDetector{
		info: domain.SystemInfo{
			OS: domain.OSRelease{
				ID:         "fedora",
				PrettyName: "Fedora Linux 44",
			},
			IsSupported: true,
		},
	}

	svc := app.NewSystemService(mock)

	// First call should invoke detector
	info1, err := svc.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 detector call, got %d", mock.calls)
	}

	// Second call should return cached info
	info2, err := svc.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if mock.calls != 1 {
		t.Errorf("expected detector call count to stay 1, got %d", mock.calls)
	}
	if info1.OS.ID != info2.OS.ID {
		t.Errorf("expected cached info match: %v vs %v", info1, info2)
	}

	// Refresh call should invoke detector again
	_, err = svc.RefreshSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on refresh: %v", err)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 detector calls after refresh, got %d", mock.calls)
	}
}

func TestSystemService_VerifyHost(t *testing.T) {
	t.Run("Supported Fedora", func(t *testing.T) {
		mock := &mockOSDetector{
			info: domain.SystemInfo{
				OS: domain.OSRelease{
					ID:         "fedora",
					PrettyName: "Fedora Linux 44",
				},
				IsSupported: true,
			},
		}
		svc := app.NewSystemService(mock)
		if err := svc.VerifyHost(context.Background()); err != nil {
			t.Errorf("expected nil error for Fedora host, got: %v", err)
		}
	})

	t.Run("Unsupported Host", func(t *testing.T) {
		mock := &mockOSDetector{
			info: domain.SystemInfo{
				OS: domain.OSRelease{
					ID:         "arch",
					PrettyName: "Arch Linux",
				},
				IsSupported: false,
			},
		}
		svc := app.NewSystemService(mock)
		err := svc.VerifyHost(context.Background())
		if err == nil {
			t.Fatal("expected error for unsupported host, got nil")
		}
		if !errors.Is(err, domain.ErrUnsupportedDistribution) {
			t.Errorf("expected ErrUnsupportedDistribution, got %v", err)
		}
	})
}
