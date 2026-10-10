package domain_test

import (
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/domain"
)

func TestUpdateStatus_StateHelpers(t *testing.T) {
	tests := []struct {
		name         string
		status       domain.UpdateStatus
		wantAvail    bool
		wantUpToDate bool
	}{
		{
			name: "available updates",
			status: domain.UpdateStatus{
				State:       domain.UpdateStateAvailable,
				UpdateCount: 5,
				Updates: []domain.PackageUpdate{
					{Name: "pkg1", Arch: "x86_64", EVR: "1.0-1.fc44", Repository: "updates"},
				},
				LastChecked: time.Now(),
			},
			wantAvail:    true,
			wantUpToDate: false,
		},
		{
			name: "up to date",
			status: domain.UpdateStatus{
				State:       domain.UpdateStateUpToDate,
				UpdateCount: 0,
				LastChecked: time.Now(),
			},
			wantAvail:    false,
			wantUpToDate: true,
		},
		{
			name: "available state but zero count",
			status: domain.UpdateStatus{
				State:       domain.UpdateStateAvailable,
				UpdateCount: 0,
			},
			wantAvail:    false,
			wantUpToDate: false,
		},
		{
			name: "failed state",
			status: domain.UpdateStatus{
				State:    domain.UpdateStateFailed,
				ErrorMsg: "network down",
			},
			wantAvail:    false,
			wantUpToDate: false,
		},
		{
			name: "checking state",
			status: domain.UpdateStatus{
				State: domain.UpdateStateChecking,
			},
			wantAvail:    false,
			wantUpToDate: false,
		},
		{
			name: "unknown state",
			status: domain.UpdateStatus{
				State: domain.UpdateStateUnknown,
			},
			wantAvail:    false,
			wantUpToDate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsAvailable(); got != tt.wantAvail {
				t.Errorf("IsAvailable() = %v, want %v", got, tt.wantAvail)
			}
			if got := tt.status.IsUpToDate(); got != tt.wantUpToDate {
				t.Errorf("IsUpToDate() = %v, want %v", got, tt.wantUpToDate)
			}
		})
	}
}
